package db

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/innacy/assistant-agent/internal/models"
)

func (m *MongoDB) GetSettings(ctx context.Context, userID string) (*models.Settings, error) {
	var settings models.Settings
	err := m.Settings().FindOne(ctx, bson.M{"user_id": userID}).Decode(&settings)
	if err == mongo.ErrNoDocuments {
		defaults := models.DefaultSettings(userID)
		_, _ = m.Settings().InsertOne(ctx, defaults)
		return defaults, nil
	}
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (m *MongoDB) UpdateSettings(ctx context.Context, settings *models.Settings) error {
	filter := bson.M{"user_id": settings.UserID}
	update := bson.M{"$set": settings}
	opts := options.Update().SetUpsert(true)
	_, err := m.Settings().UpdateOne(ctx, filter, update, opts)
	return err
}
