package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	AlertTypeBirthday     = "birthday"
	AlertTypeSubscription = "subscription"
	AlertTypePayment      = "payment"
	AlertTypeTask         = "task"
	AlertTypeEvent        = "event"

	AlertStatusUpcoming     = "upcoming"
	AlertStatusDueToday     = "due_today"
	AlertStatusMissed       = "missed"
	AlertStatusSnoozed      = "snoozed"
	AlertStatusAcknowledged = "acknowledged"

	SourceGmail    = "gmail"
	SourceCalendar = "calendar"
	SourceTasks    = "tasks"
	SourceContacts = "contacts"
	SourceManual   = "manual"

	RecurrenceNone    = "none"
	RecurrenceWeekly  = "weekly"
	RecurrenceMonthly = "monthly"
	RecurrenceYearly  = "yearly"
	RecurrenceCustom  = "custom"

	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"
)

type Alert struct {
	ID             primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	UserID         string                 `bson:"user_id" json:"user_id"`
	Type           string                 `bson:"type" json:"type"`
	Title          string                 `bson:"title" json:"title"`
	Description    string                 `bson:"description,omitempty" json:"description,omitempty"`
	DueDate        time.Time              `bson:"due_date" json:"due_date"`
	Recurrence     string                 `bson:"recurrence" json:"recurrence"`
	NextOccurrence *time.Time             `bson:"next_occurrence,omitempty" json:"next_occurrence,omitempty"`
	Amount         *float64               `bson:"amount,omitempty" json:"amount,omitempty"`
	Currency       string                 `bson:"currency,omitempty" json:"currency,omitempty"`
	Source         string                 `bson:"source" json:"source"`
	SourceRef      string                 `bson:"source_ref" json:"source_ref"`
	SourceRaw      string                 `bson:"source_raw,omitempty" json:"source_raw,omitempty"`
	Status         string                 `bson:"status" json:"status"`
	Priority       string                 `bson:"priority" json:"priority"`
	WindowBefore   int                    `bson:"window_before" json:"window_before"`
	ExpiresAt      time.Time              `bson:"expires_at" json:"expires_at"`
	AcknowledgedAt *time.Time             `bson:"acknowledged_at,omitempty" json:"acknowledged_at,omitempty"`
	SnoozedUntil   *time.Time             `bson:"snoozed_until,omitempty" json:"snoozed_until,omitempty"`
	Tags           []string               `bson:"tags,omitempty" json:"tags,omitempty"`
	Metadata       map[string]interface{} `bson:"metadata,omitempty" json:"metadata,omitempty"`
	CreatedAt      time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt      time.Time              `bson:"updated_at" json:"updated_at"`
}

type AlertHistory struct {
	Alert      `bson:",inline"`
	ArchivedAt time.Time `bson:"archived_at" json:"archived_at"`
	Outcome    string    `bson:"outcome" json:"outcome"`
}
