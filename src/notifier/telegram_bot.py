from datetime import datetime, timedelta
from zoneinfo import ZoneInfo

import structlog
from telegram import Bot, Update
from telegram.ext import (
    Application,
    CallbackQueryHandler,
    CommandHandler,
    ContextTypes,
    MessageHandler,
    filters,
)

from src.config import settings
from src.notifier.buttons import (
    ACTION_DETAILS,
    ACTION_DISMISS,
    ACTION_DONE,
    ACTION_SNOOZE_1D,
    ACTION_SNOOZE_1W,
    ACTION_SNOOZE_3D,
    item_buttons,
    parse_callback,
)
from src.notifier.digest_builder import DigestBuilder
from src.storage.models import (
    AssistantItem,
    ItemCategory,
    ItemSource,
    ItemStatus,
    NotificationRecord,
)
from src.storage.mongodb import db

logger = structlog.get_logger()

digest_builder = DigestBuilder()


def _authorized(func):
    async def wrapper(update: Update, context: ContextTypes.DEFAULT_TYPE):
        chat_id = str(update.effective_chat.id)
        if chat_id != settings.telegram_chat_id:
            logger.warning("telegram.unauthorized", chat_id=chat_id)
            await update.message.reply_text("Unauthorized.")
            return
        return await func(update, context)
    return wrapper


class TelegramBot:
    def __init__(self) -> None:
        self._app: Application | None = None
        self._bot: Bot | None = None

    async def start(self) -> Application:
        self._app = (
            Application.builder()
            .token(settings.telegram_bot_token)
            .build()
        )

        self._app.add_handler(CommandHandler("start", self._cmd_start))
        self._app.add_handler(CommandHandler("help", self._cmd_help))
        self._app.add_handler(CommandHandler("status", self._cmd_status))
        self._app.add_handler(CommandHandler("upcoming", self._cmd_upcoming))
        self._app.add_handler(CommandHandler("overdue", self._cmd_overdue))
        self._app.add_handler(CommandHandler("bills", self._cmd_category_filter))
        self._app.add_handler(CommandHandler("subscriptions", self._cmd_category_filter))
        self._app.add_handler(CommandHandler("assignments", self._cmd_category_filter))
        self._app.add_handler(CommandHandler("add", self._cmd_add))
        self._app.add_handler(CommandHandler("scan", self._cmd_scan))
        self._app.add_handler(CallbackQueryHandler(self._handle_callback))
        self._app.add_handler(
            MessageHandler(filters.TEXT & ~filters.COMMAND, self._handle_message)
        )
        self._app.add_handler(
            MessageHandler(filters.FORWARDED, self._handle_forwarded)
        )

        await self._app.initialize()
        await self._app.start()
        await self._app.updater.start_polling(drop_pending_updates=True)

        self._bot = self._app.bot
        logger.info("telegram.started")
        return self._app

    async def stop(self) -> None:
        if self._app:
            await self._app.updater.stop()
            await self._app.stop()
            await self._app.shutdown()
            logger.info("telegram.stopped")

    async def send_digest(self, items: list[dict], auto_verified: list[dict] | None = None) -> None:
        if not self._bot:
            return

        text = digest_builder.build_digest(items, auto_verified)
        msg = await self._bot.send_message(
            chat_id=settings.telegram_chat_id,
            text=text,
            parse_mode="Markdown",
        )

        overdue_and_due = [
            i for i in items
            if i.get("status") in (
                ItemStatus.OVERDUE.value,
                ItemStatus.REMINDED.value,
                ItemStatus.UPCOMING.value,
                ItemStatus.DETECTED.value,
            )
            and i.get("due_date")
        ]

        tz = ZoneInfo(settings.timezone)
        now = datetime.now(tz)
        week_end = now + timedelta(days=7)

        for item in overdue_and_due:
            due = item.get("due_date")
            if due and not due.tzinfo:
                due = due.replace(tzinfo=tz)
            if not due or due > week_end:
                continue

            item_id = str(item["_id"])
            title = item.get("title", "Unknown")
            amount_str = ""
            if item.get("amount"):
                amount_str = f" - {item.get('currency', '')} {item['amount']:,.2f}"

            item_msg = await self._bot.send_message(
                chat_id=settings.telegram_chat_id,
                text=f"📌 *{title}*{amount_str}",
                parse_mode="Markdown",
                reply_markup=item_buttons(item_id),
            )

            await db.record_notification(
                NotificationRecord(
                    item_id=item_id,
                    type="digest",
                    message_id=str(item_msg.message_id),
                    status="delivered",
                )
            )
            await db.increment_reminder(item_id)

    async def send_verification_alert(self, item: dict) -> None:
        if not self._bot:
            return
        title = item.get("title", "Unknown")
        await self._bot.send_message(
            chat_id=settings.telegram_chat_id,
            text=f"✅ *Auto-verified:* {title}\n_Confirmation email found._",
            parse_mode="Markdown",
        )

    @_authorized
    async def _cmd_start(self, update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
        await update.message.reply_text(
            "🤖 *Life Admin Agent*\n\n"
            "I scan your Gmail and Calendar to track bills, subscriptions, "
            "renewals, deadlines, and more.\n\n"
            "Use /help to see available commands.",
            parse_mode="Markdown",
        )

    @_authorized
    async def _cmd_help(self, update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
        await update.message.reply_text(
            "*Commands:*\n"
            "/upcoming - Show upcoming items\n"
            "/overdue - Show overdue items\n"
            "/bills - Show bills\n"
            "/subscriptions - Show subscriptions\n"
            "/assignments - Show assignments\n"
            "/add <description> - Add a manual item\n"
            "/scan - Trigger a manual scan\n"
            "/status - Agent health and last scan\n"
            "/help - This message\n\n"
            "You can also forward messages to me to add manual items, "
            "or ask questions in plain text.",
            parse_mode="Markdown",
        )

    @_authorized
    async def _cmd_status(self, update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
        last_scan = await db.get_last_scan()
        pending = await db.get_pending_items()

        counts: dict[str, int] = {}
        for item in pending:
            status = item.get("status", "unknown")
            counts[status] = counts.get(status, 0) + 1

        text = digest_builder.build_status_message(last_scan, counts)
        await update.message.reply_text(text, parse_mode="Markdown")

    @_authorized
    async def _cmd_upcoming(self, update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
        items = await db.get_items_by_status(
            ItemStatus.DETECTED, ItemStatus.UPCOMING, ItemStatus.REMINDED
        )
        if not items:
            await update.message.reply_text("🎉 Nothing upcoming!")
            return

        text = digest_builder.build_digest(items)
        await update.message.reply_text(text, parse_mode="Markdown")

    @_authorized
    async def _cmd_overdue(self, update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
        items = await db.get_items_by_status(ItemStatus.OVERDUE)
        if not items:
            await update.message.reply_text("🎉 Nothing overdue!")
            return

        text = digest_builder.build_digest(items)
        await update.message.reply_text(text, parse_mode="Markdown")

    @_authorized
    async def _cmd_category_filter(self, update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
        command = update.message.text.split()[0].lstrip("/")
        cat_map = {
            "bills": ItemCategory.BILL,
            "subscriptions": ItemCategory.SUBSCRIPTION,
            "assignments": ItemCategory.ASSIGNMENT,
        }
        category = cat_map.get(command)
        if not category:
            await update.message.reply_text("Unknown category.")
            return

        pending = await db.get_pending_items()
        filtered = [i for i in pending if i.get("category") == category.value]

        if not filtered:
            await update.message.reply_text(f"No pending {command}.")
            return

        text = digest_builder.build_digest(filtered)
        await update.message.reply_text(text, parse_mode="Markdown")

    @_authorized
    async def _cmd_add(self, update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
        text = update.message.text.replace("/add", "", 1).strip()
        if not text:
            await update.message.reply_text("Usage: /add <description>")
            return

        item = AssistantItem(
            title=text,
            category=ItemCategory.CUSTOM,
            source=ItemSource.MANUAL,
            source_id=f"manual_{datetime.utcnow().timestamp()}",
            status=ItemStatus.DETECTED,
        )
        await db.upsert_item(item)
        await update.message.reply_text(f"✅ Added: *{text}*", parse_mode="Markdown")

    @_authorized
    async def _cmd_scan(self, update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
        await update.message.reply_text("🔄 Starting manual scan...")
        from src.scheduler.jobs import run_scan_cycle
        await run_scan_cycle(manual=True)
        await update.message.reply_text("✅ Scan complete!")

    async def _handle_callback(self, update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
        query = update.callback_query
        await query.answer()

        chat_id = str(query.message.chat.id)
        if chat_id != settings.telegram_chat_id:
            return

        action, item_id = parse_callback(query.data)
        if not item_id:
            return

        tz = ZoneInfo(settings.timezone)
        now = datetime.now(tz)

        if action == ACTION_DONE:
            await db.mark_item_done(item_id, resolved_by="manual")
            await query.edit_message_text(f"✅ Marked as done!")
            await db.update_notification_action(str(query.message.message_id), "mark_done")

        elif action == ACTION_SNOOZE_1D:
            await db.snooze_item(item_id, now + timedelta(days=1))
            await query.edit_message_text(f"💤 Snoozed for 1 day.")
            await db.update_notification_action(str(query.message.message_id), "snooze_1d")

        elif action == ACTION_SNOOZE_3D:
            await db.snooze_item(item_id, now + timedelta(days=3))
            await query.edit_message_text(f"💤 Snoozed for 3 days.")
            await db.update_notification_action(str(query.message.message_id), "snooze_3d")

        elif action == ACTION_SNOOZE_1W:
            await db.snooze_item(item_id, now + timedelta(weeks=1))
            await query.edit_message_text(f"💤 Snoozed for 1 week.")
            await db.update_notification_action(str(query.message.message_id), "snooze_1w")

        elif action == ACTION_DISMISS:
            await db.dismiss_item(item_id)
            await query.edit_message_text(f"🗑️ Dismissed.")
            await db.update_notification_action(str(query.message.message_id), "dismiss")

        elif action == ACTION_DETAILS:
            item = await db.get_item_by_id(item_id)
            if item:
                detail = digest_builder.build_item_detail(item)
                await query.message.reply_text(detail, parse_mode="Markdown")
            else:
                await query.message.reply_text("Item not found.")

    @_authorized
    async def _handle_forwarded(self, update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
        text = update.message.text or update.message.caption or "Forwarded item"

        item = AssistantItem(
            title=text[:100],
            category=ItemCategory.CUSTOM,
            source=ItemSource.MANUAL,
            source_id=f"fwd_{update.message.message_id}_{datetime.utcnow().timestamp()}",
            status=ItemStatus.DETECTED,
            metadata={"forwarded": True, "original_text": text},
        )
        await db.upsert_item(item)
        await update.message.reply_text(
            f"✅ Tracked forwarded item: *{text[:80]}*",
            parse_mode="Markdown",
        )

    @_authorized
    async def _handle_message(self, update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
        text = update.message.text
        if not text:
            return

        lower = text.lower().strip()
        quick_queries = {
            "what": True, "how many": True, "bills": True, "due": True,
            "upcoming": True, "overdue": True, "pending": True, "status": True,
        }

        if any(lower.startswith(q) or q in lower for q in quick_queries):
            try:
                from src.extractor.llm_extractor import LLMExtractor
                llm = LLMExtractor()
                response = await llm.answer_question(text)
                if response:
                    await update.message.reply_text(response, parse_mode="Markdown")
                    return
            except Exception:
                logger.exception("telegram.llm_query_failed")

        await update.message.reply_text(
            "I can help with:\n"
            "/upcoming - upcoming items\n"
            "/overdue - overdue items\n"
            "/bills - filter bills\n"
            "/add <text> - add a manual item\n"
            "/scan - trigger a scan\n"
            "/status - agent health\n\n"
            "Or ask me a question like: _what bills are due this week?_",
            parse_mode="Markdown",
        )


telegram_bot = TelegramBot()
