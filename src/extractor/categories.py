import re
from dataclasses import dataclass, field


@dataclass
class SenderPattern:
    pattern: str
    category: str
    verification_terms: list[str] = field(default_factory=list)


KNOWN_SENDERS: list[SenderPattern] = [
    # Subscriptions
    SenderPattern("netflix", "subscription", ["netflix", "payment", "receipt"]),
    SenderPattern("spotify", "subscription", ["spotify", "payment", "receipt"]),
    SenderPattern("amazon prime", "subscription", ["amazon", "prime", "renewed"]),
    SenderPattern("disney", "subscription", ["disney", "payment"]),
    SenderPattern("youtube premium", "subscription", ["youtube", "payment"]),
    SenderPattern("apple.com", "subscription", ["apple", "receipt"]),
    SenderPattern("google storage", "subscription", ["google", "storage", "payment"]),
    SenderPattern("github", "subscription", ["github", "receipt"]),
    SenderPattern("digitalocean", "subscription", ["digitalocean", "invoice"]),
    SenderPattern("hetzner", "subscription", ["hetzner", "invoice"]),
    SenderPattern("namecheap", "subscription", ["namecheap", "renewal"]),
    SenderPattern("godaddy", "subscription", ["godaddy", "renewal", "domain"]),
    SenderPattern("cloudflare", "subscription", ["cloudflare", "invoice"]),

    # Bills / Utilities
    SenderPattern("electricity", "bill", ["electricity", "payment", "received"]),
    SenderPattern("electric bill", "bill", ["electric", "payment"]),
    SenderPattern("water bill", "bill", ["water", "payment"]),
    SenderPattern("gas bill", "bill", ["gas", "payment"]),
    SenderPattern("internet bill", "bill", ["internet", "broadband", "payment"]),
    SenderPattern("phone bill", "bill", ["phone", "mobile", "payment"]),
    SenderPattern("credit card", "bill", ["credit card", "payment", "statement"]),
    SenderPattern("insurance", "bill", ["insurance", "premium", "payment"]),
    SenderPattern("rent", "bill", ["rent", "payment"]),

    # Renewals
    SenderPattern("passport", "renewal", ["passport", "renewal"]),
    SenderPattern("license", "renewal", ["license", "renewal"]),
    SenderPattern("warranty", "renewal", ["warranty"]),
    SenderPattern("domain renewal", "renewal", ["domain", "renewed"]),
]

BILL_KEYWORDS = [
    "bill", "invoice", "payment due", "due date", "amount due",
    "pay by", "balance due", "statement", "overdue", "past due",
    "reminder to pay", "payment reminder",
]

SUBSCRIPTION_KEYWORDS = [
    "subscription", "renewal", "renew", "auto-renewal",
    "recurring", "membership", "plan", "billing cycle",
    "will be charged", "upcoming charge", "annual renewal",
]

ASSIGNMENT_KEYWORDS = [
    "deadline", "due date", "submit by", "submission",
    "assignment", "homework", "project due", "deliverable",
    "please submit", "turn in", "hand in",
]

AMOUNT_PATTERNS = [
    re.compile(r"(?:Rs\.?|INR|₹)\s*([\d,]+\.?\d*)", re.IGNORECASE),
    re.compile(r"\$\s*([\d,]+\.?\d*)"),
    re.compile(r"([\d,]+\.?\d*)\s*(?:USD|EUR|GBP)", re.IGNORECASE),
    re.compile(r"amount[:\s]*(?:Rs\.?|INR|₹|\$)?\s*([\d,]+\.?\d*)", re.IGNORECASE),
    re.compile(r"total[:\s]*(?:Rs\.?|INR|₹|\$)?\s*([\d,]+\.?\d*)", re.IGNORECASE),
]

DATE_PATTERNS = [
    re.compile(r"due\s+(?:on|by|date)[:\s]*(\d{1,2}[/-]\d{1,2}[/-]\d{2,4})", re.IGNORECASE),
    re.compile(r"(?:pay|submit|renew)\s+(?:by|before)[:\s]*(\d{1,2}[/-]\d{1,2}[/-]\d{2,4})", re.IGNORECASE),
    re.compile(
        r"(?:due|expires?|renews?|deadline)[:\s]*(\d{1,2}\s+(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\w*[\s,]*\d{2,4})",
        re.IGNORECASE,
    ),
    re.compile(
        r"(\d{1,2}\s+(?:January|February|March|April|May|June|July|August|September|October|November|December)[\s,]*\d{4})",
        re.IGNORECASE,
    ),
]
