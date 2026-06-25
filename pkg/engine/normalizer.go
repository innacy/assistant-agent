package engine

import (
	"time"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/sources"
)

func Normalize(item sources.RawItem, source string, windowConfig, ttlConfig map[string]int) models.Alert {
	now := time.Now()

	priority := item.Priority
	if priority == "" {
		priority = models.PriorityMedium
	}

	recurrence := item.Recurrence
	if recurrence == "" {
		recurrence = models.RecurrenceNone
	}

	currency := item.Currency
	if currency == "" {
		currency = "INR"
	}

	alert := models.Alert{
		UserID:       "default",
		Type:         item.Type,
		Title:        item.Title,
		Description:  item.Description,
		DueDate:      item.DueDate,
		Recurrence:   recurrence,
		Amount:       item.Amount,
		Currency:     currency,
		Source:       source,
		SourceRef:    item.SourceRef,
		SourceRaw:    item.SourceRaw,
		Priority:     priority,
		WindowBefore: ComputeWindowBefore(item.Type, windowConfig),
		ExpiresAt:    ComputeExpiresAt(item.DueDate, item.Type, ttlConfig),
		Tags:         []string{},
		Metadata:     item.Metadata,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	alert.Status = ComputeStatus(&alert, now)

	if recurrence != models.RecurrenceNone {
		alert.NextOccurrence = NextOccurrence(item.DueDate, recurrence)
	}

	return alert
}
