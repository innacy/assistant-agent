package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/innacy/assistant-agent/internal/models"
)

func (m *MongoDB) ArchiveAlert(ctx context.Context, alert *models.Alert, outcome string) error {
	history := models.AlertHistory{
		Alert:      *alert,
		ArchivedAt: time.Now(),
		Outcome:    outcome,
	}

	_, err := m.AlertHistory().InsertOne(ctx, history)
	if err != nil {
		return err
	}

	return m.DeleteAlert(ctx, alert.ID)
}

func (m *MongoDB) ArchiveExpiredAlerts(ctx context.Context, userID string) (int64, error) {
	now := time.Now()
	filter := bson.M{
		"user_id":    userID,
		"expires_at": bson.M{"$lte": now},
		"status":     bson.M{"$ne": models.AlertStatusAcknowledged},
	}

	cursor, err := m.Alerts().Find(ctx, filter)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	var count int64
	for cursor.Next(ctx) {
		var alert models.Alert
		if err := cursor.Decode(&alert); err != nil {
			continue
		}
		if err := m.ArchiveAlert(ctx, &alert, "expired_unacknowledged"); err == nil {
			count++
		}
	}
	return count, nil
}

func (m *MongoDB) ListHistory(ctx context.Context, f AlertFilter) (*AlertListResult, error) {
	filter := buildAlertFilter(f)

	total, err := m.AlertHistory().CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "due_date", Value: -1}}).
		SetLimit(limit).
		SetSkip(f.Offset)

	cursor, err := m.AlertHistory().Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var alerts []models.Alert
	if err := cursor.All(ctx, &alerts); err != nil {
		return nil, err
	}
	if alerts == nil {
		alerts = []models.Alert{}
	}

	return &AlertListResult{
		Data:    alerts,
		Total:   total,
		Limit:   limit,
		Offset:  f.Offset,
		HasMore: f.Offset+int64(len(alerts)) < total,
	}, nil
}
