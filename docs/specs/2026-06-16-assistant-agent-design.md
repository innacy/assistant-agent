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
│   │   ├── history.go              # Alert history archival + queries
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
│       ├── handlers_alerts.go      # Alert CRUD + actions
│       ├── handlers_history.go     # History queries
│       ├── handlers_sync.go        # Sync trigger/status
│       └── middleware.go           # Bearer token auth, CORS, logging
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
| `source` | string | `gmail`, `calendar`, `tasks`, `contacts`, `manual` |
| `source_ref` | string | Gmail message ID / Calendar event ID / Task ID / Contact ID. For recurring instances, includes occurrence date suffix (e.g., `event_abc:2026-07-15`). For manual: `manual:<uuid>` |
| `source_raw` | string | Original subject line or event title |
| `status` | string | `upcoming`, `due_today`, `missed`, `snoozed`, `acknowledged` |
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

### Collection: `alert_history`

Same schema as `alerts`, plus:

| Field | Type | Description |
|-------|------|-------------|
| `archived_at` | time | When the alert was moved to history |
| `outcome` | string | `acknowledged`, `expired_unacknowledged` |

Alerts are moved here:
- When acknowledged and past due_date + TTL window (completed lifecycle)
- When TTL would delete an unacknowledged alert (missed and expired)

No TTL index on this collection — history is kept permanently. Enables queries like "what did I pay last month?" and "whose birthdays did I miss?"

**Index**: `(user_id, type, due_date)`, `(user_id, archived_at)`

### Alert Lifecycle

```
[Source synced] → status: "upcoming"   (due_date is in the future, within window)
              → status: "due_today"  (due_date == today)
              → status: "missed"     (due_date < today, not acknowledged)

[User action]  → status: "acknowledged" (user marked as seen/done)

[Snooze]       → status: "snoozed" (excluded from all views until snoozed_until date)
              → on snoozed_until date: status reverts to "upcoming" or "due_today"

[TTL expires]  → document moved to alert_history collection, then deleted from alerts

[Recurring]    → daily job creates new alert for next occurrence within lookahead window
```

**Status transitions**:
- `upcoming` → `due_today` (date arrives)
- `upcoming` → `missed` (date passes without acknowledgment)
- `due_today` → `missed` (day ends without acknowledgment)
- `upcoming`/`due_today`/`missed` → `snoozed` (user snoozes)
- `snoozed` → `upcoming`/`due_today` (snooze expires)
- any → `acknowledged` (user marks done)
- `acknowledged` → moved to `alert_history` → TTL deletes from `alerts`

**Note on status computation**: The API computes status dynamically at query time based on `due_date` vs current time. The stored `status` field is updated by the daemon but the API never trusts it blindly — it recalculates before returning results. This prevents stale data between daemon ticks.

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

**Two-tier parsing logic**:
1. **Whitelisted senders** (configured in `gmail.sender_whitelist`): all emails from these senders are processed unconditionally. High confidence — these are known billers.
2. **Unknown senders**: must match BOTH a subject pattern AND pass body validation (contains an amount pattern like ₹/INR/Rs, or a date reference, or a service name). This prevents marketing spam from triggering false alerts.

**Subject patterns**: "your subscription", "payment due", "renewal", "upcoming charge", "bill generated", "reminder", "invoice", "due date"

**Body validation for unknown senders**:
- Must contain at least ONE of: amount regex (`₹\d+`, `INR \d+`, `Rs\.?\s?\d+`), a future date reference, or a renewal/expiry keyword in context
- Extracts: service name (From header or body), amount, due date

**Duplicate email handling**: Multiple emails about the same subscription event (e.g., "renewing in 3 days" + "payment processed") are merged by normalizing the title and checking for existing alerts with the same `(type, normalized_title, due_date within ±3 days)`. The later email updates the existing alert rather than creating a new one.

**Sync strategy**:
- First run: backfill using `after:<date>` query filter
- Incremental: Gmail `historyId` to fetch only new messages
- **HistoryId expiry fallback**: If historyId returns HTTP 404 (expired after ~30 days of inactivity), fall back to date-based query `after:<last_sync_at>` and reset historyId from the response.
- Query: `(subject:subscription OR subject:renewal OR subject:payment OR subject:due OR subject:bill OR subject:reminder OR from:<whitelisted_senders>)`

### Calendar Syncer

**Scope**: `calendar.readonly`

**Extracts**: Filtered events from primary calendar + "Birthdays" calendar.

**Event filtering** (to avoid noise from meetings/standups):
- All-day events: always included
- Timed events: only if title matches keywords ("pay", "due", "bill", "EMI", "renew", "deadline", "expires", "appointment")
- "Birthdays" calendar: always included (all events)
- Regular meetings, standups, 1:1s, video calls: excluded

**Mapping to alert types**:
- "Birthdays" calendar events → type: `birthday`
- Events with payment keywords ("pay", "due", "bill", "EMI", "renew") → type: `payment`
- Recurring events matching subscription patterns → type: `subscription`
- Other qualifying events (all-day or keyword-matched) → type: `event`

**Sync strategy**:
- First run: fetch events from (today - lookback) to (today + 90 days)
- Incremental: Calendar `syncToken` for delta updates
- Recurring events expanded into individual occurrences within the window
- **Deletion handling**: Calendar syncToken responses include `cancelled` events. When detected, delete or mark the corresponding alert as acknowledged.
- Source ref for recurring instances: `<event_id>:<occurrence_date>` to ensure uniqueness

### Tasks Syncer

**Scope**: `tasks.readonly`

**Extracts**: All tasks with due dates across all task lists. Tasks without due dates are skipped.

**Mapping**: All dated tasks → type: `task`. Priority inferred from proximity to due date.

**Sync strategy**:
- Fetch all task lists, then tasks per list
- Filter: `showCompleted=false`, `dueMin=<lookback>`, `dueMax=<future_window>`
- Full fetch each cycle (Tasks API has no sync token; dataset is small)
- **External completion handling**: After fetching active tasks, compare fetched task IDs against existing `source: "tasks"` alerts. Any alert whose `source_ref` is NOT in the fetched set (and isn't already acknowledged) → mark as `acknowledged` (user completed it in Google Tasks directly).

### Contacts Syncer

**Scope**: `contacts.readonly` (People API)

**Extracts**: All contacts with a birthday field. Produces yearly recurring birthday alerts.

**Birthdays without year**: Many contacts store only month/day (no year). These are still valid — the alert is created with `metadata.birth_year: null`. The UI displays "Birthday" without age. If year is present, UI can optionally show age.

**Sync strategy**:
- Full fetch each cycle (contacts change rarely)
- Compute next upcoming birthday from month/day relative to today
- Compare against existing alerts by `source_ref` (contact resource name)
- Deduplicated against Calendar birthdays using normalized name matching
- Source ref: `<contact_resource_name>:<year>` (e.g., `people/c123:2026`) to allow yearly instances

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
   - snoozed alerts where snoozed_until <= today → revert to "upcoming" or "due_today"

3. History archival pass:
   - Acknowledged alerts past TTL window → move to alert_history, delete from alerts
   - Unacknowledged alerts past TTL window → move to alert_history with outcome "expired_unacknowledged", delete from alerts
```

### Daily Recurring Job (midnight)

Runs once per day (separate from the poll-interval tick):

```
1. Scan all known recurring patterns:
   - Alerts with recurrence != "none" that have been acknowledged
   - Contacts birthdays (yearly)
   - Calendar recurring events within the next 90 days

2. For each recurring pattern, compute next occurrence date

3. If next_occurrence falls within the alert's window_before:
   - Check if an alert already exists for that occurrence (by source_ref with date suffix)
   - If not, create a new "upcoming" alert

4. This ensures recurring alerts surface exactly window_before days in advance,
   regardless of when the previous occurrence was acknowledged.
```

### Graceful Shutdown

On SIGTERM/SIGINT:
1. Stop accepting new sync ticks
2. Wait for in-progress sync goroutines to finish (timeout: 30 seconds)
3. If timeout reached, cancel context (goroutines abort)
4. Run one final history archival pass
5. Close MongoDB connection
6. Exit

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

### Authentication

Simple bearer token authentication. Token is configured in `config.yaml` under `server.api_token`. All `/api/*` endpoints require `Authorization: Bearer <token>` header. The health endpoint is unauthenticated.

### Endpoints

```
# Alerts — read
GET  /api/alerts                    — all active alerts (filterable)
     ?type=birthday,subscription
     ?status=upcoming,missed,snoozed
     ?from=2026-06-16&to=2026-06-30
     ?limit=50&offset=0

GET  /api/alerts/upcoming           — alerts within configured windows
GET  /api/alerts/missed             — past due, not acknowledged
GET  /api/alerts/today              — due today

GET  /api/alerts/:id                — single alert detail

# Alerts — write
POST   /api/alerts                  — create manual alert (user-defined reminder)
PUT    /api/alerts/:id              — edit alert (title, due_date, tags, etc.)
DELETE /api/alerts/:id              — delete alert

# Alerts — actions
PATCH /api/alerts/:id/acknowledge   — mark as seen/done
PATCH /api/alerts/:id/snooze        — snooze until date
      ?until=2026-06-20

# Alerts — batch operations
POST /api/alerts/batch/acknowledge  — acknowledge multiple alerts
     body: { "ids": ["id1", "id2"] } OR { "filter": { "status": "missed", "type": "task" } }
POST /api/alerts/batch/snooze       — snooze multiple alerts
     body: { "ids": [...], "until": "2026-06-20" } OR { "filter": {...}, "until": "..." }

# History
GET  /api/history                   — archived alerts (same filters as /api/alerts)
     ?type=subscription&from=2026-05-01&to=2026-06-01&limit=50&offset=0

# Sync
GET  /api/sync/status               — per-source sync state
POST /api/sync/trigger              — trigger immediate sync
     ?source=gmail                  — optional: specific source only

# Settings
GET  /api/settings                  — user settings
PUT  /api/settings                  — update settings (windows, poll interval)

# Health (unauthenticated)
GET  /health                        — health check
```

### Response Envelope

All list endpoints return:

```json
{
  "data": [...],
  "total": 142,
  "limit": 50,
  "offset": 0,
  "has_more": true
}
```

Single item endpoints return the object directly. Action endpoints return `{ "ok": true }`.

### Manual Alert Creation (POST /api/alerts)

Request body:

```json
{
  "type": "payment",
  "title": "Renew passport",
  "description": "Expires December 2026",
  "due_date": "2026-12-01",
  "recurrence": "none",
  "amount": null,
  "priority": "high",
  "tags": ["documents"]
}
```

Manual alerts have `source: "manual"` and `source_ref: "manual:<generated_uuid>"`. They support full CRUD — edit title, change due_date, delete entirely.

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
  api_token: "your-secret-token-here"  # Required for API access

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

### OAuth Setup (before Docker)

Run auth locally first (requires browser):

```bash
# On your machine (not Docker)
assistant-agent --auth
# Browser opens → grant access → token.json saved locally
```

Then mount the token into Docker:

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
      - ./token.json:/app/token.json        # Pre-authenticated
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

If the token expires while running in Docker, the agent logs a warning and pauses syncing. Re-run `assistant-agent --auth` locally and restart the container with the fresh token.

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
