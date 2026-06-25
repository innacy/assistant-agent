package tasks

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/api/option"
	tasks "google.golang.org/api/tasks/v1"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/sources"
)

type Syncer struct {
	client *http.Client
}

func New(client *http.Client) *Syncer {
	return &Syncer{client: client}
}

func (s *Syncer) Name() string { return models.SourceTasks }

func (s *Syncer) Sync(ctx context.Context, state *models.SyncState) ([]sources.RawItem, string, error) {
	srv, err := tasks.NewService(ctx, option.WithHTTPClient(s.client))
	if err != nil {
		return nil, "", fmt.Errorf("tasks service: %w", err)
	}

	taskLists, err := srv.Tasklists.List().Context(ctx).Do()
	if err != nil {
		return nil, "", fmt.Errorf("list task lists: %w", err)
	}

	var items []sources.RawItem
	now := time.Now()
	lookback := now.AddDate(0, -1, 0)

	for _, tl := range taskLists.Items {
		call := srv.Tasks.List(tl.Id).
			ShowCompleted(false).
			ShowHidden(false).
			DueMin(lookback.Format(time.RFC3339)).
			Context(ctx)

		result, err := call.Do()
		if err != nil {
			log.Warn().Err(err).Str("list", tl.Title).Msg("failed to fetch tasks")
			continue
		}

		for _, task := range result.Items {
			if task.Due == "" {
				continue
			}

			dueDate, err := time.Parse(time.RFC3339, task.Due)
			if err != nil {
				continue
			}

			priority := inferPriority(dueDate, now)

			items = append(items, sources.RawItem{
				Type:        models.AlertTypeTask,
				Title:       task.Title,
				Description: task.Notes,
				DueDate:     dueDate,
				Recurrence:  models.RecurrenceNone,
				SourceRef:   task.Id,
				SourceRaw:   task.Title,
				Priority:    priority,
				Metadata: map[string]interface{}{
					"task_list":   tl.Title,
					"task_status": task.Status,
				},
			})
		}
	}

	log.Info().Int("tasks_fetched", len(items)).Msg("tasks sync complete")
	return items, "", nil
}

func (s *Syncer) FetchActiveTaskIDs(ctx context.Context) (map[string]bool, error) {
	srv, err := tasks.NewService(ctx, option.WithHTTPClient(s.client))
	if err != nil {
		return nil, err
	}

	ids := make(map[string]bool)
	taskLists, err := srv.Tasklists.List().Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	for _, tl := range taskLists.Items {
		result, err := srv.Tasks.List(tl.Id).
			ShowCompleted(false).
			Context(ctx).
			Do()
		if err != nil {
			continue
		}
		for _, task := range result.Items {
			ids[task.Id] = true
		}
	}

	return ids, nil
}

func inferPriority(dueDate, now time.Time) string {
	daysUntil := int(dueDate.Sub(now).Hours() / 24)
	switch {
	case daysUntil < 0:
		return models.PriorityHigh
	case daysUntil <= 2:
		return models.PriorityMedium
	default:
		return models.PriorityLow
	}
}
