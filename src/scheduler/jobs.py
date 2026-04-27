import time
from datetime import datetime

import structlog

from src.extractor.llm_extractor import LLMExtractor
from src.extractor.rule_engine import RuleEngine
from src.notifier.telegram_bot import telegram_bot
from src.scanner.calendar_scanner import CalendarScanner
from src.scanner.gmail_scanner import GmailScanner
from src.storage.models import ScanRecord
from src.storage.mongodb import db
from src.verifier.email_verifier import EmailVerifier
from src.verifier.verification import should_remind, update_item_statuses

logger = structlog.get_logger()

gmail_scanner = GmailScanner()
calendar_scanner = CalendarScanner()
rule_engine = RuleEngine()
llm_extractor = LLMExtractor()
email_verifier = EmailVerifier(gmail_scanner)


async def initialize_scanners() -> None:
    await gmail_scanner.authenticate()
    await calendar_scanner.authenticate()
    logger.info("scanners.initialized")


async def run_scan_cycle(manual: bool = False) -> None:
    start = time.monotonic()
    scan = ScanRecord(
        scan_type="manual" if manual else "scheduled",
        sources_scanned=["gmail", "calendar"],
    )
    logger.info("scan.started", type=scan.scan_type)

    try:
        gmail_items = await gmail_scanner.scan()
        scan.emails_processed = len(gmail_items)

        cal_items = await calendar_scanner.scan()
        scan.events_processed = len(cal_items)

        all_raw = gmail_items + cal_items
        items_created = 0

        for raw in all_raw:
            existing = await db.get_item_by_source_id(raw.source_id)
            if existing:
                continue

            try:
                item = rule_engine.extract(raw)
                if not item:
                    item = await llm_extractor.extract(raw)

                if item:
                    await db.upsert_item(item)
                    items_created += 1
            except Exception:
                logger.exception("scan.item_extraction_failed", source_id=raw.source_id)
                scan.errors.append(f"extraction_failed:{raw.source_id}")

        scan.items_found = items_created

        await update_item_statuses()

        auto_verified = await email_verifier.verify_pending_items()
        scan.items_verified = len(auto_verified)

        for verified in auto_verified:
            await telegram_bot.send_verification_alert(verified)

        cfg = await db.get_config()
        pending = await db.get_pending_items()
        digest_items = [i for i in pending if should_remind(i, cfg.reminder_windows)]

        if digest_items or auto_verified:
            await telegram_bot.send_digest(digest_items, auto_verified)

        scan.duration_ms = int((time.monotonic() - start) * 1000)
        scan.completed_at = datetime.utcnow()
        await db.record_scan(scan)

        logger.info(
            "scan.completed",
            items_found=scan.items_found,
            items_verified=scan.items_verified,
            digest_items=len(digest_items),
            emails=scan.emails_processed,
            events=scan.events_processed,
            duration_ms=scan.duration_ms,
        )

    except Exception as e:
        scan.errors.append(str(e))
        scan.duration_ms = int((time.monotonic() - start) * 1000)
        scan.completed_at = datetime.utcnow()
        await db.record_scan(scan)
        logger.exception("scan.failed")
