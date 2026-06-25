package engine

import (
	"testing"
	"time"

	"github.com/innacy/assistant-agent/internal/models"
)

func TestComputeStatus(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		alert models.Alert
		want  string
	}{
		{
			name:  "future date is upcoming",
			alert: models.Alert{DueDate: now.AddDate(0, 0, 3), Status: models.AlertStatusUpcoming},
			want:  models.AlertStatusUpcoming,
		},
		{
			name:  "today is due_today",
			alert: models.Alert{DueDate: now, Status: models.AlertStatusUpcoming},
			want:  models.AlertStatusDueToday,
		},
		{
			name:  "past date is missed",
			alert: models.Alert{DueDate: now.AddDate(0, 0, -2), Status: models.AlertStatusUpcoming},
			want:  models.AlertStatusMissed,
		},
		{
			name:  "acknowledged stays acknowledged",
			alert: models.Alert{DueDate: now.AddDate(0, 0, -2), Status: models.AlertStatusAcknowledged},
			want:  models.AlertStatusAcknowledged,
		},
		{
			name: "snoozed with future snooze stays snoozed",
			alert: models.Alert{
				DueDate:      now.AddDate(0, 0, -1),
				Status:       models.AlertStatusSnoozed,
				SnoozedUntil: timePtr(now.AddDate(0, 0, 1)),
			},
			want: models.AlertStatusSnoozed,
		},
		{
			name: "snoozed with expired snooze becomes missed",
			alert: models.Alert{
				DueDate:      now.AddDate(0, 0, -3),
				Status:       models.AlertStatusSnoozed,
				SnoozedUntil: timePtr(now.AddDate(0, 0, -1)),
			},
			want: models.AlertStatusMissed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeStatus(&tt.alert, now)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
