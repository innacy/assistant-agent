package contacts

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/sources"
)

type Syncer struct {
	client *http.Client
}

func New(client *http.Client) *Syncer {
	return &Syncer{client: client}
}

func (s *Syncer) Name() string { return models.SourceContacts }

func (s *Syncer) Sync(ctx context.Context, state *models.SyncState) ([]sources.RawItem, string, error) {
	srv, err := people.NewService(ctx, option.WithHTTPClient(s.client))
	if err != nil {
		return nil, "", fmt.Errorf("people service: %w", err)
	}

	var items []sources.RawItem
	var nextPageToken string

	call := srv.People.Connections.List("people/me").
		PersonFields("names,birthdays").
		PageSize(1000)

	if state != nil && state.LastPageToken != "" {
		call = call.SyncToken(state.LastPageToken)
	}

	err = call.Pages(ctx, func(resp *people.ListConnectionsResponse) error {
		for _, person := range resp.Connections {
			item := personToRawItem(person)
			if item != nil {
				items = append(items, *item)
			}
		}
		if resp.NextSyncToken != "" {
			nextPageToken = resp.NextSyncToken
		}
		return nil
	})

	if err != nil {
		return nil, "", err
	}

	log.Info().Int("contacts_with_birthdays", len(items)).Msg("contacts sync complete")
	return items, nextPageToken, nil
}

func personToRawItem(person *people.Person) *sources.RawItem {
	if len(person.Birthdays) == 0 {
		return nil
	}

	bday := person.Birthdays[0].Date
	if bday == nil || bday.Month == 0 || bday.Day == 0 {
		return nil
	}

	name := "Unknown"
	if len(person.Names) > 0 {
		name = person.Names[0].DisplayName
	}

	nextBirthday := computeNextBirthday(int(bday.Month), int(bday.Day))
	year := nextBirthday.Year()

	metadata := map[string]interface{}{
		"contact_name": name,
		"birth_month":  bday.Month,
		"birth_day":    bday.Day,
	}
	if bday.Year != 0 {
		metadata["birth_year"] = bday.Year
	}

	return &sources.RawItem{
		Type:        models.AlertTypeBirthday,
		Title:       fmt.Sprintf("%s's Birthday", name),
		Description: formatBirthdayDescription(name, bday),
		DueDate:     nextBirthday,
		Recurrence:  models.RecurrenceYearly,
		SourceRef:   fmt.Sprintf("%s:%d", person.ResourceName, year),
		SourceRaw:   name,
		Priority:    models.PriorityMedium,
		Metadata:    metadata,
	}
}

func computeNextBirthday(month, day int) time.Time {
	now := time.Now()
	thisYear := time.Date(now.Year(), time.Month(month), day, 0, 0, 0, 0, now.Location())
	if thisYear.Before(now.Truncate(24 * time.Hour)) {
		return thisYear.AddDate(1, 0, 0)
	}
	return thisYear
}

func formatBirthdayDescription(name string, date *people.Date) string {
	if date.Year != 0 {
		age := time.Now().Year() - int(date.Year)
		return fmt.Sprintf("%s turns %d", name, age)
	}
	return fmt.Sprintf("%s's birthday on %s %d", name, time.Month(date.Month).String(), date.Day)
}
