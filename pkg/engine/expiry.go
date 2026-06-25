package engine

import (
	"time"

	"github.com/innacy/assistant-agent/internal/models"
)

func ComputeExpiresAt(dueDate time.Time, alertType string, ttlConfig map[string]int) time.Time {
	days, ok := ttlConfig[alertType]
	if !ok {
		days = 7
	}
	return dueDate.AddDate(0, 0, days)
}

func ComputeWindowBefore(alertType string, windowConfig map[string]int) int {
	days, ok := windowConfig[alertType]
	if !ok {
		return 3
	}
	return days
}

func NextOccurrence(dueDate time.Time, recurrence string) *time.Time {
	var next time.Time
	now := time.Now()

	switch recurrence {
	case models.RecurrenceYearly:
		next = time.Date(now.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, dueDate.Location())
		if next.Before(now) {
			next = next.AddDate(1, 0, 0)
		}
	case models.RecurrenceMonthly:
		next = time.Date(now.Year(), now.Month(), dueDate.Day(), 0, 0, 0, 0, dueDate.Location())
		if next.Before(now) {
			next = next.AddDate(0, 1, 0)
		}
	case models.RecurrenceWeekly:
		next = now.AddDate(0, 0, 7-int(now.Weekday())+int(dueDate.Weekday()))
		if next.Before(now) {
			next = next.AddDate(0, 0, 7)
		}
	default:
		return nil
	}
	return &next
}
