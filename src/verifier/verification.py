from datetime import datetime
from zoneinfo import ZoneInfo

import structlog

from src.config import settings
from src.storage.models import ItemStatus
from src.storage.mongodb import db

logger = structlog.get_logger()


async def update_item_statuses() -> None:
    """Transition item statuses based on due dates, reminder windows, and snooze expiry."""
    tz = ZoneInfo(settings.timezone)
    now = datetime.now(tz)

    unsnoozed = await db.unsnooze_expired()
    if unsnoozed:
        logger.info("verification.unsnoozed", count=unsnoozed)

    cfg = await db.get_config()
    pending = await db.get_pending_items()

    for item in pending:
        item_id = str(item["_id"])
        status = item.get("status")
        due = item.get("due_date")
        category = item.get("category", "")

        if not due:
            continue

        if not due.tzinfo:
            due = due.replace(tzinfo=tz)

        windows = cfg.reminder_windows.get(category, [7, 3, 1, 0])
        earliest_window = max(windows) if windows else 7
        days_until = (due - now).days

        if status == ItemStatus.DETECTED.value:
            if days_until <= earliest_window:
                await db.update_item_status(item_id, ItemStatus.UPCOMING)
                logger.info(
                    "item.now_upcoming",
                    item_id=item_id,
                    title=item.get("title"),
                    days_until=days_until,
                )

        if status in (
            ItemStatus.DETECTED.value,
            ItemStatus.UPCOMING.value,
            ItemStatus.REMINDED.value,
        ):
            if due < now:
                await db.update_item_status(item_id, ItemStatus.OVERDUE)
                logger.info(
                    "item.now_overdue",
                    item_id=item_id,
                    title=item.get("title"),
                    overdue_by_days=abs(days_until),
                )


def should_remind(item: dict, config_windows: dict) -> bool:
    """Determine if an item needs a reminder in this digest cycle."""
    status = item.get("status", "")
    category = item.get("category", "")
    due = item.get("due_date")
    reminders_sent = item.get("reminders_sent", 0)

    if status in (ItemStatus.DONE.value, ItemStatus.DISMISSED.value, ItemStatus.SNOOZED.value):
        return False

    if status == ItemStatus.OVERDUE.value:
        return True

    if not due:
        return status in (ItemStatus.UPCOMING.value, ItemStatus.REMINDED.value)

    tz = ZoneInfo(settings.timezone)
    now = datetime.now(tz)
    if not due.tzinfo:
        due = due.replace(tzinfo=tz)

    days_until = (due.date() - now.date()).days
    windows = config_windows.get(category, [7, 3, 1, 0])

    return any(days_until <= w for w in windows)
