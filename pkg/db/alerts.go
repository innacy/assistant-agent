package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/innacy/assistant-agent/internal/models"
)

type AlertFilter struct {
	UserID string
	Types  []string
	Status []string
	From   *time.Time
	To     *time.Time
	Limit  int64
	Offset int64
}

type AlertListResult struct {
	Data    []models.Alert `json:"data"`
	Total   int64          `json:"total"`
	Limit   int64          `json:"limit"`
	Offset  int64          `json:"offset"`
	HasMore bool           `json:"has_more"`
}

func (m *MongoDB) UpsertAlert(ctx context.Context, alert *models.Alert) error {
	alert.UpdatedAt = time.Now()
	if alert.CreatedAt.IsZero() {
		alert.CreatedAt = time.Now()
	}

	filter := bson.M{"source": alert.Source, "source_ref": alert.SourceRef}
	update := bson.M{
		"$set":         alert,
		"$setOnInsert": bson.M{"created_at": alert.CreatedAt},
	}
	opts := options.Update().SetUpsert(true)

	_, err := m.Alerts().UpdateOne(ctx, filter, update, opts)
	return err
}

func (m *MongoDB) GetAlert(ctx context.Context, id primitive.ObjectID) (*models.Alert, error) {
	var alert models.Alert
	err := m.Alerts().FindOne(ctx, bson.M{"_id": id}).Decode(&alert)
	if err != nil {
		return nil, err
	}
	return &alert, nil
}

func (m *MongoDB) ListAlerts(ctx context.Context, f AlertFilter) (*AlertListResult, error) {
	filter := buildAlertFilter(f)

	total, err := m.Alerts().CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "due_date", Value: 1}}).
		SetLimit(limit).
		SetSkip(f.Offset)

	cursor, err := m.Alerts().Find(ctx, filter, opts)
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

func (m *MongoDB) UpdateAlertStatus(ctx context.Context, id primitive.ObjectID, status string, extra bson.M) error {
	update := bson.M{
		"$set": bson.M{"status": status, "updated_at": time.Now()},
	}
	for k, v := range extra {
		update["$set"].(bson.M)[k] = v
	}
	_, err := m.Alerts().UpdateByID(ctx, id, update)
	return err
}

func (m *MongoDB) DeleteAlert(ctx context.Context, id primitive.ObjectID) error {
	_, err := m.Alerts().DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (m *MongoDB) FindAlertsBySourceRefs(ctx context.Context, source string, refs []string) ([]models.Alert, error) {
	cursor, err := m.Alerts().Find(ctx, bson.M{
		"source":     source,
		"source_ref": bson.M{"$in": refs},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var alerts []models.Alert
	return alerts, cursor.All(ctx, &alerts)
}

func (m *MongoDB) BulkUpdateStatus(ctx context.Context, filter bson.M, status string, extra bson.M) (int64, error) {
	update := bson.M{
		"$set": bson.M{"status": status, "updated_at": time.Now()},
	}
	for k, v := range extra {
		update["$set"].(bson.M)[k] = v
	}
	result, err := m.Alerts().UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

func buildAlertFilter(f AlertFilter) bson.M {
	filter := bson.M{"user_id": f.UserID}

	if len(f.Types) > 0 {
		filter["type"] = bson.M{"$in": f.Types}
	}
	if len(f.Status) > 0 {
		filter["status"] = bson.M{"$in": f.Status}
	}
	if f.From != nil || f.To != nil {
		dateFilter := bson.M{}
		if f.From != nil {
			dateFilter["$gte"] = *f.From
		}
		if f.To != nil {
			dateFilter["$lte"] = *f.To
		}
		filter["due_date"] = dateFilter
	}

	return filter
}
