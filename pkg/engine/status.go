package engine

import (
	"time"

	"github.com/innacy/assistant-agent/internal/models"
)

func ComputeStatus(alert *models.Alert, now time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}

	if alert.Status == models.AlertStatusAcknowledged {
		return models.AlertStatusAcknowledged
	}
	if alert.Status == models.AlertStatusSnoozed {
		if alert.SnoozedUntil != nil && alert.SnoozedUntil.After(now) {
			return models.AlertStatusSnoozed
		}
	}

	dueDay := alert.DueDate.In(loc).Format("2006-01-02")
	today := now.In(loc).Format("2006-01-02")

	switch {
	case dueDay == today:
		return models.AlertStatusDueToday
	case dueDay < today:
		return models.AlertStatusMissed
	default:
		return models.AlertStatusUpcoming
	}
}

func RecomputeStatuses(alerts []models.Alert, loc *time.Location) []models.Alert {
	now := time.Now()
	for i := range alerts {
		alerts[i].Status = ComputeStatus(&alerts[i], now, loc)
	}
	return alerts
}
