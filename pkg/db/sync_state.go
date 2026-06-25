package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/innacy/assistant-agent/internal/models"
)

func (m *MongoDB) GetSyncState(ctx context.Context, userID, source string) (*models.SyncState, error) {
	var state models.SyncState
	err := m.SyncState().FindOne(ctx, bson.M{
		"user_id": userID,
		"source":  source,
	}).Decode(&state)
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (m *MongoDB) UpsertSyncState(ctx context.Context, state *models.SyncState) error {
	filter := bson.M{"user_id": state.UserID, "source": state.Source}
	update := bson.M{"$set": state}
	opts := options.Update().SetUpsert(true)
	_, err := m.SyncState().UpdateOne(ctx, filter, update, opts)
	return err
}

func (m *MongoDB) SetSyncStatus(ctx context.Context, userID, source, status string) error {
	filter := bson.M{"user_id": userID, "source": source}
	update := bson.M{"$set": bson.M{"status": status}}
	opts := options.Update().SetUpsert(true)
	_, err := m.SyncState().UpdateOne(ctx, filter, update, opts)
	return err
}

func (m *MongoDB) SetSyncError(ctx context.Context, userID, source, errMsg string) error {
	filter := bson.M{"user_id": userID, "source": source}
	update := bson.M{"$set": bson.M{
		"status":     models.SyncStatusError,
		"last_error": errMsg,
	}}
	opts := options.Update().SetUpsert(true)
	_, err := m.SyncState().UpdateOne(ctx, filter, update, opts)
	return err
}

func (m *MongoDB) SetSyncSuccess(ctx context.Context, userID, source, pageToken string, processed int64) error {
	now := time.Now()
	filter := bson.M{"user_id": userID, "source": source}
	update := bson.M{"$set": bson.M{
		"status":          models.SyncStatusIdle,
		"last_sync_at":    now,
		"last_page_token": pageToken,
		"last_error":      "",
	}, "$inc": bson.M{
		"total_processed": processed,
	}}
	opts := options.Update().SetUpsert(true)
	_, err := m.SyncState().UpdateOne(ctx, filter, update, opts)
	return err
}

func (m *MongoDB) GetAllSyncStates(ctx context.Context, userID string) ([]models.SyncState, error) {
	cursor, err := m.SyncState().Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var states []models.SyncState
	return states, cursor.All(ctx, &states)
}
