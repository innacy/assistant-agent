package engine

import (
	"time"

	"github.com/innacy/assistant-agent/internal/models"
)

func ComputeStatus(alert *models.Alert, now time.Time) string {
	if alert.Status == models.AlertStatusAcknowledged {
		return models.AlertStatusAcknowledged
	}
	if alert.Status == models.AlertStatusSnoozed {
		if alert.SnoozedUntil != nil && alert.SnoozedUntil.After(now) {
			return models.AlertStatusSnoozed
		}
	}

	dueDay := alert.DueDate.Truncate(24 * time.Hour)
	today := now.Truncate(24 * time.Hour)

	switch {
	case dueDay.Equal(today):
		return models.AlertStatusDueToday
	case dueDay.Before(today):
		return models.AlertStatusMissed
	default:
		return models.AlertStatusUpcoming
	}
}

func RecomputeStatuses(alerts []models.Alert) []models.Alert {
	now := time.Now()
	for i := range alerts {
		alerts[i].Status = ComputeStatus(&alerts[i], now)
	}
	return alerts
}
