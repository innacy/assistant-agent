import json
from typing import Optional

import structlog

from src.config import settings
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

EXTRACTION_PROMPT = """Analyze the following email and determine if it contains an actionable item that needs tracking (bill, subscription renewal, assignment deadline, follow-up needed, document renewal, etc.).

If it IS actionable, respond with JSON:
{{
  "actionable": true,
  "title": "short descriptive title",
  "category": "bill|subscription|renewal|assignment|appointment|followup",
  "amount": null or number,
  "currency": null or "INR"|"USD"|"EUR",
  "due_date": null or "YYYY-MM-DD",
  "verification_terms": ["term1", "term2"]
}}

If it is NOT actionable (newsletters, promotions, social notifications, etc.), respond with:
{{"actionable": false}}

Email:
From: {sender}
Subject: {subject}
Date: {date}
Body (truncated):
{body}

Respond with ONLY the JSON, no other text."""


class LLMExtractor:
    async def extract(self, raw: RawItem) -> Optional[AssistantItem]:
        prompt = EXTRACTION_PROMPT.format(
            sender=raw.sender,
            subject=raw.subject,
            date=raw.date.isoformat() if raw.date else "unknown",
            body=raw.body[:2000],
        )

        result = await self._call_llm(prompt)
        if not result:
            return None

        try:
            data = json.loads(result)
        except json.JSONDecodeError:
            cleaned = result.strip()
            start = cleaned.find("{")
            end = cleaned.rfind("}") + 1
            if start >= 0 and end > start:
                try:
                    data = json.loads(cleaned[start:end])
                except json.JSONDecodeError:
                    logger.warning("llm.json_parse_failed", raw=result[:200])
                    return None
            else:
                return None

        if not data.get("actionable"):
            return None

        try:
            category = ItemCategory(data.get("category", "custom"))
        except ValueError:
            category = ItemCategory.CUSTOM

        due_date = None
        if data.get("due_date"):
            try:
                from dateutil import parser as dateparser
                due_date = dateparser.parse(data["due_date"])
            except (ValueError, TypeError):
                pass

        source = ItemSource.CALENDAR if raw.source == "calendar" else ItemSource.GMAIL

        strategy = VerificationStrategy.EMAIL_CONFIRMATION
        if category in (ItemCategory.ASSIGNMENT, ItemCategory.FOLLOWUP, ItemCategory.APPOINTMENT):
            strategy = VerificationStrategy.MANUAL

        return AssistantItem(
            title=data.get("title", raw.subject[:80]),
            category=category,
            amount=data.get("amount"),
            currency=data.get("currency"),
            due_date=due_date,
            source=source,
            source_id=raw.source_id,
            status=ItemStatus.DETECTED,
            verification=Verification(
                strategy=strategy,
                search_terms=data.get("verification_terms", []),
            ),
        )

    async def _call_llm(self, prompt: str) -> Optional[str]:
        provider = settings.llm_provider

        if provider == "ollama":
            return await self._call_ollama(prompt)
        elif provider == "openai" and settings.cloud_llm_fallback:
            return await self._call_openai(prompt)
        else:
            return await self._call_ollama(prompt)

    async def _call_ollama(self, prompt: str) -> Optional[str]:
        try:
            import ollama as ollama_lib

            client = ollama_lib.AsyncClient(host=settings.ollama_host)
            response = await client.chat(
                model=settings.llm_model,
                messages=[{"role": "user", "content": prompt}],
                options={"temperature": 0.1},
            )
            return response["message"]["content"]
        except Exception:
            logger.exception("ollama.call_failed")
            if settings.cloud_llm_fallback and settings.openai_api_key:
                logger.info("llm.falling_back_to_openai")
                return await self._call_openai(prompt)
            return None

    async def answer_question(self, question: str) -> Optional[str]:
        """Answer a free-text question about tracked items."""
        from src.storage.mongodb import db

        pending = await db.get_pending_items()
        if not pending:
            return "You have no pending items right now. 🎉"

        items_summary = "\n".join(
            f"- {i.get('title', '?')} | {i.get('category', '?')} | "
            f"due: {i.get('due_date', 'no date')} | status: {i.get('status', '?')}"
            for i in pending[:20]
        )

        prompt = (
            f"You are a personal life-admin assistant. The user has these pending items:\n\n"
            f"{items_summary}\n\n"
            f"User question: {question}\n\n"
            f"Answer concisely and helpfully. Use markdown formatting."
        )

        return await self._call_llm(prompt)

    async def _call_openai(self, prompt: str) -> Optional[str]:
        try:
            from openai import AsyncOpenAI

            client = AsyncOpenAI(api_key=settings.openai_api_key)
            response = await client.chat.completions.create(
                model="gpt-4o-mini",
                messages=[{"role": "user", "content": prompt}],
                temperature=0.1,
                max_tokens=500,
            )
            return response.choices[0].message.content
        except Exception:
            logger.exception("openai.call_failed")
            return None
