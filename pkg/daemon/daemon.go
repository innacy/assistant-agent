package daemon

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/auth"
	"github.com/innacy/assistant-agent/pkg/config"
	"github.com/innacy/assistant-agent/pkg/db"
	"github.com/innacy/assistant-agent/pkg/engine"
	"github.com/innacy/assistant-agent/pkg/sources"
	calendarpkg "github.com/innacy/assistant-agent/pkg/sources/calendar"
	contactspkg "github.com/innacy/assistant-agent/pkg/sources/contacts"
	gmailpkg "github.com/innacy/assistant-agent/pkg/sources/gmail"
	taskspkg "github.com/innacy/assistant-agent/pkg/sources/tasks"
)

const (
	defaultUserID      = "default"
	authRetryInterval  = 5 * time.Minute
	syncRetryDelay     = 30 * time.Second
)

type Daemon struct {
	db           *db.MongoDB
	cfg          *config.Config
	syncers      []sources.Syncer
	stopCh       chan struct{}
	triggerCh    <-chan struct{}
	doneCh       chan struct{}
	stopOnce     sync.Once
	pollInterval time.Duration
	userID       string
	loc          *time.Location
	syncing      atomic.Bool
	authMu       sync.Mutex
}

func New(database *db.MongoDB, cfg *config.Config, triggerCh <-chan struct{}) (*Daemon, error) {
	d := &Daemon{
		db:           database,
		cfg:          cfg,
		syncers:      nil,
		stopCh:       make(chan struct{}),
		triggerCh:    triggerCh,
		doneCh:       make(chan struct{}),
		pollInterval: cfg.Daemon.PollInterval,
		userID:       defaultUserID,
		loc:          cfg.Location(),
	}
	if d.pollInterval <= 0 {
		d.pollInterval = 15 * time.Minute
	}

	if err := d.initSyncers(); err != nil {
		log.Warn().Err(err).Msg("auth unavailable at startup; will retry every 5 minutes")
	}

	return d, nil
}

func (d *Daemon) IsSyncing() bool {
	return d.syncing.Load()
}

func (d *Daemon) initSyncers() error {
	httpClient, err := auth.GetClient(d.cfg.Google)
	if err != nil {
		return err
	}
	d.syncers = buildSyncers(httpClient, d.cfg)
	return nil
}

func buildSyncers(httpClient *http.Client, cfg *config.Config) []sources.Syncer {
	return []sources.Syncer{
		calendarpkg.New(httpClient),
		contactspkg.New(httpClient),
		taskspkg.New(httpClient),
		gmailpkg.New(httpClient, cfg.Gmail),
	}
}

func (d *Daemon) Run(ctx context.Context) error {
	defer close(d.doneCh)

	log.Info().Dur("interval", d.pollInterval).Msg("daemon starting")

	authTicker := time.NewTicker(authRetryInterval)
	defer authTicker.Stop()

	if err := d.runSyncCycle(ctx); err != nil {
		log.Error().Err(err).Msg("initial sync cycle failed")
	}

	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()

	midnightTicker := d.scheduleMidnight()
	defer midnightTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("context cancelled, shutting down")
			return ctx.Err()
		case <-d.stopCh:
			log.Info().Msg("stop requested, shutting down")
			return nil
		case <-d.triggerCh:
			if err := d.runSyncCycle(ctx); err != nil {
				log.Error().Err(err).Msg("manual sync cycle failed")
			}
		case <-authTicker.C:
			d.tryInitSyncers()
		case <-ticker.C:
			if err := d.runSyncCycle(ctx); err != nil {
				log.Error().Err(err).Msg("sync cycle failed")
			}
		case <-midnightTicker.C:
			d.runDailyRecurring(ctx)
			if err := d.refreshStatuses(ctx); err != nil {
				log.Warn().Err(err).Msg("midnight status refresh failed")
			}
			if err := d.archiveExpired(ctx); err != nil {
				log.Warn().Err(err).Msg("midnight archive expired failed")
			}
			midnightTicker = d.scheduleMidnight()
		}
	}
}

func (d *Daemon) tryInitSyncers() {
	if len(d.syncers) > 0 {
		return
	}
	d.authMu.Lock()
	defer d.authMu.Unlock()
	if len(d.syncers) > 0 {
		return
	}
	if err := d.initSyncers(); err != nil {
		log.Warn().Err(err).Msg("auth still unavailable, will retry")
		return
	}
	log.Info().Msg("auth restored, syncers initialized")
}

func (d *Daemon) RunOnce(ctx context.Context) error {
	if len(d.syncers) == 0 {
		if err := d.initSyncers(); err != nil {
			return err
		}
	}
	return d.runSyncCycle(ctx)
}

func (d *Daemon) Stop() {
	d.stopOnce.Do(func() { close(d.stopCh) })
	<-d.doneCh
}

func (d *Daemon) runSyncCycle(ctx context.Context) error {
	if !d.syncing.CompareAndSwap(false, true) {
		log.Debug().Msg("sync cycle already in progress, skipping")
		return nil
	}
	defer d.syncing.Store(false)

	if len(d.syncers) == 0 {
		log.Warn().Msg("no syncers available (auth pending)")
		return nil
	}

	log.Info().Msg("sync cycle starting")

	for _, syncer := range d.syncers {
		d.syncSourceSafe(ctx, syncer)
	}

	if err := d.refreshStatuses(ctx); err != nil {
		return err
	}
	if err := d.archiveExpired(ctx); err != nil {
		return err
	}

	log.Info().Msg("sync cycle complete")
	return nil
}

func (d *Daemon) syncSourceSafe(ctx context.Context, syncer sources.Syncer) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("source", syncer.Name()).Msg("syncer panicked, continuing")
			_ = d.db.SetSyncError(ctx, d.userID, syncer.Name(), "panic during sync")
		}
	}()

	if err := d.syncSource(ctx, syncer); err != nil {
		log.Warn().Err(err).Str("source", syncer.Name()).Msg("source sync failed, retrying in 30s")
		select {
		case <-ctx.Done():
			return
		case <-time.After(syncRetryDelay):
		}
		if retryErr := d.syncSource(ctx, syncer); retryErr != nil {
			log.Error().Err(retryErr).Str("source", syncer.Name()).Msg("source sync retry failed")
		}
	}
}

func (d *Daemon) syncSource(ctx context.Context, syncer sources.Syncer) error {
	name := syncer.Name()
	log.Info().Str("source", name).Msg("sync starting")

	if err := d.db.SetSyncStatus(ctx, d.userID, name, models.SyncStatusSyncing); err != nil {
		return err
	}

	state, err := d.db.GetSyncState(ctx, d.userID, name)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			state = &models.SyncState{UserID: d.userID, Source: name}
		} else {
			return err
		}
	}

	items, pageToken, err := syncer.Sync(ctx, state)
	if err != nil {
		log.Error().Err(err).Str("source", name).Msg("sync failed")
		_ = d.db.SetSyncError(ctx, d.userID, name, err.Error())
		return err
	}

	items = engine.DeduplicateRawItems(items)

	var processed int64
	for _, item := range items {
		if item.Type == "cancelled" {
			if err := d.handleCancellation(ctx, name, item.SourceRef); err != nil {
				log.Warn().Err(err).Str("source_ref", item.SourceRef).Msg("cancellation archive failed")
			}
			continue
		}

		if item.DueDate.IsZero() {
			log.Warn().Str("source", name).Str("source_ref", item.SourceRef).Str("title", item.Title).Msg("skipping item with zero due date")
			continue
		}

		alert := engine.Normalize(item, name, d.cfg.Alerts.Windows, d.cfg.Alerts.TTL, d.loc)
		alert.UserID = d.userID

		dup, err := engine.FindCrossSourceDuplicate(ctx, d.db, &alert)
		if err != nil {
			log.Warn().Err(err).Str("title", alert.Title).Msg("dedup check failed")
			continue
		}
		if dup != nil {
			merged := engine.MergeCrossSourceAlerts(dup, &alert)
			merged.UserID = d.userID
			if err := d.db.ReplaceAlert(ctx, merged); err != nil {
				log.Warn().Err(err).Str("title", alert.Title).Msg("merge replace failed")
			}
			continue
		}

		if err := d.db.UpsertAlert(ctx, &alert); err != nil {
			log.Warn().Err(err).Str("title", alert.Title).Msg("upsert failed")
			continue
		}
		processed++
	}

	if err := d.db.SetSyncSuccess(ctx, d.userID, name, pageToken, processed); err != nil {
		return err
	}

	log.Info().Str("source", name).Int64("processed", processed).Msg("sync complete")
	return nil
}

func (d *Daemon) handleCancellation(ctx context.Context, source, sourceRef string) error {
	alerts, err := d.db.FindAlertsBySourceRefs(ctx, source, []string{sourceRef})
	if err != nil {
		return err
	}
	for i := range alerts {
		if err := d.db.ArchiveAlert(ctx, &alerts[i], "cancelled"); err != nil {
			return err
		}
	}
	return nil
}

func (d *Daemon) refreshStatuses(ctx context.Context) error {
	result, err := d.db.ListAlerts(ctx, db.AlertFilter{
		UserID: d.userID,
		Status: []string{
			models.AlertStatusUpcoming,
			models.AlertStatusDueToday,
			models.AlertStatusMissed,
			models.AlertStatusSnoozed,
		},
		Limit: 10000,
	})
	if err != nil {
		return err
	}

	now := time.Now()
	for i := range result.Data {
		oldStatus := result.Data[i].Status
		newStatus := engine.ComputeStatus(&result.Data[i], now, d.loc)
		if newStatus == oldStatus {
			continue
		}
		extra := bson.M{}
		if newStatus != models.AlertStatusSnoozed {
			extra["snoozed_until"] = nil
		}
		if err := d.db.UpdateAlertStatus(ctx, result.Data[i].ID, newStatus, extra); err != nil {
			log.Warn().Err(err).Str("id", result.Data[i].ID.Hex()).Msg("status update failed")
		}
	}
	return nil
}

func (d *Daemon) archiveExpired(ctx context.Context) error {
	count, err := d.db.ArchiveExpiredAlerts(ctx, d.userID)
	if err != nil {
		return err
	}
	if count > 0 {
		log.Info().Int64("archived", count).Msg("expired alerts archived")
	}
	return nil
}

func (d *Daemon) runDailyRecurring(ctx context.Context) {
	log.Info().Msg("daily recurring job starting")

	now := time.Now()
	result, err := d.db.ListAlerts(ctx, db.AlertFilter{
		UserID: d.userID,
		Status: []string{models.AlertStatusAcknowledged},
		Limit:  10000,
	})
	if err != nil {
		log.Warn().Err(err).Msg("failed to list acknowledged alerts for recurring refresh")
		return
	}

	for _, alert := range result.Data {
		if alert.Recurrence == models.RecurrenceNone || !alert.DueDate.Before(now) {
			continue
		}

		next := engine.NextOccurrence(alert.DueDate, alert.Recurrence)
		if next == nil {
			continue
		}

		windowBefore := alert.WindowBefore
		if windowBefore == 0 {
			windowBefore = engine.ComputeWindowBefore(alert.Type, d.cfg.Alerts.Windows)
		}

		daysUntil := int(next.Sub(now).Hours() / 24)
		if daysUntil > windowBefore {
			continue
		}

		newAlert := alert
		newAlert.ID = primitive.NilObjectID
		newAlert.DueDate = *next
		newAlert.Status = engine.ComputeStatus(&newAlert, now, d.loc)
		newAlert.NextOccurrence = engine.NextOccurrence(*next, alert.Recurrence)
		newAlert.ExpiresAt = engine.ComputeExpiresAt(*next, alert.Type, d.cfg.Alerts.TTL)
		newAlert.AcknowledgedAt = nil
		newAlert.SnoozedUntil = nil
		newAlert.SourceRef = alert.SourceRef + ":" + next.Format("2006-01-02")
		newAlert.CreatedAt = now
		newAlert.UpdatedAt = now

		dup, err := engine.FindCrossSourceDuplicate(ctx, d.db, &newAlert)
		if err != nil {
			log.Warn().Err(err).Str("title", newAlert.Title).Msg("recurring dedup check failed")
			continue
		}
		if dup != nil {
			continue
		}

		if err := d.db.UpsertAlert(ctx, &newAlert); err != nil {
			log.Warn().Err(err).Str("title", newAlert.Title).Msg("recurring alert creation failed")
		}
	}
}

func (d *Daemon) scheduleMidnight() *time.Ticker {
	now := time.Now().In(d.loc)
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, d.loc)
	return time.NewTicker(nextMidnight.Sub(now))
}
