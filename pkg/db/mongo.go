package db

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/innacy/assistant-agent/pkg/config"
)

type MongoDB struct {
	client   *mongo.Client
	database *mongo.Database
}

func Connect(cfg config.DBConfig) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	opts := options.Client().ApplyURI(cfg.URI)
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	db := &MongoDB{
		client:   client,
		database: client.Database(cfg.Database),
	}

	if err := db.ensureIndexes(ctx); err != nil {
		return nil, err
	}

	log.Info().Str("database", cfg.Database).Msg("MongoDB connected")
	return db, nil
}

func (m *MongoDB) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return m.client.Disconnect(ctx)
}

func (m *MongoDB) Alerts() *mongo.Collection {
	return m.database.Collection("alerts")
}

func (m *MongoDB) AlertHistory() *mongo.Collection {
	return m.database.Collection("alert_history")
}

func (m *MongoDB) SyncState() *mongo.Collection {
	return m.database.Collection("sync_state")
}

func (m *MongoDB) Settings() *mongo.Collection {
	return m.database.Collection("settings")
}

func (m *MongoDB) ensureIndexes(ctx context.Context) error {
	alertIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "status", Value: 1}, {Key: "due_date", Value: 1}},
			Options: options.Index().SetName("user_status_due"),
		},
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "type", Value: 1}, {Key: "due_date", Value: 1}},
			Options: options.Index().SetName("user_type_due"),
		},
		{
			Keys:    bson.D{{Key: "source", Value: 1}, {Key: "source_ref", Value: 1}},
			Options: options.Index().SetName("source_ref_unique").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetName("ttl_expires").SetExpireAfterSeconds(0),
		},
	}

	if _, err := m.Alerts().Indexes().CreateMany(ctx, alertIndexes); err != nil {
		return err
	}

	historyIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "type", Value: 1}, {Key: "due_date", Value: 1}},
			Options: options.Index().SetName("history_user_type_due"),
		},
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "archived_at", Value: -1}},
			Options: options.Index().SetName("history_user_archived"),
		},
	}

	if _, err := m.AlertHistory().Indexes().CreateMany(ctx, historyIndexes); err != nil {
		return err
	}

	syncStateIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "source", Value: 1}},
		Options: options.Index().SetName("sync_state_unique").SetUnique(true),
	}

	if _, err := m.SyncState().Indexes().CreateOne(ctx, syncStateIndex); err != nil {
		return err
	}

	log.Info().Msg("MongoDB indexes ensured")
	return nil
}
