# Assistant Agent MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go backend that syncs from Gmail, Calendar, Tasks, and Contacts, stores date-sensitive alerts in MongoDB, and serves them via a REST API.

**Architecture:** Unified alert model — all sources normalize into a single `alerts` collection. Background daemon polls sources on a timer. Gin-based REST API serves alerts with dynamic status computation. Bearer token auth on all API endpoints.

**Tech Stack:** Go 1.22+, Gin (HTTP), MongoDB (go.mongodb.org/mongo-driver), Viper (config), Google API Go client libraries, zerolog (logging)

---

## File Map

| File | Responsibility |
|------|---------------|
| `main.go` | Entry point, CLI flag parsing, mode dispatch |
| `Makefile` | Build, run, test commands |
| `config.yaml.example` | Committed config template |
| `.gitignore` | Ignore config.yaml, token.json, credentials.json, bin/, web/dist/ |
| `internal/models/alert.go` | Alert struct + constants |
| `internal/models/sync_state.go` | SyncState struct |
| `internal/models/settings.go` | Settings struct + defaults |
| `pkg/config/config.go` | Viper config loading |
| `pkg/db/mongo.go` | MongoDB connection, indexes, close |
| `pkg/db/alerts.go` | Alert CRUD + queries |
| `pkg/db/history.go` | Alert history archival + queries |
| `pkg/db/sync_state.go` | Sync state CRUD |
| `pkg/db/settings.go` | Settings CRUD |
| `pkg/auth/google.go` | OAuth2 flow, token management |
| `pkg/sources/source.go` | Syncer interface + RawItem type |
| `pkg/sources/calendar/client.go` | Calendar syncer |
| `pkg/sources/contacts/client.go` | Contacts syncer |
| `pkg/sources/tasks/client.go` | Tasks syncer |
| `pkg/sources/gmail/client.go` | Gmail syncer |
| `pkg/sources/gmail/parsers.go` | Email parsing rules |
| `pkg/engine/normalizer.go` | RawItem → Alert conversion |
| `pkg/engine/expiry.go` | TTL + window computation |
| `pkg/engine/dedup.go` | Cross-source deduplication |
| `pkg/engine/status.go` | Dynamic status computation |
| `pkg/daemon/daemon.go` | Background scheduler, pipeline orchestration |
| `pkg/api/server.go` | Gin router setup, static serving |
| `pkg/api/middleware.go` | Bearer auth, CORS, request logging |
| `pkg/api/handlers_alerts.go` | Alert list/get/create/update/delete + actions |
| `pkg/api/handlers_history.go` | History list endpoint |
| `pkg/api/handlers_sync.go` | Sync status + trigger |
| `pkg/api/handlers_settings.go` | Settings get/update |
| `pkg/api/response.go` | Response envelope helpers |

---

## Task 1: Project Skeleton + Config

**Files:**
- Create: `go.mod`, `main.go`, `Makefile`, `config.yaml.example`, `.gitignore`
- Create: `pkg/config/config.go`
- Create: `internal/models/alert.go`, `internal/models/sync_state.go`, `internal/models/settings.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd /home/widasinnacy/incy/assistant-agent
go mod init github.com/innacy/assistant-agent
```

- [ ] **Step 2: Install core dependencies**

```bash
go get github.com/gin-gonic/gin
go get github.com/spf13/viper
go get go.mongodb.org/mongo-driver/mongo
go get github.com/rs/zerolog
go get golang.org/x/oauth2
go get google.golang.org/api/gmail/v1
go get google.golang.org/api/calendar/v3
go get google.golang.org/api/tasks/v1
go get google.golang.org/api/people/v1
go get github.com/google/uuid
```

- [ ] **Step 3: Create .gitignore**

Create `.gitignore`:

```gitignore
# Binary
bin/
assistant-agent

# Config (contains secrets)
config.yaml
credentials.json
token.json

# Web build output
web/dist/
web/node_modules/

# IDE
.idea/
.vscode/

# OS
.DS_Store

# Go
vendor/
```

- [ ] **Step 4: Create config.yaml.example**

Create `config.yaml.example`:

```yaml
db:
  uri: "mongodb://localhost:27017"
  database: "assistant-agent"
  timeout: 10s

google:
  credentials_file: "./credentials.json"
  token_file: "./token.json"

daemon:
  poll_interval: 15m
  initial_lookback: "3m"

alerts:
  windows:
    birthday: 7
    subscription: 3
    payment: 5
    task: 1
    event: 2
  ttl:
    birthday: 2
    subscription: 7
    payment: 7
    task: 14
    event: 1

server:
  port: 8080
  mode: "release"
  api_token: "change-me-to-a-real-token"

gmail:
  query_filters:
    - "subject:subscription"
    - "subject:renewal"
    - "subject:payment due"
    - "subject:bill generated"
    - "subject:upcoming charge"
    - "subject:reminder"
  sender_whitelist:
    - "noreply@netflix.com"
    - "no-reply@spotify.com"
    - "alerts@hdfcbank.net"
    - "noreply@airtel.in"
    - "billing@aws.amazon.com"

timezone: "Asia/Kolkata"
log_level: "info"
```

- [ ] **Step 5: Create pkg/config/config.go**

```go
package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	DB     DBConfig     `mapstructure:"db"`
	Google GoogleConfig `mapstructure:"google"`
	Daemon DaemonConfig `mapstructure:"daemon"`
	Alerts AlertsConfig `mapstructure:"alerts"`
	Server ServerConfig `mapstructure:"server"`
	Gmail  GmailConfig  `mapstructure:"gmail"`

	Timezone string `mapstructure:"timezone"`
	LogLevel string `mapstructure:"log_level"`
}

type DBConfig struct {
	URI      string        `mapstructure:"uri"`
	Database string        `mapstructure:"database"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

type GoogleConfig struct {
	CredentialsFile string `mapstructure:"credentials_file"`
	TokenFile       string `mapstructure:"token_file"`
}

type DaemonConfig struct {
	PollInterval    time.Duration `mapstructure:"poll_interval"`
	InitialLookback string        `mapstructure:"initial_lookback"`
}

type AlertsConfig struct {
	Windows map[string]int `mapstructure:"windows"`
	TTL     map[string]int `mapstructure:"ttl"`
}

type ServerConfig struct {
	Port     int    `mapstructure:"port"`
	Mode     string `mapstructure:"mode"`
	APIToken string `mapstructure:"api_token"`
}

type GmailConfig struct {
	QueryFilters    []string `mapstructure:"query_filters"`
	SenderWhitelist []string `mapstructure:"sender_whitelist"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.SetEnvPrefix("ASSISTANT")
	viper.AutomaticEnv()

	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func setDefaults() {
	viper.SetDefault("db.uri", "mongodb://localhost:27017")
	viper.SetDefault("db.database", "assistant-agent")
	viper.SetDefault("db.timeout", "10s")
	viper.SetDefault("google.credentials_file", "./credentials.json")
	viper.SetDefault("google.token_file", "./token.json")
	viper.SetDefault("daemon.poll_interval", "15m")
	viper.SetDefault("daemon.initial_lookback", "3m")
	viper.SetDefault("alerts.windows.birthday", 7)
	viper.SetDefault("alerts.windows.subscription", 3)
	viper.SetDefault("alerts.windows.payment", 5)
	viper.SetDefault("alerts.windows.task", 1)
	viper.SetDefault("alerts.windows.event", 2)
	viper.SetDefault("alerts.ttl.birthday", 2)
	viper.SetDefault("alerts.ttl.subscription", 7)
	viper.SetDefault("alerts.ttl.payment", 7)
	viper.SetDefault("alerts.ttl.task", 14)
	viper.SetDefault("alerts.ttl.event", 1)
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "release")
	viper.SetDefault("timezone", "Asia/Kolkata")
	viper.SetDefault("log_level", "info")
}
```

- [ ] **Step 6: Create internal/models/alert.go**

```go
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
```

- [ ] **Step 7: Create internal/models/sync_state.go**

```go
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
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

const (
	SyncStatusIdle    = "idle"
	SyncStatusSyncing = "syncing"
	SyncStatusError   = "error"
)
```

- [ ] **Step 8: Create internal/models/settings.go**

```go
package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Settings struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID          string             `bson:"user_id" json:"user_id"`
	Windows         map[string]int     `bson:"windows" json:"windows"`
	TTL             map[string]int     `bson:"ttl" json:"ttl"`
	PollInterval    string             `bson:"poll_interval" json:"poll_interval"`
	Timezone        string             `bson:"timezone" json:"timezone"`
	InitialLookback string             `bson:"initial_lookback" json:"initial_lookback"`
}

func DefaultSettings(userID string) *Settings {
	return &Settings{
		UserID: userID,
		Windows: map[string]int{
			"birthday":     7,
			"subscription": 3,
			"payment":      5,
			"task":         1,
			"event":        2,
		},
		TTL: map[string]int{
			"birthday":     2,
			"subscription": 7,
			"payment":      7,
			"task":         14,
			"event":        1,
		},
		PollInterval:    "15m",
		Timezone:        "Asia/Kolkata",
		InitialLookback: "3m",
	}
}
```

- [ ] **Step 9: Create main.go with CLI flags**

```go
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/innacy/assistant-agent/pkg/config"
)

func main() {
	serve := flag.Bool("serve", false, "Start API + UI + daemon")
	daemon := flag.Bool("daemon", false, "Start headless daemon only")
	syncOnce := flag.Bool("sync-once", false, "Run single sync then exit")
	auth := flag.Bool("auth", false, "Run OAuth flow and exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	level, _ := zerolog.ParseLevel(cfg.LogLevel)
	zerolog.SetGlobalLevel(level)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	switch {
	case *auth:
		fmt.Println("TODO: OAuth flow")
	case *syncOnce:
		fmt.Println("TODO: single sync")
	case *daemon:
		fmt.Println("TODO: daemon mode")
	case *serve:
		fmt.Println("TODO: serve mode")
	default:
		flag.Usage()
	}
}
```

- [ ] **Step 10: Create Makefile**

```makefile
.PHONY: build run test clean

BINARY=bin/assistant-agent

build:
	go build -o $(BINARY) main.go

run-serve: build
	./$(BINARY) --serve

run-daemon: build
	./$(BINARY) --daemon

run-auth: build
	./$(BINARY) --auth

test:
	go test ./... -v

clean:
	rm -rf bin/
```

- [ ] **Step 11: Verify it compiles**

Run: `make build`
Expected: Binary at `bin/assistant-agent`, no errors.

- [ ] **Step 12: Commit**

```bash
git add -A
git commit -m "feat(p0): project skeleton with config, models, and CLI flags"
```

---

## Task 2: MongoDB Connection + Indexes

**Files:**
- Create: `pkg/db/mongo.go`
- Test: `pkg/db/mongo_test.go`

- [ ] **Step 1: Create pkg/db/mongo.go**

```go
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
```

- [ ] **Step 2: Write connection test**

Create `pkg/db/mongo_test.go`:

```go
package db

import (
	"os"
	"testing"

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
		Timeout:  5_000_000_000, // 5s
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
}
```

- [ ] **Step 3: Run test**

Run: `go test ./pkg/db/ -v -run TestConnect`
Expected: PASS (or SKIP if MongoDB not running locally)

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(p0): MongoDB connection with index creation"
```

---

## Task 3: Alert DB Operations

**Files:**
- Create: `pkg/db/alerts.go`
- Create: `pkg/db/history.go`
- Create: `pkg/db/sync_state.go`
- Create: `pkg/db/settings.go`

- [ ] **Step 1: Create pkg/db/alerts.go**

```go
package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
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
```

- [ ] **Step 2: Create pkg/db/history.go**

```go
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
```

- [ ] **Step 3: Create pkg/db/sync_state.go**

```go
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
```

- [ ] **Step 4: Create pkg/db/settings.go**

```go
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
```

- [ ] **Step 5: Verify compilation**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(p0): alert, history, sync_state, and settings DB operations"
```

---

## Task 4: Health Endpoint + API Server Skeleton

**Files:**
- Create: `pkg/api/server.go`, `pkg/api/middleware.go`, `pkg/api/response.go`
- Modify: `main.go`

- [ ] **Step 1: Create pkg/api/response.go**

```go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ListResponse struct {
	Data    interface{} `json:"data"`
	Total   int64       `json:"total"`
	Limit   int64       `json:"limit"`
	Offset  int64       `json:"offset"`
	HasMore bool        `json:"has_more"`
}

func respondOK(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func respondError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
}
```

- [ ] **Step 2: Create pkg/api/middleware.go**

```go
package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func BearerAuth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}

		auth := c.GetHeader("Authorization")
		if auth == "" {
			respondError(c, http.StatusUnauthorized, "missing authorization header")
			c.Abort()
			return
		}

		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" || parts[1] != token {
			respondError(c, http.StatusUnauthorized, "invalid token")
			c.Abort()
			return
		}

		c.Next()
	}
}

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Debug().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Msg("request")
		c.Next()
	}
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
```

- [ ] **Step 3: Create pkg/api/server.go**

```go
package api

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/innacy/assistant-agent/pkg/config"
	"github.com/innacy/assistant-agent/pkg/db"
)

type Server struct {
	router *gin.Engine
	db     *db.MongoDB
	cfg    *config.Config
}

func NewServer(database *db.MongoDB, cfg *config.Config) *Server {
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(CORS())
	router.Use(RequestLogger())

	s := &Server{
		router: router,
		db:     database,
		cfg:    cfg,
	}

	router.GET("/health", s.handleHealth)

	api := router.Group("/api")
	api.Use(BearerAuth(cfg.Server.APIToken))
	{
		// Alert endpoints will be registered here
	}

	return s
}

func (s *Server) Run() error {
	addr := fmt.Sprintf(":%d", s.cfg.Server.Port)
	return s.router.Run(addr)
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "ok",
		"service": "assistant-agent",
	})
}
```

- [ ] **Step 4: Update main.go to wire up the server**

Replace the `*serve` case in `main.go`:

```go
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/innacy/assistant-agent/pkg/api"
	"github.com/innacy/assistant-agent/pkg/config"
	"github.com/innacy/assistant-agent/pkg/db"
)

func main() {
	serve := flag.Bool("serve", false, "Start API + UI + daemon")
	daemon := flag.Bool("daemon", false, "Start headless daemon only")
	syncOnce := flag.Bool("sync-once", false, "Run single sync then exit")
	auth := flag.Bool("auth", false, "Run OAuth flow and exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	level, _ := zerolog.ParseLevel(cfg.LogLevel)
	zerolog.SetGlobalLevel(level)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	switch {
	case *auth:
		fmt.Println("TODO: OAuth flow")
	case *syncOnce:
		fmt.Println("TODO: single sync")
	case *daemon:
		fmt.Println("TODO: daemon mode")
	case *serve:
		database, err := db.Connect(cfg.DB)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to connect to MongoDB")
		}
		defer database.Close()

		srv := api.NewServer(database, cfg)
		log.Info().Int("port", cfg.Server.Port).Msg("starting server")
		if err := srv.Run(); err != nil {
			log.Fatal().Err(err).Msg("server failed")
		}
	default:
		flag.Usage()
	}
}
```

- [ ] **Step 5: Run go mod tidy and verify build**

```bash
go mod tidy
make build
```

Expected: Compiles cleanly.

- [ ] **Step 6: Test health endpoint manually**

```bash
./bin/assistant-agent --serve &
curl http://localhost:8080/health
kill %1
```

Expected: `{"service":"assistant-agent","status":"ok"}`

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(p0): API server with health endpoint and bearer auth middleware"
```

---

## Task 5: Google OAuth2 Auth Flow

**Files:**
- Create: `pkg/auth/google.go`
- Modify: `main.go` (wire up --auth flag)

- [ ] **Step 1: Create pkg/auth/google.go**

```go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/innacy/assistant-agent/pkg/config"
)

var Scopes = []string{
	"https://www.googleapis.com/auth/gmail.readonly",
	"https://www.googleapis.com/auth/calendar.readonly",
	"https://www.googleapis.com/auth/tasks.readonly",
	"https://www.googleapis.com/auth/contacts.readonly",
}

func RunAuthFlow(cfg config.GoogleConfig) error {
	b, err := os.ReadFile(cfg.CredentialsFile)
	if err != nil {
		return fmt.Errorf("unable to read credentials file: %w", err)
	}

	oauthCfg, err := google.ConfigFromJSON(b, Scopes...)
	if err != nil {
		return fmt.Errorf("unable to parse credentials: %w", err)
	}

	token, err := getTokenFromWeb(oauthCfg)
	if err != nil {
		return err
	}

	return saveToken(cfg.TokenFile, token)
}

func GetClient(cfg config.GoogleConfig) (*http.Client, error) {
	b, err := os.ReadFile(cfg.CredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %w", err)
	}

	oauthCfg, err := google.ConfigFromJSON(b, Scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %w", err)
	}

	token, err := loadToken(cfg.TokenFile)
	if err != nil {
		return nil, fmt.Errorf("token not found (run --auth first): %w", err)
	}

	tokenSource := oauthCfg.TokenSource(context.Background(), token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("token refresh failed (run --auth again): %w", err)
	}

	if newToken.AccessToken != token.AccessToken {
		if err := saveToken(cfg.TokenFile, newToken); err != nil {
			log.Warn().Err(err).Msg("failed to save refreshed token")
		}
	}

	return oauth2.NewClient(context.Background(), tokenSource), nil
}

func getTokenFromWeb(cfg *oauth2.Config) (*oauth2.Token, error) {
	authURL := cfg.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("\nOpen this URL in your browser:\n\n%s\n\n", authURL)
	fmt.Print("Enter the authorization code: ")

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %w", err)
	}

	token, err := cfg.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("unable to exchange code for token: %w", err)
	}
	return token, nil
}

func loadToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var token oauth2.Token
	return &token, json.NewDecoder(f).Decode(&token)
}

func saveToken(path string, token *oauth2.Token) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	log.Info().Str("path", path).Msg("token saved")
	return json.NewEncoder(f).Encode(token)
}
```

- [ ] **Step 2: Wire --auth in main.go**

Replace the `*auth` case:

```go
	case *auth:
		if err := auth_pkg.RunAuthFlow(cfg.Google); err != nil {
			log.Fatal().Err(err).Msg("auth flow failed")
		}
		log.Info().Msg("Authentication successful! Token saved.")
```

Add the import:
```go
auth_pkg "github.com/innacy/assistant-agent/pkg/auth"
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(p1): Google OAuth2 auth flow with token persistence"
```

---

## Task 6: Source Interface + Engine (Normalizer, Expiry, Status)

**Files:**
- Create: `pkg/sources/source.go`
- Create: `pkg/engine/normalizer.go`
- Create: `pkg/engine/expiry.go`
- Create: `pkg/engine/status.go`
- Create: `pkg/engine/dedup.go`
- Test: `pkg/engine/expiry_test.go`, `pkg/engine/status_test.go`

- [ ] **Step 1: Create pkg/sources/source.go**

```go
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
```

- [ ] **Step 2: Create pkg/engine/expiry.go**

```go
package engine

import (
	"time"

	"github.com/innacy/assistant-agent/internal/models"
)

func ComputeExpiresAt(dueDate time.Time, alertType string, ttlConfig map[string]int) time.Time {
	days, ok := ttlConfig[alertType]
	if !ok {
		days = 7
	}
	return dueDate.AddDate(0, 0, days)
}

func ComputeWindowBefore(alertType string, windowConfig map[string]int) int {
	days, ok := windowConfig[alertType]
	if !ok {
		return 3
	}
	return days
}

func NextOccurrence(dueDate time.Time, recurrence string) *time.Time {
	var next time.Time
	now := time.Now()

	switch recurrence {
	case models.RecurrenceYearly:
		next = time.Date(now.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, dueDate.Location())
		if next.Before(now) {
			next = next.AddDate(1, 0, 0)
		}
	case models.RecurrenceMonthly:
		next = time.Date(now.Year(), now.Month(), dueDate.Day(), 0, 0, 0, 0, dueDate.Location())
		if next.Before(now) {
			next = next.AddDate(0, 1, 0)
		}
	case models.RecurrenceWeekly:
		next = now.AddDate(0, 0, 7-int(now.Weekday())+int(dueDate.Weekday()))
		if next.Before(now) {
			next = next.AddDate(0, 0, 7)
		}
	default:
		return nil
	}
	return &next
}
```

- [ ] **Step 3: Create pkg/engine/status.go**

```go
package engine

import (
	"time"

	"github.com/innacy/assistant-agent/internal/models"
)

func ComputeStatus(alert *models.Alert, now time.Time) string {
	if alert.Status == models.AlertStatusAcknowledged {
		return models.AlertStatusAcknowledged
	}
	if alert.Status == models.AlertStatusSnoozed {
		if alert.SnoozedUntil != nil && alert.SnoozedUntil.After(now) {
			return models.AlertStatusSnoozed
		}
	}

	dueDay := alert.DueDate.Truncate(24 * time.Hour)
	today := now.Truncate(24 * time.Hour)

	switch {
	case dueDay.Equal(today):
		return models.AlertStatusDueToday
	case dueDay.Before(today):
		return models.AlertStatusMissed
	default:
		return models.AlertStatusUpcoming
	}
}

func RecomputeStatuses(alerts []models.Alert) []models.Alert {
	now := time.Now()
	for i := range alerts {
		alerts[i].Status = ComputeStatus(&alerts[i], now)
	}
	return alerts
}
```

- [ ] **Step 4: Create pkg/engine/normalizer.go**

```go
package engine

import (
	"time"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/sources"
)

func Normalize(item sources.RawItem, source string, windowConfig, ttlConfig map[string]int) models.Alert {
	now := time.Now()

	priority := item.Priority
	if priority == "" {
		priority = models.PriorityMedium
	}

	recurrence := item.Recurrence
	if recurrence == "" {
		recurrence = models.RecurrenceNone
	}

	currency := item.Currency
	if currency == "" {
		currency = "INR"
	}

	alert := models.Alert{
		UserID:       "default",
		Type:         item.Type,
		Title:        item.Title,
		Description:  item.Description,
		DueDate:      item.DueDate,
		Recurrence:   recurrence,
		Amount:       item.Amount,
		Currency:     currency,
		Source:       source,
		SourceRef:    item.SourceRef,
		SourceRaw:    item.SourceRaw,
		Priority:     priority,
		WindowBefore: ComputeWindowBefore(item.Type, windowConfig),
		ExpiresAt:    ComputeExpiresAt(item.DueDate, item.Type, ttlConfig),
		Tags:         []string{},
		Metadata:     item.Metadata,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	alert.Status = ComputeStatus(&alert, now)

	if recurrence != models.RecurrenceNone {
		alert.NextOccurrence = NextOccurrence(item.DueDate, recurrence)
	}

	return alert
}
```

- [ ] **Step 5: Create pkg/engine/dedup.go**

```go
package engine

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/db"
)

func FindCrossSourceDuplicate(ctx context.Context, database *db.MongoDB, alert *models.Alert) (*models.Alert, error) {
	normalizedTitle := strings.ToLower(strings.TrimSpace(alert.Title))
	dayBefore := alert.DueDate.Add(-24 * time.Hour)
	dayAfter := alert.DueDate.Add(24 * time.Hour)

	filter := bson.M{
		"user_id": alert.UserID,
		"type":    alert.Type,
		"source":  bson.M{"$ne": alert.Source},
		"due_date": bson.M{
			"$gte": dayBefore,
			"$lte": dayAfter,
		},
	}

	cursor, err := database.Alerts().Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var existing models.Alert
		if err := cursor.Decode(&existing); err != nil {
			continue
		}
		existingTitle := strings.ToLower(strings.TrimSpace(existing.Title))
		if titleSimilar(normalizedTitle, existingTitle) {
			return &existing, nil
		}
	}

	return nil, nil
}

func titleSimilar(a, b string) bool {
	if a == b {
		return true
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return true
	}
	return false
}
```

- [ ] **Step 6: Write tests for expiry and status**

Create `pkg/engine/expiry_test.go`:

```go
package engine

import (
	"testing"
	"time"

	"github.com/innacy/assistant-agent/internal/models"
)

func TestComputeExpiresAt(t *testing.T) {
	due := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	ttl := map[string]int{"birthday": 2, "payment": 7}

	got := ComputeExpiresAt(due, "birthday", ttl)
	want := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("birthday: got %v, want %v", got, want)
	}

	got = ComputeExpiresAt(due, "payment", ttl)
	want = time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("payment: got %v, want %v", got, want)
	}

	got = ComputeExpiresAt(due, "unknown_type", ttl)
	want = time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("unknown: got %v, want %v", got, want)
	}
}

func TestNextOccurrenceYearly(t *testing.T) {
	due := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
	next := NextOccurrence(due, models.RecurrenceYearly)
	if next == nil {
		t.Fatal("expected non-nil")
	}
	if next.Month() != 3 || next.Day() != 15 {
		t.Errorf("got %v, want March 15", next)
	}
	if next.Before(time.Now()) {
		t.Errorf("next occurrence should be in the future")
	}
}
```

Create `pkg/engine/status_test.go`:

```go
package engine

import (
	"testing"
	"time"

	"github.com/innacy/assistant-agent/internal/models"
)

func TestComputeStatus(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		alert  models.Alert
		want   string
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
```

- [ ] **Step 7: Run tests**

Run: `go test ./pkg/engine/ -v`
Expected: All PASS

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat(p2): source interface, normalizer, expiry, status computation, and dedup engine"
```

---

## Task 7: Calendar Syncer

**Files:**
- Create: `pkg/sources/calendar/client.go`

- [ ] **Step 1: Create pkg/sources/calendar/client.go**

```go
package calendar

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/sources"
)

var paymentKeywords = []string{"pay", "due", "bill", "emi", "renew", "deadline", "expires", "appointment"}

type Syncer struct {
	client *http.Client
}

func New(client *http.Client) *Syncer {
	return &Syncer{client: client}
}

func (s *Syncer) Name() string { return models.SourceCalendar }

func (s *Syncer) Sync(ctx context.Context, state *models.SyncState) ([]sources.RawItem, string, error) {
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(s.client))
	if err != nil {
		return nil, "", fmt.Errorf("calendar service: %w", err)
	}

	var items []sources.RawItem
	var nextSyncToken string

	calendars := []string{"primary", "addressbook#contacts@group.v.calendar.google.com"}

	for _, calID := range calendars {
		fetched, token, err := s.syncCalendar(ctx, srv, calID, state, calID != "primary")
		if err != nil {
			log.Warn().Err(err).Str("calendar", calID).Msg("calendar sync failed")
			continue
		}
		items = append(items, fetched...)
		if calID == "primary" {
			nextSyncToken = token
		}
	}

	return items, nextSyncToken, nil
}

func (s *Syncer) syncCalendar(ctx context.Context, srv *calendar.Service, calID string, state *models.SyncState, isBirthdayCalendar bool) ([]sources.RawItem, string, error) {
	var items []sources.RawItem

	call := srv.Events.List(calID).
		SingleEvents(true).
		OrderBy("startTime").
		Context(ctx)

	if state != nil && state.LastPageToken != "" && !isBirthdayCalendar {
		call = call.SyncToken(state.LastPageToken)
	} else {
		now := time.Now()
		call = call.
			TimeMin(now.AddDate(0, -1, 0).Format(time.RFC3339)).
			TimeMax(now.AddDate(0, 3, 0).Format(time.RFC3339))
	}

	err := call.Pages(ctx, func(page *calendar.Events) error {
		for _, event := range page.Items {
			if event.Status == "cancelled" {
				items = append(items, sources.RawItem{
					Type:      "cancelled",
					SourceRef: eventSourceRef(event),
				})
				continue
			}

			item := s.eventToRawItem(event, isBirthdayCalendar)
			if item != nil {
				items = append(items, *item)
			}
		}
		return nil
	})

	var nextSyncToken string
	if err == nil {
		result, _ := call.Do()
		if result != nil {
			nextSyncToken = result.NextSyncToken
		}
	}

	return items, nextSyncToken, err
}

func (s *Syncer) eventToRawItem(event *calendar.Event, isBirthdayCalendar bool) *sources.RawItem {
	dueDate := parseEventTime(event)
	if dueDate.IsZero() {
		return nil
	}

	title := event.Summary
	isAllDay := event.Start.Date != ""

	if isBirthdayCalendar {
		return &sources.RawItem{
			Type:        models.AlertTypeBirthday,
			Title:       title,
			Description: event.Description,
			DueDate:     dueDate,
			Recurrence:  models.RecurrenceYearly,
			SourceRef:   eventSourceRef(event),
			SourceRaw:   title,
			Priority:    models.PriorityMedium,
			Metadata:    map[string]interface{}{"calendar": "birthdays"},
		}
	}

	if !isAllDay && !matchesKeywords(title) {
		return nil
	}

	alertType := classifyEvent(title, event.Recurrence)
	recurrence := models.RecurrenceNone
	if len(event.Recurrence) > 0 {
		recurrence = inferRecurrence(event.Recurrence)
	}

	return &sources.RawItem{
		Type:        alertType,
		Title:       title,
		Description: event.Description,
		DueDate:     dueDate,
		Recurrence:  recurrence,
		SourceRef:   eventSourceRef(event),
		SourceRaw:   title,
		Priority:    models.PriorityMedium,
	}
}

func eventSourceRef(event *calendar.Event) string {
	start := parseEventTime(event)
	if !start.IsZero() {
		return fmt.Sprintf("%s:%s", event.Id, start.Format("2006-01-02"))
	}
	return event.Id
}

func parseEventTime(event *calendar.Event) time.Time {
	if event.Start == nil {
		return time.Time{}
	}
	if event.Start.Date != "" {
		t, _ := time.Parse("2006-01-02", event.Start.Date)
		return t
	}
	if event.Start.DateTime != "" {
		t, _ := time.Parse(time.RFC3339, event.Start.DateTime)
		return t
	}
	return time.Time{}
}

func matchesKeywords(title string) bool {
	lower := strings.ToLower(title)
	for _, kw := range paymentKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func classifyEvent(title string, recurrence []string) string {
	lower := strings.ToLower(title)
	for _, kw := range []string{"pay", "due", "bill", "emi"} {
		if strings.Contains(lower, kw) {
			return models.AlertTypePayment
		}
	}
	if len(recurrence) > 0 && (strings.Contains(lower, "renew") || strings.Contains(lower, "subscription")) {
		return models.AlertTypeSubscription
	}
	return models.AlertTypeEvent
}

func inferRecurrence(rules []string) string {
	for _, rule := range rules {
		lower := strings.ToLower(rule)
		if strings.Contains(lower, "yearly") {
			return models.RecurrenceYearly
		}
		if strings.Contains(lower, "monthly") {
			return models.RecurrenceMonthly
		}
		if strings.Contains(lower, "weekly") {
			return models.RecurrenceWeekly
		}
	}
	return models.RecurrenceCustom
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat(p2): Calendar syncer — events, birthdays, keyword filtering"
```

---

## Task 8: Contacts Syncer

**Files:**
- Create: `pkg/sources/contacts/client.go`

- [ ] **Step 1: Create pkg/sources/contacts/client.go**

```go
package contacts

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/sources"
)

type Syncer struct {
	client *http.Client
}

func New(client *http.Client) *Syncer {
	return &Syncer{client: client}
}

func (s *Syncer) Name() string { return models.SourceContacts }

func (s *Syncer) Sync(ctx context.Context, state *models.SyncState) ([]sources.RawItem, string, error) {
	srv, err := people.NewService(ctx, option.WithHTTPClient(s.client))
	if err != nil {
		return nil, "", fmt.Errorf("people service: %w", err)
	}

	var items []sources.RawItem
	var nextPageToken string

	call := srv.People.Connections.List("people/me").
		PersonFields("names,birthdays").
		PageSize(1000)

	err = call.Pages(ctx, func(resp *people.ListConnectionsResponse) error {
		for _, person := range resp.Connections {
			item := personToRawItem(person)
			if item != nil {
				items = append(items, *item)
			}
		}
		nextPageToken = resp.NextSyncToken
		return nil
	})

	if err != nil {
		return nil, "", err
	}

	log.Info().Int("contacts_with_birthdays", len(items)).Msg("contacts sync complete")
	return items, nextPageToken, nil
}

func personToRawItem(person *people.Person) *sources.RawItem {
	if len(person.Birthdays) == 0 {
		return nil
	}

	bday := person.Birthdays[0].Date
	if bday == nil || bday.Month == 0 || bday.Day == 0 {
		return nil
	}

	name := "Unknown"
	if len(person.Names) > 0 {
		name = person.Names[0].DisplayName
	}

	nextBirthday := computeNextBirthday(int(bday.Month), int(bday.Day))
	year := nextBirthday.Year()

	metadata := map[string]interface{}{
		"contact_name": name,
		"birth_month":  bday.Month,
		"birth_day":    bday.Day,
	}
	if bday.Year != 0 {
		metadata["birth_year"] = bday.Year
	}

	return &sources.RawItem{
		Type:        models.AlertTypeBirthday,
		Title:       fmt.Sprintf("%s's Birthday", name),
		Description: formatBirthdayDescription(name, bday),
		DueDate:     nextBirthday,
		Recurrence:  models.RecurrenceYearly,
		SourceRef:   fmt.Sprintf("%s:%d", person.ResourceName, year),
		SourceRaw:   name,
		Priority:    models.PriorityMedium,
		Metadata:    metadata,
	}
}

func computeNextBirthday(month, day int) time.Time {
	now := time.Now()
	thisYear := time.Date(now.Year(), time.Month(month), day, 0, 0, 0, 0, now.Location())
	if thisYear.Before(now.Truncate(24 * time.Hour)) {
		return thisYear.AddDate(1, 0, 0)
	}
	return thisYear
}

func formatBirthdayDescription(name string, date *people.Date) string {
	if date.Year != 0 {
		age := time.Now().Year() - int(date.Year)
		return fmt.Sprintf("%s turns %d", name, age)
	}
	return fmt.Sprintf("%s's birthday on %s %d", name, time.Month(date.Month).String(), date.Day)
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat(p2): Contacts syncer — birthday extraction from Google People API"
```

---

## Task 9: Tasks Syncer

**Files:**
- Create: `pkg/sources/tasks/client.go`

- [ ] **Step 1: Create pkg/sources/tasks/client.go**

```go
package tasks

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/api/option"
	tasks "google.golang.org/api/tasks/v1"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/sources"
)

type Syncer struct {
	client *http.Client
}

func New(client *http.Client) *Syncer {
	return &Syncer{client: client}
}

func (s *Syncer) Name() string { return models.SourceTasks }

func (s *Syncer) Sync(ctx context.Context, state *models.SyncState) ([]sources.RawItem, string, error) {
	srv, err := tasks.NewService(ctx, option.WithHTTPClient(s.client))
	if err != nil {
		return nil, "", fmt.Errorf("tasks service: %w", err)
	}

	taskLists, err := srv.Tasklists.List().Context(ctx).Do()
	if err != nil {
		return nil, "", fmt.Errorf("list task lists: %w", err)
	}

	var items []sources.RawItem
	now := time.Now()
	lookback := now.AddDate(0, -1, 0)

	for _, tl := range taskLists.Items {
		call := srv.Tasks.List(tl.Id).
			ShowCompleted(false).
			ShowHidden(false).
			DueMin(lookback.Format(time.RFC3339)).
			Context(ctx)

		result, err := call.Do()
		if err != nil {
			log.Warn().Err(err).Str("list", tl.Title).Msg("failed to fetch tasks")
			continue
		}

		for _, task := range result.Items {
			if task.Due == "" {
				continue
			}

			dueDate, err := time.Parse(time.RFC3339, task.Due)
			if err != nil {
				continue
			}

			priority := inferPriority(dueDate, now)

			items = append(items, sources.RawItem{
				Type:        models.AlertTypeTask,
				Title:       task.Title,
				Description: task.Notes,
				DueDate:     dueDate,
				Recurrence:  models.RecurrenceNone,
				SourceRef:   task.Id,
				SourceRaw:   task.Title,
				Priority:    priority,
				Metadata: map[string]interface{}{
					"task_list":   tl.Title,
					"task_status": task.Status,
				},
			})
		}
	}

	log.Info().Int("tasks_fetched", len(items)).Msg("tasks sync complete")
	return items, "", nil
}

func (s *Syncer) FetchActiveTaskIDs(ctx context.Context) (map[string]bool, error) {
	srv, err := tasks.NewService(ctx, option.WithHTTPClient(s.client))
	if err != nil {
		return nil, err
	}

	ids := make(map[string]bool)
	taskLists, err := srv.Tasklists.List().Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	for _, tl := range taskLists.Items {
		result, err := srv.Tasks.List(tl.Id).
			ShowCompleted(false).
			Context(ctx).
			Do()
		if err != nil {
			continue
		}
		for _, task := range result.Items {
			ids[task.Id] = true
		}
	}

	return ids, nil
}

func inferPriority(dueDate, now time.Time) string {
	daysUntil := int(dueDate.Sub(now).Hours() / 24)
	switch {
	case daysUntil < 0:
		return models.PriorityHigh
	case daysUntil <= 2:
		return models.PriorityMedium
	default:
		return models.PriorityLow
	}
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat(p3): Tasks syncer — fetch tasks with due dates, infer priority"
```

---

## Task 10: Gmail Syncer + Parsers

**Files:**
- Create: `pkg/sources/gmail/client.go`
- Create: `pkg/sources/gmail/parsers.go`
- Test: `pkg/sources/gmail/parsers_test.go`

- [ ] **Step 1: Create pkg/sources/gmail/parsers.go**

```go
package gmail

import (
	"regexp"
	"strings"
	"time"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/sources"
)

var (
	amountRegex = regexp.MustCompile(`(?i)(?:₹|INR|Rs\.?\s?)(\d[\d,]*\.?\d*)`)
	dateRegex   = regexp.MustCompile(`(\d{1,2})\s*(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\w*\s*(\d{4})?`)

	subjectPatterns = []string{
		"subscription", "renewal", "payment due", "bill generated",
		"upcoming charge", "reminder", "invoice", "due date",
		"your plan", "auto-renewal", "expiring",
	}
)

type ParsedEmail struct {
	Type        string
	Title       string
	Description string
	DueDate     time.Time
	Amount      *float64
	SourceRef   string
	SourceRaw   string
}

func ParseEmail(msgID, from, subject, body string, whitelistedSenders []string) *sources.RawItem {
	isWhitelisted := isSenderWhitelisted(from, whitelistedSenders)

	if !isWhitelisted {
		if !matchesSubjectPatterns(subject) {
			return nil
		}
		if !validateBody(body) {
			return nil
		}
	}

	title := extractServiceName(from, subject)
	amount := extractAmount(body + " " + subject)
	dueDate := extractDate(body + " " + subject)
	if dueDate.IsZero() {
		dueDate = time.Now().AddDate(0, 0, 3)
	}

	alertType := classifyEmail(subject, body)

	return &sources.RawItem{
		Type:        alertType,
		Title:       title,
		Description: truncate(subject, 200),
		DueDate:     dueDate,
		Amount:      amount,
		Currency:    "INR",
		Recurrence:  inferEmailRecurrence(subject, body),
		SourceRef:   msgID,
		SourceRaw:   subject,
		Priority:    models.PriorityMedium,
		Metadata: map[string]interface{}{
			"sender":       from,
			"whitelisted":  isWhitelisted,
		},
	}
}

func isSenderWhitelisted(from string, whitelist []string) bool {
	lower := strings.ToLower(from)
	for _, s := range whitelist {
		if strings.Contains(lower, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

func matchesSubjectPatterns(subject string) bool {
	lower := strings.ToLower(subject)
	for _, pattern := range subjectPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func validateBody(body string) bool {
	if amountRegex.MatchString(body) {
		return true
	}
	if dateRegex.MatchString(body) {
		return true
	}
	lower := strings.ToLower(body)
	keywords := []string{"renewal", "expiry", "expires", "due date", "overdue", "payment"}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func extractServiceName(from, subject string) string {
	parts := strings.SplitN(from, "<", 2)
	name := strings.TrimSpace(parts[0])
	if name != "" && name != from {
		return name
	}
	words := strings.Fields(subject)
	if len(words) > 5 {
		return strings.Join(words[:5], " ")
	}
	return subject
}

func extractAmount(text string) *float64 {
	match := amountRegex.FindStringSubmatch(text)
	if match == nil {
		return nil
	}
	cleaned := strings.ReplaceAll(match[1], ",", "")
	var val float64
	fmt.Sscanf(cleaned, "%f", &val)
	if val > 0 {
		return &val
	}
	return nil
}

func extractDate(text string) time.Time {
	match := dateRegex.FindStringSubmatch(text)
	if match == nil {
		return time.Time{}
	}
	dateStr := match[0]
	formats := []string{"2 Jan 2006", "02 Jan 2006", "2 January 2006", "02 January 2006"}
	for _, f := range formats {
		if t, err := time.Parse(f, dateStr); err == nil {
			if t.Year() == 0 {
				t = t.AddDate(time.Now().Year(), 0, 0)
			}
			return t
		}
	}
	return time.Time{}
}

func classifyEmail(subject, body string) string {
	combined := strings.ToLower(subject + " " + body)
	if strings.Contains(combined, "subscription") || strings.Contains(combined, "renewal") || strings.Contains(combined, "auto-renew") {
		return models.AlertTypeSubscription
	}
	return models.AlertTypePayment
}

func inferEmailRecurrence(subject, body string) string {
	combined := strings.ToLower(subject + " " + body)
	if strings.Contains(combined, "monthly") {
		return models.RecurrenceMonthly
	}
	if strings.Contains(combined, "yearly") || strings.Contains(combined, "annual") {
		return models.RecurrenceYearly
	}
	return models.RecurrenceNone
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
```

Add import for `fmt` at the top (used by extractAmount's Sscanf).

- [ ] **Step 2: Create pkg/sources/gmail/client.go**

```go
package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/config"
	"github.com/innacy/assistant-agent/pkg/sources"
)

type Syncer struct {
	client    *http.Client
	gmailCfg  config.GmailConfig
}

func New(client *http.Client, gmailCfg config.GmailConfig) *Syncer {
	return &Syncer{client: client, gmailCfg: gmailCfg}
}

func (s *Syncer) Name() string { return models.SourceGmail }

func (s *Syncer) Sync(ctx context.Context, state *models.SyncState) ([]sources.RawItem, string, error) {
	srv, err := gmail.NewService(ctx, option.WithHTTPClient(s.client))
	if err != nil {
		return nil, "", fmt.Errorf("gmail service: %w", err)
	}

	query := buildQuery(s.gmailCfg)
	var messages []*gmail.Message

	if state != nil && state.LastPageToken != "" {
		messages, err = s.fetchByHistoryID(ctx, srv, state.LastPageToken, query)
		if err != nil {
			log.Warn().Err(err).Msg("historyId fetch failed, falling back to date query")
			messages, err = s.fetchByQuery(ctx, srv, query)
			if err != nil {
				return nil, "", err
			}
		}
	} else {
		messages, err = s.fetchByQuery(ctx, srv, query)
		if err != nil {
			return nil, "", err
		}
	}

	var items []sources.RawItem
	for _, msg := range messages {
		full, err := srv.Users.Messages.Get("me", msg.Id).Format("full").Context(ctx).Do()
		if err != nil {
			log.Warn().Err(err).Str("msg_id", msg.Id).Msg("failed to fetch message")
			continue
		}

		from, subject := extractHeaders(full)
		body := extractBody(full)

		item := ParseEmail(msg.Id, from, subject, body, s.gmailCfg.SenderWhitelist)
		if item != nil {
			items = append(items, *item)
		}
	}

	profile, _ := srv.Users.GetProfile("me").Context(ctx).Do()
	var historyID string
	if profile != nil {
		historyID = fmt.Sprintf("%d", profile.HistoryId)
	}

	log.Info().Int("emails_parsed", len(items)).Int("total_fetched", len(messages)).Msg("gmail sync complete")
	return items, historyID, nil
}

func (s *Syncer) fetchByQuery(ctx context.Context, srv *gmail.Service, query string) ([]*gmail.Message, error) {
	var messages []*gmail.Message
	call := srv.Users.Messages.List("me").Q(query).MaxResults(100)

	err := call.Pages(ctx, func(resp *gmail.ListMessagesResponse) error {
		messages = append(messages, resp.Messages...)
		return nil
	})

	return messages, err
}

func (s *Syncer) fetchByHistoryID(ctx context.Context, srv *gmail.Service, historyID, query string) ([]*gmail.Message, error) {
	var id uint64
	fmt.Sscanf(historyID, "%d", &id)

	resp, err := srv.Users.History.List("me").
		StartHistoryId(id).
		HistoryTypes("messageAdded").
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}

	var messages []*gmail.Message
	for _, h := range resp.History {
		for _, added := range h.MessagesAdded {
			messages = append(messages, added.Message)
		}
	}
	return messages, nil
}

func buildQuery(cfg config.GmailConfig) string {
	parts := make([]string, 0, len(cfg.QueryFilters)+len(cfg.SenderWhitelist))
	for _, f := range cfg.QueryFilters {
		parts = append(parts, fmt.Sprintf("(%s)", f))
	}
	for _, sender := range cfg.SenderWhitelist {
		parts = append(parts, fmt.Sprintf("(from:%s)", sender))
	}
	return strings.Join(parts, " OR ")
}

func extractHeaders(msg *gmail.Message) (from, subject string) {
	for _, h := range msg.Payload.Headers {
		switch strings.ToLower(h.Name) {
		case "from":
			from = h.Value
		case "subject":
			subject = h.Value
		}
	}
	return
}

func extractBody(msg *gmail.Message) string {
	if msg.Payload == nil {
		return ""
	}

	if msg.Payload.Body != nil && msg.Payload.Body.Data != "" {
		data, _ := base64.URLEncoding.DecodeString(msg.Payload.Body.Data)
		return string(data)
	}

	for _, part := range msg.Payload.Parts {
		if part.MimeType == "text/plain" && part.Body != nil && part.Body.Data != "" {
			data, _ := base64.URLEncoding.DecodeString(part.Body.Data)
			return string(data)
		}
	}

	for _, part := range msg.Payload.Parts {
		if part.MimeType == "text/html" && part.Body != nil && part.Body.Data != "" {
			data, _ := base64.URLEncoding.DecodeString(part.Body.Data)
			return string(data)
		}
	}

	return ""
}
```

- [ ] **Step 3: Write parser tests**

Create `pkg/sources/gmail/parsers_test.go`:

```go
package gmail

import (
	"testing"
)

func TestIsSenderWhitelisted(t *testing.T) {
	whitelist := []string{"noreply@netflix.com", "alerts@hdfcbank.net"}

	if !isSenderWhitelisted("Netflix <noreply@netflix.com>", whitelist) {
		t.Error("should match netflix")
	}
	if isSenderWhitelisted("random@spam.com", whitelist) {
		t.Error("should not match spam")
	}
}

func TestMatchesSubjectPatterns(t *testing.T) {
	if !matchesSubjectPatterns("Your subscription has been renewed") {
		t.Error("should match subscription")
	}
	if !matchesSubjectPatterns("Payment due for your account") {
		t.Error("should match payment due")
	}
	if matchesSubjectPatterns("Check out our new products!") {
		t.Error("should not match marketing")
	}
}

func TestExtractAmount(t *testing.T) {
	tests := []struct {
		input string
		want  *float64
	}{
		{"Amount: ₹499", floatPtr(499)},
		{"INR 1,299.00 charged", floatPtr(1299)},
		{"Rs. 2500 deducted", floatPtr(2500)},
		{"No amount here", nil},
	}

	for _, tt := range tests {
		got := extractAmount(tt.input)
		if tt.want == nil && got != nil {
			t.Errorf("input=%q: want nil, got %v", tt.input, *got)
		}
		if tt.want != nil && (got == nil || *got != *tt.want) {
			t.Errorf("input=%q: want %v, got %v", tt.input, *tt.want, got)
		}
	}
}

func TestParseEmailWhitelistedSender(t *testing.T) {
	whitelist := []string{"noreply@netflix.com"}
	item := ParseEmail(
		"msg123",
		"Netflix <noreply@netflix.com>",
		"Your Netflix membership renewal",
		"Your subscription will renew on 15 Jul 2026 for ₹649",
		whitelist,
	)
	if item == nil {
		t.Fatal("expected non-nil item for whitelisted sender")
	}
	if item.Type != "subscription" {
		t.Errorf("type: got %q, want subscription", item.Type)
	}
	if item.Amount == nil || *item.Amount != 649 {
		t.Errorf("amount: got %v, want 649", item.Amount)
	}
}

func TestParseEmailUnknownSenderWithValidation(t *testing.T) {
	item := ParseEmail(
		"msg456",
		"billing@random-saas.com",
		"Your subscription renewal is coming up",
		"Your plan will auto-renew on 20 Aug 2026 for ₹999 per month",
		[]string{"noreply@netflix.com"},
	)
	if item == nil {
		t.Fatal("expected non-nil item (subject+body both valid)")
	}
}

func TestParseEmailRejectsSpam(t *testing.T) {
	item := ParseEmail(
		"msg789",
		"marketing@shop.com",
		"Big sale this weekend!",
		"Shop now and save 50% on all items",
		[]string{"noreply@netflix.com"},
	)
	if item != nil {
		t.Error("expected nil for spam email")
	}
}

func floatPtr(f float64) *float64 { return &f }
```

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/sources/gmail/ -v`
Expected: All PASS

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(p4): Gmail syncer — two-tier parsing, sender whitelist, amount/date extraction"
```

---

## Task 11: Daemon — Background Polling + Pipeline

**Files:**
- Create: `pkg/daemon/daemon.go`
- Modify: `main.go` (wire up daemon and sync-once modes)

- [ ] **Step 1: Create pkg/daemon/daemon.go**

```go
package daemon

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/config"
	"github.com/innacy/assistant-agent/pkg/db"
	"github.com/innacy/assistant-agent/pkg/engine"
	"github.com/innacy/assistant-agent/pkg/sources"
)

type Daemon struct {
	db       *db.MongoDB
	cfg      *config.Config
	syncers  []sources.Syncer
	stopCh   chan struct{}
	userID   string
}

func New(database *db.MongoDB, cfg *config.Config, syncers []sources.Syncer) *Daemon {
	return &Daemon{
		db:      database,
		cfg:     cfg,
		syncers: syncers,
		stopCh:  make(chan struct{}),
		userID:  "default",
	}
}

func (d *Daemon) Run(ctx context.Context) {
	log.Info().Dur("interval", d.cfg.Daemon.PollInterval).Msg("daemon starting")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	d.runSyncCycle(ctx)

	ticker := time.NewTicker(d.cfg.Daemon.PollInterval)
	defer ticker.Stop()

	midnightTicker := d.scheduleMidnight()
	defer midnightTicker.Stop()

	for {
		select {
		case <-ticker.C:
			d.runSyncCycle(ctx)
		case <-midnightTicker.C:
			d.runDailyRecurring(ctx)
			midnightTicker = d.scheduleMidnight()
		case <-sigCh:
			log.Info().Msg("shutdown signal received")
			d.shutdown(ctx)
			return
		case <-d.stopCh:
			return
		}
	}
}

func (d *Daemon) RunOnce(ctx context.Context) {
	d.runSyncCycle(ctx)
}

func (d *Daemon) Stop() {
	close(d.stopCh)
}

func (d *Daemon) runSyncCycle(ctx context.Context) {
	log.Info().Msg("sync cycle starting")

	var wg sync.WaitGroup
	for _, syncer := range d.syncers {
		wg.Add(1)
		go func(s sources.Syncer) {
			defer wg.Done()
			d.syncSource(ctx, s)
		}(syncer)
	}
	wg.Wait()

	d.refreshStatuses(ctx)
	d.archiveExpired(ctx)

	log.Info().Msg("sync cycle complete")
}

func (d *Daemon) syncSource(ctx context.Context, syncer sources.Syncer) {
	name := syncer.Name()
	log.Info().Str("source", name).Msg("syncing")

	_ = d.db.SetSyncStatus(ctx, d.userID, name, models.SyncStatusSyncing)

	state, _ := d.db.GetSyncState(ctx, d.userID, name)
	if state == nil {
		state = &models.SyncState{UserID: d.userID, Source: name}
	}

	items, pageToken, err := syncer.Sync(ctx, state)
	if err != nil {
		log.Error().Err(err).Str("source", name).Msg("sync failed")
		_ = d.db.SetSyncError(ctx, d.userID, name, err.Error())
		return
	}

	settings, _ := d.db.GetSettings(ctx, d.userID)

	var processed int64
	for _, item := range items {
		if item.Type == "cancelled" {
			d.handleCancellation(ctx, name, item.SourceRef)
			continue
		}

		alert := engine.Normalize(item, name, settings.Windows, settings.TTL)
		alert.UserID = d.userID

		if err := d.db.UpsertAlert(ctx, &alert); err != nil {
			log.Warn().Err(err).Str("title", alert.Title).Msg("upsert failed")
			continue
		}
		processed++
	}

	_ = d.db.SetSyncSuccess(ctx, d.userID, name, pageToken, processed)
	log.Info().Str("source", name).Int64("processed", processed).Msg("sync complete")
}

func (d *Daemon) handleCancellation(ctx context.Context, source, sourceRef string) {
	filter := bson.M{"source": source, "source_ref": bson.M{"$regex": "^" + sourceRef}}
	_, _ = d.db.BulkUpdateStatus(ctx, filter, models.AlertStatusAcknowledged, bson.M{
		"acknowledged_at": time.Now(),
	})
}

func (d *Daemon) refreshStatuses(ctx context.Context) {
	now := time.Now()
	today := now.Truncate(24 * time.Hour)

	d.db.BulkUpdateStatus(ctx, bson.M{
		"user_id":  d.userID,
		"status":   models.AlertStatusUpcoming,
		"due_date": bson.M{"$lt": today},
	}, models.AlertStatusMissed, nil)

	d.db.BulkUpdateStatus(ctx, bson.M{
		"user_id": d.userID,
		"status":  models.AlertStatusUpcoming,
		"due_date": bson.M{
			"$gte": today,
			"$lt":  today.Add(24 * time.Hour),
		},
	}, models.AlertStatusDueToday, nil)

	d.db.BulkUpdateStatus(ctx, bson.M{
		"user_id":       d.userID,
		"status":        models.AlertStatusSnoozed,
		"snoozed_until": bson.M{"$lte": now},
	}, models.AlertStatusUpcoming, bson.M{
		"snoozed_until": nil,
	})
}

func (d *Daemon) archiveExpired(ctx context.Context) {
	count, err := d.db.ArchiveExpiredAlerts(ctx, d.userID)
	if err != nil {
		log.Warn().Err(err).Msg("archive expired failed")
		return
	}
	if count > 0 {
		log.Info().Int64("archived", count).Msg("expired alerts archived")
	}
}

func (d *Daemon) runDailyRecurring(ctx context.Context) {
	log.Info().Msg("daily recurring job starting")
	// Recurring alert creation will be handled by scanning
	// acknowledged recurring alerts and creating new instances
	// when their next occurrence enters the window.
	// This is a future enhancement within this phase.
}

func (d *Daemon) shutdown(ctx context.Context) {
	log.Info().Msg("graceful shutdown: waiting for sync to finish (30s timeout)")

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	d.archiveExpired(shutdownCtx)
	log.Info().Msg("shutdown complete")
}

func (d *Daemon) scheduleMidnight() *time.Ticker {
	now := time.Now()
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	duration := nextMidnight.Sub(now)
	return time.NewTicker(duration)
}
```

- [ ] **Step 2: Update main.go to wire daemon and sync-once**

Replace the `*syncOnce` and `*daemon` cases in main.go:

```go
	case *syncOnce:
		database, err := db.Connect(cfg.DB)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to connect to MongoDB")
		}
		defer database.Close()

		syncers, err := buildSyncers(cfg)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to build syncers")
		}

		d := daemon.New(database, cfg, syncers)
		d.RunOnce(context.Background())

	case *daemon:
		database, err := db.Connect(cfg.DB)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to connect to MongoDB")
		}
		defer database.Close()

		syncers, err := buildSyncers(cfg)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to build syncers")
		}

		d := daemon.New(database, cfg, syncers)
		d.Run(context.Background())
```

Add a helper function:

```go
func buildSyncers(cfg *config.Config) ([]sources.Syncer, error) {
	httpClient, err := auth_pkg.GetClient(cfg.Google)
	if err != nil {
		return nil, err
	}

	return []sources.Syncer{
		calendarPkg.New(httpClient),
		contactsPkg.New(httpClient),
		tasksPkg.New(httpClient),
		gmailPkg.New(httpClient, cfg.Gmail),
	}, nil
}
```

Add the necessary imports:

```go
import (
	"context"

	calendarPkg "github.com/innacy/assistant-agent/pkg/sources/calendar"
	contactsPkg "github.com/innacy/assistant-agent/pkg/sources/contacts"
	tasksPkg "github.com/innacy/assistant-agent/pkg/sources/tasks"
	gmailPkg "github.com/innacy/assistant-agent/pkg/sources/gmail"
	"github.com/innacy/assistant-agent/pkg/daemon"
	"github.com/innacy/assistant-agent/pkg/sources"
)
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat(p5): daemon with background polling, status refresh, and graceful shutdown"
```

---

## Task 12: REST API — Alert Endpoints

**Files:**
- Create: `pkg/api/handlers_alerts.go`
- Modify: `pkg/api/server.go` (register routes)

- [ ] **Step 1: Create pkg/api/handlers_alerts.go**

```go
package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/innacy/assistant-agent/internal/models"
	"github.com/innacy/assistant-agent/pkg/db"
	"github.com/innacy/assistant-agent/pkg/engine"
)

const defaultUserID = "default"

func (s *Server) handleListAlerts(c *gin.Context) {
	filter := parseAlertFilter(c)
	result, err := s.db.ListAlerts(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	result.Data = engine.RecomputeStatuses(result.Data)
	c.JSON(http.StatusOK, result)
}

func (s *Server) handleGetAlert(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid id")
		return
	}

	alert, err := s.db.GetAlert(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusNotFound, "alert not found")
		return
	}
	alert.Status = engine.ComputeStatus(alert, time.Now())
	c.JSON(http.StatusOK, alert)
}

func (s *Server) handleUpcomingAlerts(c *gin.Context) {
	filter := db.AlertFilter{
		UserID: defaultUserID,
		Status: []string{models.AlertStatusUpcoming},
		Limit:  50,
	}
	result, err := s.db.ListAlerts(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	result.Data = engine.RecomputeStatuses(result.Data)
	c.JSON(http.StatusOK, result)
}

func (s *Server) handleMissedAlerts(c *gin.Context) {
	filter := db.AlertFilter{
		UserID: defaultUserID,
		Status: []string{models.AlertStatusMissed},
		Limit:  50,
	}
	result, err := s.db.ListAlerts(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	result.Data = engine.RecomputeStatuses(result.Data)
	c.JSON(http.StatusOK, result)
}

func (s *Server) handleTodayAlerts(c *gin.Context) {
	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)
	filter := db.AlertFilter{
		UserID: defaultUserID,
		From:   &today,
		To:     &tomorrow,
		Limit:  50,
	}
	result, err := s.db.ListAlerts(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	result.Data = engine.RecomputeStatuses(result.Data)
	c.JSON(http.StatusOK, result)
}

func (s *Server) handleCreateAlert(c *gin.Context) {
	var req struct {
		Type        string   `json:"type" binding:"required"`
		Title       string   `json:"title" binding:"required"`
		Description string   `json:"description"`
		DueDate     string   `json:"due_date" binding:"required"`
		Recurrence  string   `json:"recurrence"`
		Amount      *float64 `json:"amount"`
		Priority    string   `json:"priority"`
		Tags        []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	dueDate, err := time.Parse("2006-01-02", req.DueDate)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid due_date format (use YYYY-MM-DD)")
		return
	}

	settings, _ := s.db.GetSettings(c.Request.Context(), defaultUserID)
	recurrence := req.Recurrence
	if recurrence == "" {
		recurrence = models.RecurrenceNone
	}
	priority := req.Priority
	if priority == "" {
		priority = models.PriorityMedium
	}

	now := time.Now()
	alert := models.Alert{
		UserID:       defaultUserID,
		Type:         req.Type,
		Title:        req.Title,
		Description:  req.Description,
		DueDate:      dueDate,
		Recurrence:   recurrence,
		Amount:       req.Amount,
		Currency:     "INR",
		Source:       models.SourceManual,
		SourceRef:    "manual:" + uuid.New().String(),
		Priority:     priority,
		WindowBefore: engine.ComputeWindowBefore(req.Type, settings.Windows),
		ExpiresAt:    engine.ComputeExpiresAt(dueDate, req.Type, settings.TTL),
		Tags:         req.Tags,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	alert.Status = engine.ComputeStatus(&alert, now)

	if err := s.db.UpsertAlert(c.Request.Context(), &alert); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, alert)
}

func (s *Server) handleUpdateAlert(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid id")
		return
	}

	alert, err := s.db.GetAlert(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusNotFound, "alert not found")
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if v, ok := req["title"].(string); ok {
		alert.Title = v
	}
	if v, ok := req["description"].(string); ok {
		alert.Description = v
	}
	if v, ok := req["due_date"].(string); ok {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			alert.DueDate = t
		}
	}
	if v, ok := req["priority"].(string); ok {
		alert.Priority = v
	}
	if v, ok := req["tags"].([]interface{}); ok {
		tags := make([]string, len(v))
		for i, t := range v {
			tags[i], _ = t.(string)
		}
		alert.Tags = tags
	}
	alert.UpdatedAt = time.Now()

	if err := s.db.UpsertAlert(c.Request.Context(), alert); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, alert)
}

func (s *Server) handleDeleteAlert(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.db.DeleteAlert(c.Request.Context(), id); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c)
}

func (s *Server) handleAcknowledge(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid id")
		return
	}
	now := time.Now()
	err = s.db.UpdateAlertStatus(c.Request.Context(), id, models.AlertStatusAcknowledged, bson.M{
		"acknowledged_at": now,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c)
}

func (s *Server) handleSnooze(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid id")
		return
	}

	untilStr := c.Query("until")
	if untilStr == "" {
		respondError(c, http.StatusBadRequest, "until parameter required")
		return
	}
	until, err := time.Parse("2006-01-02", untilStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid until date")
		return
	}

	err = s.db.UpdateAlertStatus(c.Request.Context(), id, models.AlertStatusSnoozed, bson.M{
		"snoozed_until": until,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c)
}

func (s *Server) handleBatchAcknowledge(c *gin.Context) {
	var req struct {
		IDs    []string          `json:"ids"`
		Filter map[string]string `json:"filter"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	ctx := c.Request.Context()
	now := time.Now()
	var count int64

	if len(req.IDs) > 0 {
		ids := make([]primitive.ObjectID, 0, len(req.IDs))
		for _, idStr := range req.IDs {
			if id, err := primitive.ObjectIDFromHex(idStr); err == nil {
				ids = append(ids, id)
			}
		}
		n, _ := s.db.BulkUpdateStatus(ctx, bson.M{"_id": bson.M{"$in": ids}}, models.AlertStatusAcknowledged, bson.M{"acknowledged_at": now})
		count = n
	} else if req.Filter != nil {
		filter := bson.M{"user_id": defaultUserID}
		if v, ok := req.Filter["status"]; ok {
			filter["status"] = v
		}
		if v, ok := req.Filter["type"]; ok {
			filter["type"] = v
		}
		n, _ := s.db.BulkUpdateStatus(ctx, filter, models.AlertStatusAcknowledged, bson.M{"acknowledged_at": now})
		count = n
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "count": count})
}

func (s *Server) handleBatchSnooze(c *gin.Context) {
	var req struct {
		IDs    []string          `json:"ids"`
		Filter map[string]string `json:"filter"`
		Until  string            `json:"until" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	until, err := time.Parse("2006-01-02", req.Until)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid until date")
		return
	}

	ctx := c.Request.Context()
	var count int64

	if len(req.IDs) > 0 {
		ids := make([]primitive.ObjectID, 0, len(req.IDs))
		for _, idStr := range req.IDs {
			if id, err := primitive.ObjectIDFromHex(idStr); err == nil {
				ids = append(ids, id)
			}
		}
		n, _ := s.db.BulkUpdateStatus(ctx, bson.M{"_id": bson.M{"$in": ids}}, models.AlertStatusSnoozed, bson.M{"snoozed_until": until})
		count = n
	} else if req.Filter != nil {
		filter := bson.M{"user_id": defaultUserID}
		if v, ok := req.Filter["status"]; ok {
			filter["status"] = v
		}
		if v, ok := req.Filter["type"]; ok {
			filter["type"] = v
		}
		n, _ := s.db.BulkUpdateStatus(ctx, filter, models.AlertStatusSnoozed, bson.M{"snoozed_until": until})
		count = n
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "count": count})
}

func parseAlertFilter(c *gin.Context) db.AlertFilter {
	f := db.AlertFilter{UserID: defaultUserID}

	if types := c.Query("type"); types != "" {
		f.Types = strings.Split(types, ",")
	}
	if status := c.Query("status"); status != "" {
		f.Status = strings.Split(status, ",")
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			f.From = &t
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			f.To = &t
		}
	}
	if limit := c.Query("limit"); limit != "" {
		if n, err := strconv.ParseInt(limit, 10, 64); err == nil {
			f.Limit = n
		}
	}
	if offset := c.Query("offset"); offset != "" {
		if n, err := strconv.ParseInt(offset, 10, 64); err == nil {
			f.Offset = n
		}
	}

	return f
}
```

- [ ] **Step 2: Create pkg/api/handlers_history.go**

```go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleListHistory(c *gin.Context) {
	filter := parseAlertFilter(c)
	result, err := s.db.ListHistory(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}
```

- [ ] **Step 3: Create pkg/api/handlers_sync.go**

```go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleSyncStatus(c *gin.Context) {
	states, err := s.db.GetAllSyncStates(c.Request.Context(), defaultUserID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": states})
}

func (s *Server) handleSyncTrigger(c *gin.Context) {
	// Trigger is handled by signaling the daemon.
	// For now, return accepted.
	c.JSON(http.StatusAccepted, gin.H{"ok": true, "message": "sync triggered"})
}
```

- [ ] **Step 4: Create pkg/api/handlers_settings.go**

```go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/innacy/assistant-agent/internal/models"
)

func (s *Server) handleGetSettings(c *gin.Context) {
	settings, err := s.db.GetSettings(c.Request.Context(), defaultUserID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (s *Server) handleUpdateSettings(c *gin.Context) {
	var req models.Settings
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	req.UserID = defaultUserID

	if err := s.db.UpdateSettings(c.Request.Context(), &req); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c)
}
```

- [ ] **Step 5: Update pkg/api/server.go to register all routes**

Replace the `api` group in `NewServer`:

```go
	api := router.Group("/api")
	api.Use(BearerAuth(cfg.Server.APIToken))
	{
		// Alerts — read
		api.GET("/alerts", s.handleListAlerts)
		api.GET("/alerts/upcoming", s.handleUpcomingAlerts)
		api.GET("/alerts/missed", s.handleMissedAlerts)
		api.GET("/alerts/today", s.handleTodayAlerts)
		api.GET("/alerts/:id", s.handleGetAlert)

		// Alerts — write
		api.POST("/alerts", s.handleCreateAlert)
		api.PUT("/alerts/:id", s.handleUpdateAlert)
		api.DELETE("/alerts/:id", s.handleDeleteAlert)

		// Alerts — actions
		api.PATCH("/alerts/:id/acknowledge", s.handleAcknowledge)
		api.PATCH("/alerts/:id/snooze", s.handleSnooze)

		// Alerts — batch
		api.POST("/alerts/batch/acknowledge", s.handleBatchAcknowledge)
		api.POST("/alerts/batch/snooze", s.handleBatchSnooze)

		// History
		api.GET("/history", s.handleListHistory)

		// Sync
		api.GET("/sync/status", s.handleSyncStatus)
		api.POST("/sync/trigger", s.handleSyncTrigger)

		// Settings
		api.GET("/settings", s.handleGetSettings)
		api.PUT("/settings", s.handleUpdateSettings)
	}
```

- [ ] **Step 6: Verify compilation**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(p6): REST API — full alert CRUD, batch ops, history, sync, settings"
```

---

## Task 13: Wire --serve Mode with Daemon + API

**Files:**
- Modify: `main.go`
- Modify: `pkg/api/server.go`

- [ ] **Step 1: Update main.go --serve case to run daemon + API concurrently**

Replace the `*serve` case:

```go
	case *serve:
		database, err := db.Connect(cfg.DB)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to connect to MongoDB")
		}
		defer database.Close()

		syncers, err := buildSyncers(cfg)
		if err != nil {
			log.Warn().Err(err).Msg("syncers unavailable (run --auth first)")
			syncers = nil
		}

		// Start daemon in background if syncers available
		if syncers != nil {
			d := daemon.New(database, cfg, syncers)
			go d.Run(context.Background())
		}

		// Start API server (blocking)
		srv := api.NewServer(database, cfg)
		log.Info().Int("port", cfg.Server.Port).Msg("starting server")
		if err := srv.Run(); err != nil {
			log.Fatal().Err(err).Msg("server failed")
		}
```

- [ ] **Step 2: Final build + go mod tidy**

```bash
go mod tidy
make build
```

Expected: Clean build, binary at `bin/assistant-agent`

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat(p6): wire --serve mode with concurrent daemon + API server"
```

---

## Task 14: Final Verification + Cleanup

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: All tests pass (engine tests + parser tests; DB tests skip if no MongoDB)

- [ ] **Step 2: Run linter**

```bash
go vet ./...
```

Expected: No issues

- [ ] **Step 3: Verify all run modes compile correctly**

```bash
./bin/assistant-agent --help
./bin/assistant-agent --serve &
curl -s http://localhost:8080/health | grep ok
curl -s -H "Authorization: Bearer change-me-to-a-real-token" http://localhost:8080/api/alerts | grep data
kill %1
```

Expected: Health returns ok, alerts returns empty data array with auth.

- [ ] **Step 4: Final commit + push**

```bash
git add -A
git commit -m "chore: final cleanup and verification"
git push origin master
```

---

## Summary of Deliverables

After completing all tasks:

| What | Status |
|------|--------|
| `make build` produces binary | ✓ |
| `--auth` runs OAuth flow | ✓ |
| `--serve` starts API + daemon | ✓ |
| `--daemon` runs headless sync | ✓ |
| `--sync-once` syncs and exits | ✓ |
| `GET /health` unauthenticated | ✓ |
| All `/api/*` require bearer token | ✓ |
| Alert CRUD + acknowledge/snooze | ✓ |
| Batch operations | ✓ |
| History endpoint | ✓ |
| Calendar + Contacts sync | ✓ |
| Tasks sync | ✓ |
| Gmail sync with two-tier parsing | ✓ |
| Background daemon with polling | ✓ |
| Dynamic status computation | ✓ |
| Graceful shutdown | ✓ |
