package db

import (
	"os"
	"testing"
	"time"

	"github.com/innacy/assistant-agent/pkg/config"
)

func TestConnect(t *testing.T) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	cfg := config.DBConfig{
		URI:      uri,
		Database: "assistant-agent-test",
		Timeout:  5 * time.Second,
	}

	db, err := Connect(cfg)
	if err != nil {
		t.Skipf("MongoDB not available: %v", err)
	}
	defer db.Close()

	if db.Alerts() == nil {
		t.Error("Alerts collection is nil")
	}
	if db.AlertHistory() == nil {
		t.Error("AlertHistory collection is nil")
	}
	if db.SyncState() == nil {
		t.Error("SyncState collection is nil")
	}
	if db.Settings() == nil {
		t.Error("Settings collection is nil")
	}
}
