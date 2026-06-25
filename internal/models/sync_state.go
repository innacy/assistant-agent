package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	SyncStatusIdle    = "idle"
	SyncStatusSyncing = "syncing"
	SyncStatusError   = "error"
)

type SyncState struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID         string             `bson:"user_id" json:"user_id"`
	Source         string             `bson:"source" json:"source"`
	LastSyncAt     *time.Time         `bson:"last_sync_at,omitempty" json:"last_sync_at,omitempty"`
	LastPageToken  string             `bson:"last_page_token,omitempty" json:"last_page_token,omitempty"`
	TotalProcessed int64              `bson:"total_processed" json:"total_processed"`
	LastError      string             `bson:"last_error,omitempty" json:"last_error,omitempty"`
	Status         string             `bson:"status" json:"status"`
}
