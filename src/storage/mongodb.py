from datetime import datetime
from typing import Optional

import structlog
from bson import ObjectId
from motor.motor_asyncio import AsyncIOMotorClient, AsyncIOMotorDatabase

from src.config import settings
from src.storage.models import (
    AssistantConfig,
    AssistantItem,
    ItemStatus,
    NotificationRecord,
    ScanRecord,
)

logger = structlog.get_logger()

ITEMS = "assistant_items"
SCANS = "assistant_scans"
NOTIFICATIONS = "assistant_notifications"
CONFIG = "assistant_config"


class MongoDB:
    def __init__(self) -> None:
        self._client: Optional[AsyncIOMotorClient] = None
        self._db: Optional[AsyncIOMotorDatabase] = None

    async def connect(self, max_retries: int = 5) -> None:
        import asyncio

        for attempt in range(1, max_retries + 1):
            try:
                self._client = AsyncIOMotorClient(
                    settings.mongo_uri,
                    serverSelectionTimeoutMS=5000,
                )
                await self._client.admin.command("ping")
                self._db = self._client[settings.database_name]
                await self._ensure_indexes()
                logger.info("mongodb.connected", uri=settings.mongo_uri, db=settings.database_name)
                return
            except Exception:
                logger.warning(
                    "mongodb.connect_retry",
                    attempt=attempt,
                    max_retries=max_retries,
                )
                if attempt == max_retries:
                    raise
                await asyncio.sleep(2 ** attempt)

    async def close(self) -> None:
        if self._client:
            self._client.close()
            logger.info("mongodb.disconnected")

    @property
    def db(self) -> AsyncIOMotorDatabase:
        assert self._db is not None, "Call connect() first"
        return self._db

    async def _ensure_indexes(self) -> None:
        await self.db[ITEMS].create_index("source_id", unique=True)
        await self.db[ITEMS].create_index("status")
        await self.db[ITEMS].create_index("due_date")
        await self.db[ITEMS].create_index("category")
        await self.db[SCANS].create_index("started_at")
        await self.db[NOTIFICATIONS].create_index("item_id")
        await self.db[NOTIFICATIONS].create_index("sent_at")
        logger.info("mongodb.indexes_ensured")

    # --- Items ---

    async def upsert_item(self, item: AssistantItem) -> str:
        doc = item.model_dump()
        doc["updated_at"] = datetime.utcnow()
        result = await self.db[ITEMS].update_one(
            {"source_id": item.source_id},
            {"$set": doc, "$setOnInsert": {"created_at": datetime.utcnow()}},
            upsert=True,
        )
        item_id = str(result.upserted_id) if result.upserted_id else None
        if result.upserted_id:
            logger.info("item.created", title=item.title, source_id=item.source_id)
        return item_id

    async def get_item_by_id(self, item_id: str) -> Optional[dict]:
        return await self.db[ITEMS].find_one({"_id": ObjectId(item_id)})

    async def get_item_by_source_id(self, source_id: str) -> Optional[dict]:
        return await self.db[ITEMS].find_one({"source_id": source_id})

    async def get_items_by_status(self, *statuses: ItemStatus) -> list[dict]:
        cursor = self.db[ITEMS].find(
            {"status": {"$in": [s.value for s in statuses]}}
        ).sort("due_date", 1)
        return await cursor.to_list(length=500)

    async def get_pending_items(self) -> list[dict]:
        return await self.get_items_by_status(
            ItemStatus.DETECTED,
            ItemStatus.UPCOMING,
            ItemStatus.REMINDED,
            ItemStatus.OVERDUE,
            ItemStatus.SNOOZED,
        )

    async def update_item_status(
        self, item_id: str, status: ItemStatus, **extra_fields
    ) -> None:
        update = {"$set": {"status": status.value, "updated_at": datetime.utcnow()}}
        update["$set"].update(extra_fields)
        await self.db[ITEMS].update_one({"_id": ObjectId(item_id)}, update)
        logger.info("item.status_updated", item_id=item_id, status=status.value)

    async def mark_item_done(
        self, item_id: str, resolved_by: str, confirmation_source_id: Optional[str] = None
    ) -> None:
        now = datetime.utcnow()
        update: dict = {
            "status": ItemStatus.DONE.value,
            "resolved_at": now,
            "resolved_by": resolved_by,
            "updated_at": now,
        }
        if confirmation_source_id:
            update["verification.confirmed_at"] = now
            update["verification.confirmation_source_id"] = confirmation_source_id
        await self.db[ITEMS].update_one({"_id": ObjectId(item_id)}, {"$set": update})
        logger.info("item.done", item_id=item_id, resolved_by=resolved_by)

    async def snooze_item(self, item_id: str, until: datetime) -> None:
        await self.db[ITEMS].update_one(
            {"_id": ObjectId(item_id)},
            {
                "$set": {
                    "status": ItemStatus.SNOOZED.value,
                    "snoozed_until": until,
                    "updated_at": datetime.utcnow(),
                }
            },
        )
        logger.info("item.snoozed", item_id=item_id, until=until.isoformat())

    async def dismiss_item(self, item_id: str) -> None:
        await self.mark_item_done(item_id, resolved_by="dismissed")

    async def increment_reminder(self, item_id: str) -> None:
        now = datetime.utcnow()
        await self.db[ITEMS].update_one(
            {"_id": ObjectId(item_id)},
            {
                "$inc": {"reminders_sent": 1},
                "$set": {"last_reminded_at": now, "updated_at": now},
            },
        )

    async def unsnooze_expired(self) -> int:
        now = datetime.utcnow()
        result = await self.db[ITEMS].update_many(
            {"status": ItemStatus.SNOOZED.value, "snoozed_until": {"$lte": now}},
            {
                "$set": {
                    "status": ItemStatus.REMINDED.value,
                    "snoozed_until": None,
                    "updated_at": now,
                }
            },
        )
        if result.modified_count:
            logger.info("items.unsnoozed", count=result.modified_count)
        return result.modified_count

    # --- Scans ---

    async def record_scan(self, scan: ScanRecord) -> str:
        doc = scan.model_dump()
        result = await self.db[SCANS].insert_one(doc)
        return str(result.inserted_id)

    async def get_last_scan(self) -> Optional[dict]:
        return await self.db[SCANS].find_one(sort=[("started_at", -1)])

    # --- Notifications ---

    async def record_notification(self, notification: NotificationRecord) -> str:
        doc = notification.model_dump()
        result = await self.db[NOTIFICATIONS].insert_one(doc)
        return str(result.inserted_id)

    async def update_notification_action(
        self, message_id: str, action: str
    ) -> None:
        await self.db[NOTIFICATIONS].update_one(
            {"message_id": message_id},
            {"$set": {"user_action": action, "acted_at": datetime.utcnow()}},
        )

    # --- Config ---

    async def get_config(self) -> AssistantConfig:
        doc = await self.db[CONFIG].find_one()
        if doc:
            doc.pop("_id", None)
            return AssistantConfig(**doc)
        cfg = AssistantConfig(telegram_chat_id=settings.telegram_chat_id)
        await self.db[CONFIG].insert_one(cfg.model_dump())
        return cfg

    async def update_config(self, **fields) -> None:
        await self.db[CONFIG].update_one({}, {"$set": fields}, upsert=True)


db = MongoDB()
