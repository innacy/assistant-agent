import base64
from datetime import datetime
from email.utils import parsedate_to_datetime
from typing import Optional

import structlog
from googleapiclient.discovery import build

from src.scanner.base import BaseScanner, RawItem
from src.scanner.google_auth import get_google_credentials
from src.storage.mongodb import db

logger = structlog.get_logger()


class GmailScanner(BaseScanner):
    def __init__(self) -> None:
        self._service = None

    async def authenticate(self) -> None:
        creds = get_google_credentials()
        if not creds:
            return
        self._service = build("gmail", "v1", credentials=creds)
        logger.info("gmail.authenticated")

    async def scan(self) -> list[RawItem]:
        if not self._service:
            logger.warning("gmail.not_authenticated")
            return []

        config = await db.get_config()
        items: list[RawItem] = []

        try:
            query = "newer_than:1d"
            if config.gmail_last_history_id:
                query = "newer_than:12h"

            results = (
                self._service.users()
                .messages()
                .list(userId="me", q=query, maxResults=50)
                .execute()
            )
            messages = results.get("messages", [])
            logger.info("gmail.fetched", count=len(messages))

            for msg_meta in messages:
                msg_id = msg_meta["id"]

                existing = await db.get_item_by_source_id(f"gmail_{msg_id}")
                if existing:
                    continue

                try:
                    msg = (
                        self._service.users()
                        .messages()
                        .get(userId="me", id=msg_id, format="full")
                        .execute()
                    )
                    raw_item = self._parse_message(msg)
                    if raw_item:
                        items.append(raw_item)
                except Exception:
                    logger.exception("gmail.parse_failed", msg_id=msg_id)

            if messages:
                last_msg = (
                    self._service.users()
                    .messages()
                    .get(userId="me", id=messages[0]["id"], format="metadata")
                    .execute()
                )
                history_id = last_msg.get("historyId")
                if history_id:
                    await db.update_config(gmail_last_history_id=history_id)

        except Exception:
            logger.exception("gmail.scan_failed")

        return items

    async def search_confirmations(
        self, search_terms: list[str], after_date: datetime
    ) -> list[dict]:
        """Search Gmail for confirmation/receipt emails matching given terms."""
        if not self._service:
            return []

        date_str = after_date.strftime("%Y/%m/%d")
        term_query = " OR ".join(search_terms)
        receipt_keywords = "receipt OR confirmation OR payment OR successful"
        query = f"after:{date_str} ({term_query}) ({receipt_keywords})"

        try:
            results = (
                self._service.users()
                .messages()
                .list(userId="me", q=query, maxResults=10)
                .execute()
            )
            confirmations = []
            for msg_meta in results.get("messages", []):
                msg = (
                    self._service.users()
                    .messages()
                    .get(userId="me", id=msg_meta["id"], format="metadata")
                    .execute()
                )
                headers = {
                    h["name"].lower(): h["value"]
                    for h in msg.get("payload", {}).get("headers", [])
                }
                confirmations.append(
                    {
                        "id": msg_meta["id"],
                        "subject": headers.get("subject", ""),
                        "from": headers.get("from", ""),
                        "date": headers.get("date", ""),
                    }
                )
            return confirmations

        except Exception:
            logger.exception("gmail.confirmation_search_failed")
            return []

    def _parse_message(self, msg: dict) -> Optional[RawItem]:
        headers = {
            h["name"].lower(): h["value"]
            for h in msg.get("payload", {}).get("headers", [])
        }

        subject = headers.get("subject", "")
        sender = headers.get("from", "")
        date_str = headers.get("date", "")

        body = self._extract_body(msg.get("payload", {}))

        msg_date = None
        if date_str:
            try:
                msg_date = parsedate_to_datetime(date_str)
            except Exception:
                pass

        return RawItem(
            source="gmail",
            source_id=f"gmail_{msg['id']}",
            subject=subject,
            body=body[:3000],
            sender=sender,
            date=msg_date,
            metadata={"gmail_id": msg["id"], "labels": msg.get("labelIds", [])},
        )

    def _extract_body(self, payload: dict) -> str:
        if payload.get("body", {}).get("data"):
            return base64.urlsafe_b64decode(payload["body"]["data"]).decode(
                "utf-8", errors="replace"
            )

        parts = payload.get("parts", [])
        for part in parts:
            mime = part.get("mimeType", "")
            if mime == "text/plain" and part.get("body", {}).get("data"):
                return base64.urlsafe_b64decode(part["body"]["data"]).decode(
                    "utf-8", errors="replace"
                )

        for part in parts:
            if part.get("parts"):
                result = self._extract_body(part)
                if result:
                    return result

        return ""
