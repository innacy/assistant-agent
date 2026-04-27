from datetime import datetime, timedelta, timezone
from typing import Optional

import structlog
from googleapiclient.discovery import build

from src.scanner.base import BaseScanner, RawItem
from src.scanner.google_auth import get_google_credentials
from src.storage.mongodb import db

logger = structlog.get_logger()


class CalendarScanner(BaseScanner):
    def __init__(self) -> None:
        self._service = None

    async def authenticate(self) -> None:
        creds = get_google_credentials()
        if not creds:
            return
        self._service = build("calendar", "v3", credentials=creds)
        logger.info("calendar.authenticated")

    async def scan(self) -> list[RawItem]:
        if not self._service:
            logger.warning("calendar.not_authenticated")
            return []

        items: list[RawItem] = []
        now = datetime.now(timezone.utc)
        time_min = now.isoformat()
        time_max = (now + timedelta(days=30)).isoformat()

        try:
            events_result = (
                self._service.events()
                .list(
                    calendarId="primary",
                    timeMin=time_min,
                    timeMax=time_max,
                    maxResults=100,
                    singleEvents=True,
                    orderBy="startTime",
                )
                .execute()
            )
            events = events_result.get("items", [])
            logger.info("calendar.fetched", count=len(events))

            for event in events:
                try:
                    existing = await db.get_item_by_source_id(f"cal_{event.get('id', '')}")
                    if existing:
                        continue
                    raw_item = self._parse_event(event)
                    if raw_item:
                        items.append(raw_item)
                except Exception:
                    logger.exception("calendar.parse_failed", event_id=event.get("id"))

        except Exception:
            logger.exception("calendar.scan_failed")

        return items

    def _parse_event(self, event: dict) -> Optional[RawItem]:
        summary = event.get("summary", "")
        if not summary:
            return None

        description = event.get("description", "")
        event_id = event.get("id", "")

        start = event.get("start", {})
        start_dt = None
        if "dateTime" in start:
            try:
                start_dt = datetime.fromisoformat(start["dateTime"])
            except ValueError:
                pass
        elif "date" in start:
            try:
                start_dt = datetime.strptime(start["date"], "%Y-%m-%d")
            except ValueError:
                pass

        location = event.get("location", "")
        organizer = event.get("organizer", {}).get("email", "")
        attendees = [a.get("email", "") for a in event.get("attendees", [])]

        return RawItem(
            source="calendar",
            source_id=f"cal_{event_id}",
            subject=summary,
            body=description,
            sender=organizer,
            date=start_dt,
            metadata={
                "calendar_id": event_id,
                "location": location,
                "attendees": attendees,
                "all_day": "date" in start and "dateTime" not in start,
            },
        )
