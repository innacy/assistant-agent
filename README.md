# Life Admin Assistant Agent

A personal closed-loop assistant that scans Gmail and Google Calendar, extracts actionable items (bills, subscriptions, renewals, deadlines), sends smart reminders via Telegram, and auto-verifies completion by checking for confirmation emails.

## Quick Start

### Prerequisites

- Docker & Docker Compose
- A Google Cloud project with Gmail and Calendar APIs enabled
- A Telegram bot token (from [@BotFather](https://t.me/BotFather))
- MongoDB running locally or accessible via network

### Setup

1. **Clone and configure:**
   ```bash
   cp .env.example .env
   # Edit .env with your credentials
   ```

2. **Google OAuth credentials:**
   - Go to [Google Cloud Console](https://console.cloud.google.com/)
   - Create a project, enable Gmail API and Google Calendar API
   - Create OAuth 2.0 credentials (Desktop application)
   - Download as `data/credentials.json`

3. **Telegram bot:**
   - Message [@BotFather](https://t.me/BotFather) to create a bot
   - Copy the token to `TELEGRAM_BOT_TOKEN` in `.env`
   - Send a message to your bot, then get your chat ID via `https://api.telegram.org/bot<TOKEN>/getUpdates`
   - Set `TELEGRAM_CHAT_ID` in `.env`

4. **First run (for OAuth consent):**
   ```bash
   pip install -r requirements.txt
   python -m src.main
   # Browser will open for Google OAuth consent
   # After authorizing, the token is saved to data/token.json
   ```

5. **Run with Docker Compose:**
   ```bash
   docker compose up -d
   ```

## Architecture

```
Gmail API + Google Calendar API
        ↓
   Scanner (twice daily)
        ↓
   Rule Engine → LLM (Ollama) for ambiguous items
        ↓
   MongoDB (assistant_* collections)
        ↓
   Telegram Bot (digests + inline actions)
        ↓
   Verification Engine (auto-checks for confirmation emails)
```

## Telegram Commands

| Command | Description |
|---------|-------------|
| `/upcoming` | Show upcoming items |
| `/overdue` | Show overdue items |
| `/bills` | Show pending bills |
| `/subscriptions` | Show subscription renewals |
| `/assignments` | Show assignment deadlines |
| `/add <text>` | Manually add an item |
| `/scan` | Trigger a manual scan |
| `/status` | Agent health and last scan info |

You can also forward messages to the bot or ask free-text questions.

## Configuration

All configuration via environment variables (see `.env.example`):

- `MONGO_URI` - MongoDB connection string
- `TELEGRAM_BOT_TOKEN` / `TELEGRAM_CHAT_ID` - Telegram bot credentials
- `MORNING_SCAN_TIME` / `EVENING_SCAN_TIME` - Scan schedule (24h format)
- `TIMEZONE` - Your timezone (default: Asia/Kolkata)
- `LLM_PROVIDER` - `ollama` or `openai`
- `CLOUD_LLM_FALLBACK` - Enable OpenAI fallback when Ollama fails
