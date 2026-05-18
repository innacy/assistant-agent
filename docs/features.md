# Life Admin Assistant Agent — Feature Documentation

> Personal closed-loop assistant that scans Gmail and Google Calendar, extracts actionable items, sends smart reminders via Telegram, and auto-verifies completion.

---

## Table of Contents

- [Data Source Integrations](#data-source-integrations)
- [Item Extraction Pipeline](#item-extraction-pipeline)
- [Item Lifecycle & Status Machine](#item-lifecycle--status-machine)
- [Verification Engine](#verification-engine)
- [Telegram Bot Interface](#telegram-bot-interface)
- [Scheduling & Scan Cycle](#scheduling--scan-cycle)
- [Storage Layer](#storage-layer)
- [LLM Integration](#llm-integration)
- [Configuration](#configuration)
- [Error Handling & Resilience](#error-handling--resilience)
- [Module Map](#module-map)

---

## Data Source Integrations

### Gmail Scanner (`src/scanner/gmail_scanner.py`)

- **OAuth2** authentication via shared Google credentials (`src/scanner/google_auth.py`) with both `gmail.readonly` and `calendar.readonly` scopes in a single consent flow.
- **Incremental fetch**: queries `newer_than:1d` on first scan, `newer_than:12h` on subsequent scans (tracked via `gmail_last_history_id` in config).
- Fetches up to **50 messages** per scan cycle.
- **Deduplication**: skips messages whose `gmail_<message_id>` already exists as a tracked item.
- Parses full message payload — extracts headers (subject, sender, date), body (plain text preferred, recursive multipart traversal), and Gmail labels.
- Body truncated to **3,000 characters** for extraction.
- **Confirmation search**: dedicated `search_confirmations()` method queries Gmail for receipt/payment/confirmation emails matching given search terms after a given date, used by the verification engine.

### Google Calendar Scanner (`src/scanner/calendar_scanner.py`)

- Reads the **primary calendar** via Calendar API v3.
- Fetches events for the next **30 days**, up to **100 events**, ordered by start time, with recurring events expanded (`singleEvents=True`).
- Deduplicates against existing items by `cal_<event_id>`.
- Extracts: summary, description, start datetime (handles both `dateTime` and all-day `date` formats), location, organizer email, attendee list, and all-day flag.

### Manual Input via Telegram

- Users can **forward any message** to the bot — the forwarded text or caption is saved as a custom tracked item.
- Users can type `/add <description>` to manually create an item.

---

## Item Extraction Pipeline

Incoming raw data passes through a two-tier extraction pipeline:

### Tier 1: Rule Engine (`src/extractor/rule_engine.py`)

Handles approximately 60–70% of items deterministically, with zero API cost.

**Sender pattern matching** — 26 known sender patterns across three categories:

| Category | Senders |
|----------|---------|
| Subscriptions | Netflix, Spotify, Amazon Prime, Disney+, YouTube Premium, Apple, Google Storage, GitHub, DigitalOcean, Hetzner, Namecheap, GoDaddy, Cloudflare |
| Bills / Utilities | Electricity, water, gas, internet, phone, credit card, insurance, rent |
| Renewals | Passport, license, warranty, domain renewal |

Each pattern also carries pre-built **verification search terms** for the auto-verify engine.

**Keyword classification** — when no sender matches, the engine scores the email text against three keyword lists:
- **Bill keywords** (12): bill, invoice, payment due, due date, amount due, pay by, balance due, statement, overdue, past due, reminder to pay, payment reminder.
- **Subscription keywords** (12): subscription, renewal, renew, auto-renewal, recurring, membership, plan, billing cycle, will be charged, upcoming charge, annual renewal.
- **Assignment keywords** (11): deadline, due date, submit by, submission, assignment, homework, project due, deliverable, please submit, turn in, hand in.

Items are classified by whichever category scores highest (minimum score of 1).

**Calendar event classification** — calendar items bypass sender/keyword matching and are classified by event text:
- Contains assignment/deadline/submit/due keywords → `assignment`
- Contains bill/pay/payment keywords → `bill`
- Everything else → `appointment`

**Amount extraction** — 5 regex patterns covering:
- Indian Rupees: `Rs.`, `INR`, `₹`
- US Dollars: `$`, `USD`
- Euro: `EUR`, `€`
- Generic "amount:" and "total:" prefixes

**Date extraction** — 4 regex patterns for due dates in formats like:
- `due on 05/15/2026`, `pay by 15-05-2026`
- `deadline: 15 May 2026`, `renews 15 January 2026`
- Falls back to `python-dateutil` fuzzy parsing, then to the email's own date.

**Currency detection** — identifies INR, USD, or EUR from text patterns.

**Title generation** — builds a clean title from the subject line (max 80 chars) with a category prefix if not already present.

### Tier 2: LLM Extractor (`src/extractor/llm_extractor.py`)

Falls back for the remaining 25–30% of emails that the rule engine cannot classify.

- Uses a structured **JSON extraction prompt** asking the LLM to determine if an email is actionable and extract: title, category, amount, currency, due date, and verification terms.
- Parses the LLM's JSON response with a fallback brace-slice parser for malformed output.
- Non-actionable emails (newsletters, promotions, social notifications) return `{"actionable": false}` and are skipped.

### Verification Strategy Assignment

The extraction pipeline assigns a verification strategy to each item:

| Condition | Strategy |
|-----------|----------|
| Gmail source + bill or subscription category | `email_confirmation` (auto-verifiable) |
| Assignment or follow-up category | `manual` (user must mark done) |
| Calendar appointments | `none` |
| Everything else | `none` |

---

## Item Lifecycle & Status Machine

Every tracked item follows this state machine:

```
                    ┌─────────────────────────────────────┐
                    │                                     │
  [New] ──► DETECTED ──► UPCOMING ──► REMINDED ──► OVERDUE
                                        │    │        │
                                        │    │        ├──► AUTO_VERIFIED ──► [Done]
                                        │    │        └──► MANUAL_DONE ──► [Done]
                                        │    │
                                        │    └──► SNOOZED ──► REMINDED (un-snooze)
                                        │
                                        ├──► AUTO_VERIFIED ──► [Done]
                                        └──► MANUAL_DONE ──► [Done]
                                                             DISMISSED ──► [Done]
```

**Status transitions** (`src/verifier/verification.py`):

| From | To | Trigger |
|------|----|---------|
| `detected` | `upcoming` | Days until due date falls within the category's earliest reminder window |
| `upcoming` / `reminded` | `overdue` | Due date has passed |
| `snoozed` | `reminded` | Snooze expiry time reached (`unsnooze_expired`) |
| `reminded` / `overdue` | `done` | Confirmation email found (auto-verified) |
| Any active | `done` | User clicks "Mark Done" in Telegram |
| Any active | `done` (dismissed) | User clicks "Dismiss" in Telegram |
| Any active | `snoozed` | User clicks Snooze (1 day / 3 days / 1 week) |

### Reminder Window Logic

The `should_remind` function gates which items appear in each digest, preventing notification noise:

| Category | Reminder Windows (days before due) |
|----------|-----------------------------------|
| Bill | 7, 3, 1, 0 (day of) |
| Subscription | 7, 3, 1, 0 |
| Assignment | 3, 1, 0 |
| Appointment | 1 |
| Renewal | 30, 7, 1 |
| Follow-up | 2 (days after detection) |

- **Overdue items**: always included in digests.
- **Snoozed / done / dismissed**: never included.
- Items without a due date: included if status is `upcoming` or `reminded`.

---

## Verification Engine

### Email Auto-Verification (`src/verifier/email_verifier.py`)

The core "closed-loop" feature — the agent checks if you've already handled something before nagging you again.

**How it works:**

1. Loads items in `reminded`, `overdue`, or `upcoming` status that have `email_confirmation` verification strategy and non-empty search terms.
2. Sorts by due date (most urgent first).
3. Processes up to **10 items per scan cycle** (rate-limited to avoid Gmail API quota issues).
4. For each item, searches Gmail for confirmation/receipt emails matching the item's verification terms, sent after the item was first detected.
5. Filters out the original triggering email (same message ID) to avoid false positives.
6. If a confirmation is found → marks the item as `done` with `resolved_by=auto_verified` and stores the confirmation email's ID.
7. Sends a Telegram alert for each auto-verified item.

**Example flow:**
- Agent detects "Netflix renewal - $15.99 due Apr 29" from a notification email.
- Verification terms: `["netflix", "payment", "receipt"]`.
- On the next scan, the verifier searches Gmail for emails containing those terms + "receipt OR confirmation OR payment OR successful".
- Finds "Your Netflix payment receipt" → auto-marks as done, notifies you in Telegram: "Auto-verified: Netflix renewal (confirmed via email)".

---

## Telegram Bot Interface

### Security

All commands, messages, and forwarded content are gated by the `_authorized` decorator — only the configured `TELEGRAM_CHAT_ID` can interact with the bot.

### Commands

| Command | Description |
|---------|-------------|
| `/start` | Welcome message with intro text |
| `/help` | Full command reference |
| `/status` | Last scan stats (emails/events processed, items found, errors) + pending item counts by status |
| `/upcoming` | All items in detected/upcoming/reminded status, formatted as a digest |
| `/overdue` | All overdue items |
| `/bills` | Pending items filtered to bill category |
| `/subscriptions` | Pending items filtered to subscription category |
| `/assignments` | Pending items filtered to assignment category |
| `/add <text>` | Manually create a custom tracked item |
| `/scan` | Trigger an immediate manual scan cycle |

### Digest Messages

Sent automatically after each scan cycle (morning and evening). Structured as:

```
*Morning Digest (May 18, 2026)*

🚨 *OVERDUE (1)*
  💰 Electricity bill - INR 2,340.00 (due May 15)

⏰ *DUE THIS WEEK (3)*
  🔄 Netflix renewal - USD 15.99 (due May 22)
  💰 Credit card payment - INR 45,000.00 (due May 23)
  📝 ML Lab Report (due May 24)

📋 *UPCOMING (2)*
  📋 Passport renewal (due Jun 15)
  📅 Doctor appointment (due Jun 10)

✅ *AUTO-VERIFIED*
  ✓ Amazon Prime renewed (confirmed via email)
```

**Category emojis**: 💰 bill, 🔄 subscription, 📋 renewal, 📝 assignment, 📅 appointment, 📧 follow-up, 📌 custom.

### Inline Action Buttons

Each item due within 7 days gets its own message with interactive buttons:

| Button | Action |
|--------|--------|
| **Mark Done** | Resolves the item (`resolved_by=manual`) |
| **Snooze 1d** | Postpones reminders for 1 day |
| **Snooze 3d** | Postpones reminders for 3 days |
| **Snooze 1w** | Postpones reminders for 1 week |
| **Dismiss** | Permanently removes from tracking |
| **Details** | Shows full item details (category, amount, due date, verification status, reminders sent, tracked-since date) |

### Free-Text Queries

Plain text messages are checked against query heuristics (keywords like "what", "how many", "bills", "due", "upcoming", "overdue", "pending", "status"). Matching messages are routed to the LLM for a natural-language answer based on current pending items.

Example: "what bills are due this week?" → LLM receives a summary of all pending items and answers conversationally.

### Forwarded Messages

Any forwarded message is automatically saved as a custom tracked item with the forwarded text as the title.

---

## Scheduling & Scan Cycle

### Schedule (`src/main.py`)

| Job | Default Time | Configurable Via |
|-----|-------------|-----------------|
| Morning scan | 07:00 | `MORNING_SCAN_TIME` |
| Evening scan | 19:00 | `EVENING_SCAN_TIME` |
| Startup scan | ~5 seconds after boot | Always runs |

All jobs use APScheduler's `AsyncIOScheduler` with `CronTrigger` and `misfire_grace_time=3600` (jobs missed by up to 1 hour still execute).

### Scan Cycle Pipeline (`src/scheduler/jobs.py`)

Each scan cycle executes these steps in order:

1. **Gmail scan** — fetch new emails since last scan
2. **Calendar scan** — fetch events for next 30 days
3. **Extract items** — for each raw item not already tracked:
   - Try rule engine first
   - Fall back to LLM if rules produce nothing
   - Upsert to MongoDB
4. **Update statuses** — transition detected→upcoming→overdue based on dates; un-snooze expired items
5. **Verify pending items** — search Gmail for confirmation emails, auto-resolve up to 10 items
6. **Send Telegram alerts** — individual alerts for each auto-verified item
7. **Build and send digest** — filtered by `should_remind` logic, only items within their reminder window
8. **Record scan** — persist scan metadata (duration, counts, errors)

---

## Storage Layer

### MongoDB Collections (`src/storage/mongodb.py`)

All collections are prefixed with `assistant_` and live in the existing `idata-dev` database.

| Collection | Purpose | Key Indexes |
|-----------|---------|-------------|
| `assistant_items` | Tracked actionable items | `source_id` (unique), `status`, `due_date`, `category` |
| `assistant_scans` | Scan execution records | `started_at` |
| `assistant_notifications` | Telegram message tracking | `item_id`, `sent_at` |
| `assistant_config` | Single-document agent configuration | — |

### Key Operations

- **Items**: upsert by `source_id` (idempotent), query by status/category, mark done (with optional verification fields), snooze with expiry, dismiss, increment reminder count, bulk un-snooze expired.
- **Scans**: insert record, get latest scan.
- **Notifications**: record sent message, update user action (done/snooze/dismiss) by Telegram message ID.
- **Config**: get-or-create with defaults, partial update.

### Connection Resilience

MongoDB connection uses **exponential backoff retry** — up to 5 attempts with `2^attempt` second delays. Connection validated with a `ping` command. `serverSelectionTimeoutMS=5000`.

---

## LLM Integration

### Providers

| Provider | Role | Model | Cost |
|----------|------|-------|------|
| **Ollama** (primary) | Local LLM for extraction + Q&A | `llama3.1:8b` (configurable) | Free |
| **OpenAI** (optional fallback) | Cloud fallback when Ollama fails | `gpt-4o-mini` | ~$0.15/1M input tokens |

### Integration Points

1. **Email extraction** (`LLMExtractor.extract`) — called when the rule engine returns no match. Sends email subject/sender/date/body to the LLM with a structured JSON extraction prompt.
2. **Free-text Q&A** (`LLMExtractor.answer_question`) — called from the Telegram bot for natural-language queries. Passes a summary of up to 20 pending items as context.

### Fallback Chain

```
Ollama (local, free) ──[failure]──► OpenAI (cloud, cheap)
                                         │
                                    [failure]──► None (item skipped / help text shown)
```

Fallback is **opt-in** via `CLOUD_LLM_FALLBACK=true` and requires `OPENAI_API_KEY`.

---

## Configuration

All settings are environment variables loaded via Pydantic Settings (supports `.env` file).

| Variable | Default | Description |
|----------|---------|-------------|
| `MONGO_URI` | `mongodb://localhost:27017` | MongoDB connection string |
| `DATABASE_NAME` | `idata-dev` | Database name |
| `TELEGRAM_BOT_TOKEN` | — | Telegram bot token from BotFather |
| `TELEGRAM_CHAT_ID` | — | Your Telegram chat ID (authorization) |
| `GOOGLE_CREDENTIALS_PATH` | `data/credentials.json` | Google OAuth client secrets |
| `GOOGLE_TOKEN_PATH` | `data/token.json` | Saved OAuth refresh token |
| `TIMEZONE` | `Asia/Kolkata` | Timezone for scheduling and display |
| `MORNING_SCAN_TIME` | `07:00` | Morning scan time (24h format) |
| `EVENING_SCAN_TIME` | `19:00` | Evening scan time (24h format) |
| `LLM_PROVIDER` | `ollama` | Primary LLM provider |
| `LLM_MODEL` | `llama3.1:8b` | Ollama model name |
| `OLLAMA_HOST` | `http://ollama:11434` | Ollama API endpoint |
| `CLOUD_LLM_FALLBACK` | `false` | Enable OpenAI fallback |
| `OPENAI_API_KEY` | — | OpenAI API key (if fallback enabled) |
| `LOG_LEVEL` | `INFO` | Logging level |

### Stored Config (`assistant_config` collection)

Runtime-adjustable settings persisted in MongoDB:
- Scan schedule, timezone
- Reminder windows per category (customizable days-before arrays)
- LLM provider and model
- Gmail last history ID and calendar sync token (scan state)

---

## Error Handling & Resilience

| Component | Failure Behavior |
|-----------|-----------------|
| MongoDB connection | Exponential backoff retry (5 attempts), then crash |
| Scanner initialization | Log error, continue without scanners (bot still works for manual items) |
| Telegram bot startup | Log error, close DB, exit with code 1 (Telegram is essential) |
| Scheduled/startup scans | Catch all exceptions, log, do not crash the process |
| Individual email/event parsing | Log and skip the single item, continue processing others |
| Item extraction (rule + LLM) | Log per-item, append to scan error list, continue |
| Scan cycle outer failure | Record scan with error metadata, log full exception |
| Gmail API calls | Catch, log, return empty results |
| Ollama LLM call | Log, optionally fallback to OpenAI |
| OpenAI LLM call | Log, return None (item skipped or help text shown) |
| Telegram LLM Q&A | Log, show fallback help text |
| Verification per-item | Log and skip, continue to next item |
| Graceful shutdown | SIGINT/SIGTERM → stop scheduler, stop Telegram polling, close MongoDB |

---

## Module Map

```
src/
├── main.py                  # Entry point, logging, scheduler, graceful shutdown
├── config.py                # Pydantic settings from environment/.env
│
├── scanner/
│   ├── base.py              # RawItem dataclass + BaseScanner interface
│   ├── google_auth.py       # Shared Google OAuth2 (Gmail + Calendar scopes)
│   ├── gmail_scanner.py     # Gmail API: fetch emails, search confirmations
│   └── calendar_scanner.py  # Calendar API: fetch 30 days of events
│
├── extractor/
│   ├── categories.py        # 26 sender patterns, keyword lists, regex patterns
│   ├── rule_engine.py       # Deterministic extraction (sender/keyword/regex)
│   └── llm_extractor.py     # LLM extraction + free-text Q&A
│
├── verifier/
│   ├── verification.py      # Status transitions, reminder window gating
│   └── email_verifier.py    # Gmail confirmation search, auto-resolve
│
├── notifier/
│   ├── telegram_bot.py      # Full Telegram UX (commands, callbacks, forwarding)
│   ├── digest_builder.py    # Markdown digest/detail/status formatting
│   └── buttons.py           # Inline keyboard definitions + callback parsing
│
├── storage/
│   ├── models.py            # Pydantic models, enums, default configs
│   └── mongodb.py           # Motor async client, all CRUD, indexes, retries
│
└── scheduler/
    └── jobs.py              # Scan cycle orchestration (scan→extract→verify→notify)
```

---

## Docker Deployment

```yaml
services:
  assistant-agent:    # Python app
    build: .
    env_file: .env
    depends_on: [ollama]
    volumes: [./data:/app/data]  # OAuth tokens
    restart: unless-stopped
    network_mode: host

  ollama:             # Local LLM
    image: ollama/ollama
    volumes: [ollama_data:/root/.ollama]
    ports: ["11434:11434"]
    restart: unless-stopped
```

MongoDB is **not** in compose — it connects to the existing iData MongoDB instance via `MONGO_URI`.
