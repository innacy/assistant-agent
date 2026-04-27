import re
from datetime import datetime
from typing import Optional

import structlog
from dateutil import parser as dateparser

from src.extractor.categories import (
    AMOUNT_PATTERNS,
    ASSIGNMENT_KEYWORDS,
    BILL_KEYWORDS,
    DATE_PATTERNS,
    KNOWN_SENDERS,
    SUBSCRIPTION_KEYWORDS,
)
from src.scanner.base import RawItem
from src.storage.models import (
    AssistantItem,
    ItemCategory,
    ItemSource,
    ItemStatus,
    Verification,
    VerificationStrategy,
)

logger = structlog.get_logger()


class RuleEngine:
    def extract(self, raw: RawItem) -> Optional[AssistantItem]:
        text = f"{raw.subject} {raw.body}".lower()
        sender_lower = raw.sender.lower()

        if raw.source == "calendar":
            category = self._classify_calendar_event(raw)
            verification_terms = []
        else:
            sender_match = self._match_sender(sender_lower)
            if sender_match:
                category = ItemCategory(sender_match.category)
                verification_terms = sender_match.verification_terms
            else:
                category = self._classify_by_keywords(text)
                if category is None:
                    return None
                verification_terms = self._build_verification_terms(raw)

        amount = self._extract_amount(text)
        currency = self._detect_currency(text)
        due_date = self._extract_date(text, raw.date)

        source = ItemSource.CALENDAR if raw.source == "calendar" else ItemSource.GMAIL

        title = self._build_title(raw, category)

        strategy = VerificationStrategy.NONE
        if source == ItemSource.GMAIL and category in (
            ItemCategory.BILL,
            ItemCategory.SUBSCRIPTION,
        ):
            strategy = VerificationStrategy.EMAIL_CONFIRMATION
        elif category in (ItemCategory.ASSIGNMENT, ItemCategory.FOLLOWUP):
            strategy = VerificationStrategy.MANUAL

        return AssistantItem(
            title=title,
            category=category,
            amount=amount,
            currency=currency,
            due_date=due_date,
            source=source,
            source_id=raw.source_id,
            status=ItemStatus.DETECTED,
            verification=Verification(
                strategy=strategy,
                search_terms=verification_terms,
            ),
        )

    def _match_sender(self, sender: str):
        for sp in KNOWN_SENDERS:
            if sp.pattern in sender:
                return sp
        return None

    def _classify_by_keywords(self, text: str) -> Optional[ItemCategory]:
        bill_score = sum(1 for kw in BILL_KEYWORDS if kw in text)
        sub_score = sum(1 for kw in SUBSCRIPTION_KEYWORDS if kw in text)
        assign_score = sum(1 for kw in ASSIGNMENT_KEYWORDS if kw in text)

        scores = {
            ItemCategory.BILL: bill_score,
            ItemCategory.SUBSCRIPTION: sub_score,
            ItemCategory.ASSIGNMENT: assign_score,
        }

        best = max(scores, key=scores.get)
        if scores[best] >= 2:
            return best
        if scores[best] == 1:
            return best
        return None

    def _classify_calendar_event(self, raw: RawItem) -> ItemCategory:
        text = f"{raw.subject} {raw.body}".lower()
        if any(kw in text for kw in ["assignment", "deadline", "submit", "due"]):
            return ItemCategory.ASSIGNMENT
        if any(kw in text for kw in ["bill", "pay", "payment"]):
            return ItemCategory.BILL
        return ItemCategory.APPOINTMENT

    def _extract_amount(self, text: str) -> Optional[float]:
        for pattern in AMOUNT_PATTERNS:
            match = pattern.search(text)
            if match:
                try:
                    return float(match.group(1).replace(",", ""))
                except ValueError:
                    continue
        return None

    def _detect_currency(self, text: str) -> Optional[str]:
        if re.search(r"(?:Rs\.?|INR|₹)", text, re.IGNORECASE):
            return "INR"
        if "$" in text or "USD" in text.upper():
            return "USD"
        if "EUR" in text.upper() or "€" in text:
            return "EUR"
        return None

    def _extract_date(self, text: str, fallback: Optional[datetime]) -> Optional[datetime]:
        for pattern in DATE_PATTERNS:
            match = pattern.search(text)
            if match:
                try:
                    return dateparser.parse(match.group(1), fuzzy=True)
                except (ValueError, TypeError):
                    continue
        return fallback

    def _build_title(self, raw: RawItem, category: ItemCategory) -> str:
        subject = raw.subject.strip()
        if len(subject) > 80:
            subject = subject[:77] + "..."
        if not subject:
            subject = f"{category.value.title()} item"

        prefix_map = {
            ItemCategory.BILL: "Bill",
            ItemCategory.SUBSCRIPTION: "Subscription",
            ItemCategory.RENEWAL: "Renewal",
            ItemCategory.ASSIGNMENT: "Assignment",
            ItemCategory.APPOINTMENT: "Appointment",
            ItemCategory.FOLLOWUP: "Follow-up",
            ItemCategory.CUSTOM: "Item",
        }
        prefix = prefix_map.get(category, "Item")

        if prefix.lower() not in subject.lower():
            return f"{prefix}: {subject}"
        return subject

    def _build_verification_terms(self, raw: RawItem) -> list[str]:
        terms = []
        sender = raw.sender.lower()
        at_pos = sender.find("@")
        if at_pos > 0:
            domain = sender[at_pos + 1 :].split(">")[0].strip()
            name_part = domain.split(".")[0]
            if name_part and len(name_part) > 2:
                terms.append(name_part)

        words = raw.subject.lower().split()
        for w in words[:5]:
            clean = re.sub(r"[^a-z0-9]", "", w)
            if len(clean) > 3 and clean not in ("your", "this", "from", "that", "with"):
                terms.append(clean)
                if len(terms) >= 3:
                    break

        return terms
