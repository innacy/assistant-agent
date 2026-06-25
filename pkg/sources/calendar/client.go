package calendar

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/sources"
)

var paymentKeywords = []string{"pay", "due", "bill", "emi", "renew", "deadline", "expires", "appointment"}

type Syncer struct {
	client *http.Client
}

func New(client *http.Client) *Syncer {
	return &Syncer{client: client}
}

func (s *Syncer) Name() string { return models.SourceCalendar }

func (s *Syncer) Sync(ctx context.Context, state *models.SyncState) ([]sources.RawItem, string, error) {
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(s.client))
	if err != nil {
		return nil, "", fmt.Errorf("calendar service: %w", err)
	}

	var items []sources.RawItem
	var nextSyncToken string

	calendars := []string{"primary", "addressbook#contacts@group.v.calendar.google.com"}

	for _, calID := range calendars {
		fetched, token, err := s.syncCalendar(ctx, srv, calID, state, calID != "primary")
		if err != nil {
			log.Warn().Err(err).Str("calendar", calID).Msg("calendar sync failed")
			continue
		}
		items = append(items, fetched...)
		if calID == "primary" {
			nextSyncToken = token
		}
	}

	return items, nextSyncToken, nil
}

func (s *Syncer) syncCalendar(ctx context.Context, srv *calendar.Service, calID string, state *models.SyncState, isBirthdayCalendar bool) ([]sources.RawItem, string, error) {
	var items []sources.RawItem
	var nextSyncToken string

	call := srv.Events.List(calID).
		SingleEvents(true).
		OrderBy("startTime").
		Context(ctx)

	if state != nil && state.LastPageToken != "" && !isBirthdayCalendar {
		call = call.SyncToken(state.LastPageToken)
	} else {
		now := time.Now()
		call = call.
			TimeMin(now.AddDate(0, -1, 0).Format(time.RFC3339)).
			TimeMax(now.AddDate(0, 3, 0).Format(time.RFC3339))
	}

	err := call.Pages(ctx, func(page *calendar.Events) error {
		for _, event := range page.Items {
			if event.Status == "cancelled" {
				items = append(items, sources.RawItem{
					Type:      "cancelled",
					SourceRef: eventSourceRef(event),
				})
				continue
			}

			item := s.eventToRawItem(event, isBirthdayCalendar)
			if item != nil {
				items = append(items, *item)
			}
		}
		if page.NextSyncToken != "" {
			nextSyncToken = page.NextSyncToken
		}
		return nil
	})

	return items, nextSyncToken, err
}

func (s *Syncer) eventToRawItem(event *calendar.Event, isBirthdayCalendar bool) *sources.RawItem {
	dueDate := parseEventTime(event)
	if dueDate.IsZero() {
		return nil
	}

	title := event.Summary
	isAllDay := event.Start.Date != ""

	if isBirthdayCalendar {
		return &sources.RawItem{
			Type:        models.AlertTypeBirthday,
			Title:       title,
			Description: event.Description,
			DueDate:     dueDate,
			Recurrence:  models.RecurrenceYearly,
			SourceRef:   eventSourceRef(event),
			SourceRaw:   title,
			Priority:    models.PriorityMedium,
			Metadata:    map[string]interface{}{"calendar": "birthdays"},
		}
	}

	if !isAllDay && !matchesKeywords(title) {
		return nil
	}

	alertType := classifyEvent(title, event.Recurrence)
	recurrence := models.RecurrenceNone
	if len(event.Recurrence) > 0 {
		recurrence = inferRecurrence(event.Recurrence)
	}

	return &sources.RawItem{
		Type:        alertType,
		Title:       title,
		Description: event.Description,
		DueDate:     dueDate,
		Recurrence:  recurrence,
		SourceRef:   eventSourceRef(event),
		SourceRaw:   title,
		Priority:    models.PriorityMedium,
	}
}

func eventSourceRef(event *calendar.Event) string {
	start := parseEventTime(event)
	if !start.IsZero() {
		return fmt.Sprintf("%s:%s", event.Id, start.Format("2006-01-02"))
	}
	return event.Id
}

func parseEventTime(event *calendar.Event) time.Time {
	if event.Start == nil {
		return time.Time{}
	}
	if event.Start.Date != "" {
		t, _ := time.Parse("2006-01-02", event.Start.Date)
		return t
	}
	if event.Start.DateTime != "" {
		t, _ := time.Parse(time.RFC3339, event.Start.DateTime)
		return t
	}
	return time.Time{}
}

func matchesKeywords(title string) bool {
	lower := strings.ToLower(title)
	for _, kw := range paymentKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func classifyEvent(title string, recurrence []string) string {
	lower := strings.ToLower(title)
	for _, kw := range []string{"pay", "due", "bill", "emi"} {
		if strings.Contains(lower, kw) {
			return models.AlertTypePayment
		}
	}
	if len(recurrence) > 0 && (strings.Contains(lower, "renew") || strings.Contains(lower, "subscription")) {
		return models.AlertTypeSubscription
	}
	return models.AlertTypeEvent
}

func inferRecurrence(rules []string) string {
	for _, rule := range rules {
		lower := strings.ToLower(rule)
		if strings.Contains(lower, "yearly") {
			return models.RecurrenceYearly
		}
		if strings.Contains(lower, "monthly") {
			return models.RecurrenceMonthly
		}
		if strings.Contains(lower, "weekly") {
			return models.RecurrenceWeekly
		}
	}
	return models.RecurrenceCustom
}
