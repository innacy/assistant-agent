from datetime import datetime, timedelta
from zoneinfo import ZoneInfo

from src.config import settings
from src.storage.models import ItemCategory, ItemStatus


CATEGORY_EMOJI = {
    ItemCategory.BILL.value: "💰",
    ItemCategory.SUBSCRIPTION.value: "🔄",
    ItemCategory.RENEWAL.value: "📋",
    ItemCategory.ASSIGNMENT.value: "📝",
    ItemCategory.APPOINTMENT.value: "📅",
    ItemCategory.FOLLOWUP.value: "📧",
    ItemCategory.CUSTOM.value: "📌",
}


class DigestBuilder:
    def build_digest(self, items: list[dict], auto_verified: list[dict] | None = None) -> str:
        tz = ZoneInfo(settings.timezone)
        now = datetime.now(tz)
        date_str = now.strftime("%b %d, %Y")
        period = "Morning" if now.hour < 14 else "Evening"

        overdue = []
        due_this_week = []
        upcoming = []

        week_end = now + timedelta(days=7)

        for item in items:
            status = item.get("status")
            if status in (ItemStatus.DONE.value, ItemStatus.DISMISSED.value):
                continue

            due = item.get("due_date")
            if due and not due.tzinfo:
                due = due.replace(tzinfo=tz)

            if status == ItemStatus.OVERDUE.value or (due and due < now):
                overdue.append(item)
            elif due and due <= week_end:
                due_this_week.append(item)
            else:
                upcoming.append(item)

        lines = [f"*{period} Digest ({date_str})*\n"]

        if overdue:
            lines.append(f"🚨 *OVERDUE ({len(overdue)})*")
            for item in overdue:
                lines.append(self._format_item(item, tz))
            lines.append("")

        if due_this_week:
            lines.append(f"⏰ *DUE THIS WEEK ({len(due_this_week)})*")
            for item in due_this_week:
                lines.append(self._format_item(item, tz))
            lines.append("")

        if upcoming:
            lines.append(f"📋 *UPCOMING ({len(upcoming)})*")
            for item in upcoming[:10]:
                lines.append(self._format_item(item, tz))
            if len(upcoming) > 10:
                lines.append(f"  _...and {len(upcoming) - 10} more_")
            lines.append("")

        if auto_verified:
            lines.append(f"✅ *AUTO-VERIFIED*")
            for item in auto_verified:
                lines.append(f"  ✓ {item.get('title', 'Unknown')} _(confirmed via email)_")
            lines.append("")

        if not overdue and not due_this_week and not upcoming:
            lines.append("🎉 _All clear! Nothing pending._")

        return "\n".join(lines)

    def _format_item(self, item: dict, tz: ZoneInfo) -> str:
        emoji = CATEGORY_EMOJI.get(item.get("category", ""), "📌")
        title = item.get("title", "Unknown")
        amount = item.get("amount")
        currency = item.get("currency", "")
        due = item.get("due_date")

        parts = [f"  {emoji} {title}"]
        if amount:
            parts.append(f" - {currency} {amount:,.2f}")
        if due:
            if not due.tzinfo:
                due = due.replace(tzinfo=tz)
            parts.append(f" _(due {due.strftime('%b %d')})_")

        return "".join(parts)

    def build_item_detail(self, item: dict) -> str:
        tz = ZoneInfo(settings.timezone)
        lines = [f"*{item.get('title', 'Unknown')}*\n"]

        fields = [
            ("Category", item.get("category", "").title()),
            ("Status", item.get("status", "").replace("_", " ").title()),
            ("Source", item.get("source", "").title()),
        ]

        if item.get("amount"):
            fields.append(("Amount", f"{item.get('currency', '')} {item['amount']:,.2f}"))

        due = item.get("due_date")
        if due:
            if not due.tzinfo:
                due = due.replace(tzinfo=tz)
            fields.append(("Due", due.strftime("%b %d, %Y %I:%M %p")))

        fields.append(("Reminders sent", str(item.get("reminders_sent", 0))))

        created = item.get("created_at")
        if created:
            if not created.tzinfo:
                created = created.replace(tzinfo=tz)
            fields.append(("Tracked since", created.strftime("%b %d, %Y")))

        verification = item.get("verification", {})
        if verification.get("strategy") == "email_confirmation":
            if verification.get("confirmed_at"):
                fields.append(("Verification", "Confirmed via email ✅"))
            else:
                fields.append(("Verification", "Awaiting email confirmation"))

        for label, value in fields:
            lines.append(f"*{label}:* {value}")

        return "\n".join(lines)

    def build_status_message(self, last_scan: dict | None, item_counts: dict) -> str:
        lines = ["*🤖 Agent Status*\n"]

        if last_scan:
            tz = ZoneInfo(settings.timezone)
            started = last_scan.get("started_at")
            if started:
                if not started.tzinfo:
                    started = started.replace(tzinfo=tz)
                lines.append(f"*Last scan:* {started.strftime('%b %d, %I:%M %p')}")
            lines.append(f"*Emails processed:* {last_scan.get('emails_processed', 0)}")
            lines.append(f"*Events processed:* {last_scan.get('events_processed', 0)}")
            lines.append(f"*Items found:* {last_scan.get('items_found', 0)}")
            errors = last_scan.get("errors", [])
            if errors:
                lines.append(f"*Errors:* {len(errors)}")
        else:
            lines.append("_No scans recorded yet._")

        lines.append("")
        lines.append("*Active items:*")
        for status, count in item_counts.items():
            lines.append(f"  {status}: {count}")

        return "\n".join(lines)
