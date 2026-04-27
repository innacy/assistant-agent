from datetime import datetime, timedelta

import structlog

from src.scanner.gmail_scanner import GmailScanner
from src.storage.models import ItemStatus, VerificationStrategy
from src.storage.mongodb import db

logger = structlog.get_logger()

MAX_VERIFY_PER_SCAN = 10


class EmailVerifier:
    def __init__(self, gmail_scanner: GmailScanner) -> None:
        self._gmail = gmail_scanner

    async def verify_pending_items(self) -> list[dict]:
        """Check pending items for email confirmations. Returns auto-verified items."""
        items = await db.get_items_by_status(
            ItemStatus.REMINDED, ItemStatus.OVERDUE, ItemStatus.UPCOMING
        )

        candidates = [
            i for i in items
            if i.get("verification", {}).get("strategy")
            == VerificationStrategy.EMAIL_CONFIRMATION.value
            and i.get("verification", {}).get("search_terms")
        ]

        candidates.sort(key=lambda i: i.get("due_date") or datetime.max)

        auto_verified = []

        for item in candidates[:MAX_VERIFY_PER_SCAN]:
            try:
                confirmed = await self._verify_single(item)
                if confirmed:
                    auto_verified.append(item)
            except Exception:
                logger.exception(
                    "verifier.item_failed",
                    item_id=str(item["_id"]),
                    title=item.get("title"),
                )

        return auto_verified

    async def _verify_single(self, item: dict) -> bool:
        verification = item.get("verification", {})
        search_terms = verification.get("search_terms", [])
        item_id = str(item["_id"])

        search_after = item.get("created_at", datetime.utcnow() - timedelta(days=30))

        confirmations = await self._gmail.search_confirmations(search_terms, search_after)

        if not confirmations:
            return False

        item_source_id = item.get("source_id", "")
        filtered = [
            c for c in confirmations
            if c.get("id") != item_source_id.replace("gmail_", "")
        ]

        if not filtered:
            return False

        conf = filtered[0]
        await db.mark_item_done(
            item_id,
            resolved_by="auto_verified",
            confirmation_source_id=conf.get("id"),
        )
        logger.info(
            "verifier.auto_verified",
            item_id=item_id,
            title=item.get("title"),
            confirmation_subject=conf.get("subject"),
        )
        return True
