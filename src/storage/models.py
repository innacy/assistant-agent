from datetime import datetime
from enum import Enum
from typing import Optional

from pydantic import BaseModel, Field


class ItemCategory(str, Enum):
    BILL = "bill"
    SUBSCRIPTION = "subscription"
    RENEWAL = "renewal"
    ASSIGNMENT = "assignment"
    APPOINTMENT = "appointment"
    FOLLOWUP = "followup"
    CUSTOM = "custom"


class ItemStatus(str, Enum):
    DETECTED = "detected"
    UPCOMING = "upcoming"
    REMINDED = "reminded"
    OVERDUE = "overdue"
    DONE = "done"
    SNOOZED = "snoozed"
    DISMISSED = "dismissed"


class ItemSource(str, Enum):
    GMAIL = "gmail"
    CALENDAR = "calendar"
    MANUAL = "manual"


class ResolvedBy(str, Enum):
    AUTO_VERIFIED = "auto_verified"
    MANUAL = "manual"
    DISMISSED = "dismissed"


class VerificationStrategy(str, Enum):
    EMAIL_CONFIRMATION = "email_confirmation"
    MANUAL = "manual"
    NONE = "none"


class Verification(BaseModel):
    strategy: VerificationStrategy = VerificationStrategy.EMAIL_CONFIRMATION
    search_terms: list[str] = Field(default_factory=list)
    confirmed_at: Optional[datetime] = None
    confirmation_source_id: Optional[str] = None


class AssistantItem(BaseModel):
    title: str
    category: ItemCategory
    amount: Optional[float] = None
    currency: Optional[str] = None
    due_date: Optional[datetime] = None
    source: ItemSource
    source_id: str
    status: ItemStatus = ItemStatus.DETECTED
    verification: Verification = Field(default_factory=Verification)
    reminders_sent: int = 0
    last_reminded_at: Optional[datetime] = None
    snoozed_until: Optional[datetime] = None
    resolved_at: Optional[datetime] = None
    resolved_by: Optional[ResolvedBy] = None
    metadata: dict = Field(default_factory=dict)
    created_at: datetime = Field(default_factory=datetime.utcnow)
    updated_at: datetime = Field(default_factory=datetime.utcnow)


class ScanRecord(BaseModel):
    scan_type: str = "scheduled"
    sources_scanned: list[str] = Field(default_factory=list)
    items_found: int = 0
    items_verified: int = 0
    emails_processed: int = 0
    events_processed: int = 0
    duration_ms: int = 0
    errors: list[str] = Field(default_factory=list)
    started_at: datetime = Field(default_factory=datetime.utcnow)
    completed_at: Optional[datetime] = None


class NotificationRecord(BaseModel):
    item_id: Optional[str] = None
    channel: str = "telegram"
    type: str = "digest"
    message_id: Optional[str] = None
    status: str = "pending"
    user_action: Optional[str] = None
    sent_at: datetime = Field(default_factory=datetime.utcnow)
    acted_at: Optional[datetime] = None


class AssistantConfig(BaseModel):
    scan_schedule: dict = Field(
        default_factory=lambda: {"morning": "07:00", "evening": "19:00"}
    )
    timezone: str = "Asia/Kolkata"
    reminder_windows: dict = Field(
        default_factory=lambda: {
            "bill": [7, 3, 1, 0],
            "subscription": [7, 3, 1, 0],
            "assignment": [3, 1, 0],
            "appointment": [1],
            "renewal": [30, 7, 1],
            "followup": [2],
        }
    )
    telegram_chat_id: str = ""
    llm_provider: str = "ollama"
    llm_model: str = "llama3.1:8b"
    cloud_llm_fallback: bool = False
    gmail_last_history_id: Optional[str] = None
    calendar_sync_token: Optional[str] = None
