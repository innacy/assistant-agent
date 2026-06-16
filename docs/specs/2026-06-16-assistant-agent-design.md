# Assistant Agent — Design Specification

**Date**: 2026-06-16
**Status**: Approved
**Stack**: Go 1.22+, MongoDB, Google APIs (Gmail, Calendar, Tasks, Contacts)

## Overview

A personal assistant agent that syncs from Gmail, Google Calendar, Google Tasks, and Google Contacts to surface date-sensitive items — subscriptions, birthdays, payments, deadlines — and anything recently missed. Items are stored in MongoDB with TTL-based validity and served via a REST API to an embedded Vite/React/TypeScript UI.

### Goals

- Unified timeline of upcoming dates across all Google sources
- Auto-detect subscriptions and payment reminders from Gmail
- Surface birthdays from Contacts + Calendar (deduplicated)
- Track tasks with due dates from Google Tasks
- Configurable lookahead windows per alert type
- TTL-based alert expiry (alerts disappear when no longer relevant)
- Background daemon for continuous syncing
- Embedded web UI for viewing and managing alerts

### Non-Goals (v1)

- AI/LLM parsing (deferred — rules-only for v1)
- Telegram or push notifications
- Multi-user support (single user; `user_id` field exists for future-proofing)
- Mobile native app (embedded web UI only)
- Financial transaction tracking (handled by finance-agent)

---

## Architecture

### Layered Design

```
Google APIs (Gmail, Calendar, Tasks, Contacts)
    ↓ [Source Syncers]
Raw items (emails, events, tasks, contact birthdays)
    ↓ [Normalizer]
Unified Alert documents
    ↓ [Dedup + TTL]
MongoDB `alerts` collection
    ↓ [REST API]
Embedded React UI
```

### Project Structure

```
assistant-agent/
├── main.go                         # Entry point, wire up daemon + API server
├── go.mod / go.sum
├── Makefile
├── config.yaml.example
│
├── cmd/
│   └── root.go                     # CLI flags (--serve, --sync-once, --daemon, --auth)
│
├── internal/
│   └── models/
│       ├── alert.go                # Unified alert model
│       ├── source.go               # Source metadata (sync state)
│       └── config.go               # Config struct
│
├── pkg/
│   ├── config/                     # Viper config loading
│   ├── db/
│   │   ├── mongo.go                # Connection, indexes, lifecycle
│   │   ├── alerts.go               # Alert CRUD + queries (upcoming, missed, by type)
│   │   └── sync_state.go           # Per-source sync bookmarks
│   ├── auth/
│   │   └── google.go               # OAuth2 flow (shared across all sources)
│   ├── sources/
│   │   ├── source.go               # Syncer interface
│   │   ├── gmail/
│   │   │   ├── client.go           # Gmail API client
│   │   │   └── parsers.go          # Extract subscriptions/payments from emails
│   │   ├── calendar/
│   │   │   └── client.go           # Google Calendar API — events + birthdays
│   │   ├── tasks/
│   │   │   └── client.go           # Google Tasks API — tasks with due dates
│   │   └── contacts/
│   │       └── client.go           # Google People API — contact birthdays
│   ├── engine/
│   │   ├── normalizer.go           # Convert raw source data → unified Alert
│   │   ├── dedup.go                # Cross-source deduplication
│   │   └── expiry.go               # TTL computation per alert type
│   ├── daemon/
│   │   └── daemon.go               # Background sync scheduler
│   └── api/
│       ├── server.go               # Router + static file serving
│       ├── handlers_alerts.go      # Alert endpoints
│       ├── handlers_sync.go        # Sync trigger/status
│       └── middleware.go           # CORS, logging, error handling
│
├── web/                            # Embedded UI (Vite + React + TS)
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── index.html
│   ├── src/
│   │   ├── main.tsx
│   │   ├── App.tsx
│   │   ├── pages/
│   │   │   ├── Dashboard.tsx       # Upcoming + today + missed overview
│   │   │   ├── Alerts.tsx          # Full alert list with filters
│   │   │   └── Settings.tsx        # Window config, sync status
│   │   ├── components/
│   │   ├── hooks/
│   │   └── lib/
│   │       └── api.ts              # API client
│   └── dist/                       # Build output (gitignored, embedded in Go binary)
│
└── docs/
    └── specs/
```

### Run Modes

| Mode | Command | Behavior |
|------|---------|----------|
| Full | `assistant-agent --serve` | API + UI + daemon (default) |
| Sync once | `assistant-agent --sync-once` | Single sync, then exit |
| Headless | `assistant-agent --daemon` | Daemon only, no UI |
| Auth | `assistant-agent --auth` | OAuth browser flow, store token, exit |

---

## Data Model

### Collection: `alerts`

| Field | Type | Description |
|-------|------|-------------|
| `_id` | ObjectID | Auto-generated |
| `user_id` | string | Owner identifier |
| `type` | string | `birthday`, `subscription`, `payment`, `task`, `event` |
| `title` | string | Display name |
| `description` | string | Extra context (email snippet, calendar notes) |
| `due_date` | time | When it's due/happening |
| `recurrence` | string | `none`, `monthly`, `yearly`, `weekly`, `custom` |
| `next_occurrence` | time | Computed next date for recurring items |
| `amount` | float64 | Payment amount if applicable (nullable) |
| `currency` | string | Default "INR" |
| `source` | string | `gmail`, `calendar`, `tasks`, `contacts` |
| `source_ref` | string | Gmail message ID / Calendar event ID / Task ID / Contact ID |
| `source_raw` | string | Original subject line or event title |
| `status` | string | `upcoming`, `due_today`, `missed`, `acknowledged` |
| `priority` | string | `low`, `medium`, `high` |
| `window_before` | int | Days before `due_date` to surface this alert |
| `expires_at` | time | TTL — MongoDB auto-deletes after this |
| `acknowledged_at` | time | When user dismissed (nullable) |
| `snoozed_until` | time | Snooze target date (nullable) |
| `tags` | []string | User-defined tags |
| `metadata` | map | Type-specific extras |
| `created_at` | time | Record creation |
| `updated_at` | time | Last update |

**Indexes**:
- `(user_id, status, due_date)` — primary query for upcoming/missed views
- `(user_id, type, due_date)` — filter by alert type
- `(source, source_ref)` unique — deduplication within same source
- `expires_at` TTL index — MongoDB auto-deletes expired documents

### Collection: `sync_state`

| Field | Type | Description |
|-------|------|-------------|
| `_id` | ObjectID | Auto-generated |
| `user_id` | string | Owner |
| `source` | string | `gmail`, `calendar`, `tasks`, `contacts` |
| `last_sync_at` | time | Last successful sync |
| `last_page_token` | string | Gmail historyId / Calendar syncToken / etc. |
| `total_processed` | int64 | Lifetime items processed |
| `last_error` | string | Last error if any |
| `status` | string | `idle`, `syncing`, `error` |

**Unique index**: `(user_id, source)`

### Collection: `settings`

| Field | Type | Description |
|-------|------|-------------|
| `_id` | ObjectID | Auto-generated |
| `user_id` | string | Owner |
| `windows` | map | Per-type lookahead: `{"birthday": 7, "subscription": 3, "payment": 5, "task": 1, "event": 2}` |
| `ttl` | map | Per-type expiry days after due: `{"birthday": 2, "subscription": 7, "payment": 7, "task": 14, "event": 1}` |
| `poll_interval` | string | Daemon poll frequency (default "15m") |
| `timezone` | string | "Asia/Kolkata" |
| `initial_lookback` | string | How far back on first sync ("3m") |

### Alert Lifecycle

```
[Source synced] → status: "upcoming"   (due_date is in the future, within window)
              → status: "due_today"  (due_date == today)
              → status: "missed"     (due_date < today, not acknowledged)

[User action]  → status: "acknowledged" (user marked as seen/done)

[Snooze]       → snoozed_until set, re-surfaces on that date

[TTL expires]  → document auto-deleted by MongoDB (no intermediate state)

[Recurring]    → after acknowledged, compute next_occurrence, create new alert
```

**TTL computation per type**:

| Type | Expires after due_date |
|------|----------------------|
| birthday | +2 days |
| subscription | +7 days |
| payment | +7 days |
| task | +14 days |
| event | +1 day |

---

## Source Syncers

### Common Interface

```go
type Syncer interface {
    Name() string
    Sync(ctx context.Context, state *SyncState) ([]RawItem, *SyncState, error)
}

type RawItem struct {
    Type        string
    Title       string
    Description string
    DueDate     time.Time
    Amount      *float64
    Recurrence  string
    SourceRef   string
    SourceRaw   string
    Metadata    map[string]interface{}
}
```

### Gmail Syncer

**Scope**: `gmail.readonly`

**Extracts**: Subscription renewals, payment due reminders, booking confirmations with dates.

**Template matching (rules-only)**:
- Subject line patterns: "your subscription", "payment due", "renewal", "upcoming charge", "bill generated", "reminder"
- Sender whitelist: configurable known billers
- Body extraction: regex for amounts (₹/INR/Rs patterns), dates, service names

**Sync strategy**:
- First run: backfill using `after:<date>` query filter
- Incremental: Gmail `historyId` to fetch only new messages
- Query: `(subject:subscription OR subject:renewal OR subject:payment OR subject:due OR subject:bill OR subject:reminder)`

### Calendar Syncer

**Scope**: `calendar.readonly`

**Extracts**: All events from primary calendar + "Birthdays" calendar.

**Mapping to alert types**:
- "Birthdays" calendar events → type: `birthday`
- Events with keywords ("pay", "due", "bill", "EMI", "renew") → type: `payment`
- Recurring events matching subscription patterns → type: `subscription`
- Other events → type: `event`

**Sync strategy**:
- First run: fetch events from (today - lookback) to (today + 90 days)
- Incremental: Calendar `syncToken` for delta updates
- Recurring events expanded into individual occurrences within the window

### Tasks Syncer

**Scope**: `tasks.readonly`

**Extracts**: All tasks with due dates across all task lists. Tasks without due dates are skipped.

**Mapping**: All dated tasks → type: `task`. Priority inferred from proximity to due date.

**Sync strategy**:
- Fetch all task lists, then tasks per list
- Filter: `showCompleted=false`, `dueMin=<lookback>`, `dueMax=<future_window>`
- Full fetch each cycle (Tasks API has no sync token; dataset is small)

### Contacts Syncer

**Scope**: `contacts.readonly` (People API)

**Extracts**: All contacts with a birthday field. Produces yearly recurring birthday alerts.

**Sync strategy**:
- Full fetch each cycle (contacts change rarely)
- Compute next upcoming birthday from month/day relative to today
- Compare against existing alerts by `source_ref` (contact resource name)
- Deduplicated against Calendar birthdays using normalized name matching

### Cross-Source Deduplication

1. **Same source**: unique index on `(source, source_ref)` prevents duplicates
2. **Cross-source**: match by `(type, normalized_title, due_date within ±1 day)`. Keep the richer record (one with amount/description), store secondary source_ref in metadata.

---

## Daemon & Sync Pipeline

### Pipeline Per Tick

```
1. For each source (parallel goroutines):
   a. Load sync_state from DB
   b. Call source.Sync(ctx, state)
   c. For each raw item:
      - Normalize → Alert struct
      - Compute expires_at (TTL)
      - Compute initial status
      - Dedup check (upsert with source_ref)
      - Cross-source dedup pass
   d. Update sync_state

2. Status refresh pass:
   - alerts where due_date < now AND status = "upcoming" → set "missed"
   - alerts where due_date == today AND status = "upcoming" → set "due_today"
   - snoozed alerts where snoozed_until <= today → clear snooze, re-surface

3. Recurring alert rotation:
   - Acknowledged recurring alerts → compute next_occurrence
   - Create new alert for next cycle if within lookahead window
```

### First-Run / Auth Flow

1. User runs `assistant-agent --auth`
2. No token found → open browser for Google OAuth consent (all 4 scopes in one flow)
3. User grants access → refresh token + access token stored in `token.json`
4. On first `--serve` or `--sync-once`: prompt for initial lookback period
5. Full backfill sync from chosen date
6. Store sync_state baselines per source
7. Subsequent runs: silent token refresh, incremental sync

### Error Handling

- **Auth expired**: attempt silent refresh. If fails, mark source status `error`, continue other sources. UI shows re-auth prompt.
- **Rate limits**: exponential backoff per source. Google APIs have generous quotas for personal use (~10k requests/day).
- **Partial failure**: each source syncs independently. One failing doesn't block others.
- **Network down**: skip cycle, retry next tick, log warning.
- **Daemon consecutive failures**: after 5 consecutive failures for a source, pause that source and surface in sync status.

---

## REST API

Served by Go binary (Gin router). Serves both JSON endpoints and embedded static UI files.

### Endpoints

```
GET  /api/alerts                    — all active alerts (filterable)
     ?type=birthday,subscription
     ?status=upcoming,missed
     ?from=2026-06-16&to=2026-06-30
     ?limit=50&offset=0

GET  /api/alerts/upcoming           — alerts within configured windows
GET  /api/alerts/missed             — past due, not acknowledged
GET  /api/alerts/today              — due today

GET  /api/alerts/:id                — single alert detail

PATCH /api/alerts/:id/acknowledge   — mark as seen/done
PATCH /api/alerts/:id/snooze        — snooze until date
      ?until=2026-06-20

GET  /api/sync/status               — per-source sync state
POST /api/sync/trigger              — trigger immediate sync
     ?source=gmail                  — optional: specific source only

GET  /api/settings                  — user settings
PUT  /api/settings                  — update settings (windows, poll interval)

GET  /health                        — health check
```

### Static UI Serving

```go
//go:embed web/dist/*
var webFS embed.FS

// Any non-/api/ route serves the React SPA (client-side routing)
router.NoRoute(serveStaticFS(webFS))
```

- **Development**: Vite dev server on `:5173`, proxy `/api` to Go on `:8080`
- **Production**: Go serves everything on `:8080`

---

## Configuration

Config loaded via Viper from `config.yaml` (gitignored) or `config.yaml.example` (committed template).
Environment variable override prefix: `ASSISTANT_`.

```yaml
db:
  uri: "mongodb://localhost:27017"
  database: "assistant-agent"
  timeout: 10s

google:
  credentials_file: "./credentials.json"
  token_file: "./token.json"
  scopes:
    - "https://www.googleapis.com/auth/gmail.readonly"
    - "https://www.googleapis.com/auth/calendar.readonly"
    - "https://www.googleapis.com/auth/tasks.readonly"
    - "https://www.googleapis.com/auth/contacts.readonly"

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

---

## Implementation Phases

| Phase | Scope | Deliverable |
|-------|-------|-------------|
| **P0** | Skeleton — Go project, config, MongoDB, health endpoint, Makefile | `make build`, `GET /health` |
| **P1** | Google Auth — single OAuth2 flow for all 4 scopes, token storage | `--auth` opens browser, stores token |
| **P2** | Calendar + Contacts sync — events + birthdays → alerts collection | Birthday and event alerts in DB |
| **P3** | Tasks sync — tasks with due dates → alerts | Task alerts in DB |
| **P4** | Gmail sync — email parsing, template matching, subscription/payment extraction | Subscription/payment alerts from emails |
| **P5** | Daemon — background polling, status refresh, recurring rotation | `--daemon` runs continuously |
| **P6** | REST API — all endpoints, pagination, filtering, acknowledge/snooze | Full API contract |
| **P7** | Embedded UI — Vite + React + TS dashboard | `--serve` shows web UI at `:8080` |
| **P8** | Cross-source dedup + polish — fuzzy matching, edge cases, error recovery | Clean merged timeline |

**MVP**: P0–P6 (backend fully functional via API). P7 adds visual layer. P8 is polish.

---

## Docker Deployment

```yaml
services:
  assistant-agent:
    build: .
    command: ["./assistant-agent", "--serve"]
    env_file: .env
    ports:
      - "8080:8080"
    volumes:
      - ./credentials.json:/app/credentials.json
      - ./token.json:/app/token.json
    depends_on:
      - mongodb
    restart: unless-stopped

  mongodb:
    image: mongo:7
    ports:
      - "27017:27017"
    volumes:
      - mongo-data:/data/db

volumes:
  mongo-data:
```

---

## Prerequisites

### Google Cloud Setup

1. Create Google Cloud project
2. Enable APIs: Gmail, Calendar, Tasks, People (Contacts)
3. Create OAuth 2.0 credentials (desktop app type)
4. Download `credentials.json`
5. First run: `assistant-agent --auth` → browser consent → `token.json` stored

### MongoDB

Local: `mongod` on default port. Production: Atlas connection string in config.

---

## Future Considerations (Post v1)

- **AI parsing**: Gemini fallback for emails that don't match templates
- **Telegram notifications**: push alerts for high-priority items
- **Smart priority**: ML-based priority scoring from user acknowledgment patterns
- **iData-UI integration**: REST API consumed by the React Native app
- **Recurring pattern detection**: auto-detect subscription patterns from transaction history (link with finance-agent)
- **Multi-calendar support**: non-primary calendars, shared calendars
- **Snooze intelligence**: learn preferred snooze patterns per alert type
