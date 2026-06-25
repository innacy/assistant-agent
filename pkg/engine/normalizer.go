package engine

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/sources"
)

const (
	maxTitleLen = 200
	maxDescLen  = 2000
)

func DeduplicateRawItems(items []sources.RawItem) []sources.RawItem {
	seen := make(map[string]struct{}, len(items))
	out := make([]sources.RawItem, 0, len(items))
	for _, item := range items {
		if item.SourceRef == "" {
			out = append(out, item)
			continue
		}
		if _, ok := seen[item.SourceRef]; ok {
			continue
		}
		seen[item.SourceRef] = struct{}{}
		out = append(out, item)
	}
	return out
}

func Normalize(item sources.RawItem, source string, windowConfig, ttlConfig map[string]int, loc *time.Location) models.Alert {
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

	title := truncateRunes(strings.TrimSpace(item.Title), maxTitleLen)
	description := truncateRunes(strings.TrimSpace(item.Description), maxDescLen)

	alert := models.Alert{
		UserID:       "default",
		Type:         item.Type,
		Title:        title,
		Description:  description,
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

	alert.Status = ComputeStatus(&alert, now, loc)

	if recurrence != models.RecurrenceNone {
		alert.NextOccurrence = NextOccurrence(item.DueDate, recurrence)
	}

	return alert
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}
