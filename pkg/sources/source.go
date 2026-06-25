package sources

import (
	"context"
	"time"

	"github.com/innacy/assistant-agent/internal/models"
)

type Syncer interface {
	Name() string
	Sync(ctx context.Context, state *models.SyncState) ([]RawItem, string, error)
}

type RawItem struct {
	Type        string
	Title       string
	Description string
	DueDate     time.Time
	Amount      *float64
	Currency    string
	Recurrence  string
	SourceRef   string
	SourceRaw   string
	Priority    string
	Metadata    map[string]interface{}
}
