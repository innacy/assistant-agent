import asyncio
import signal
import sys

import structlog
from apscheduler.schedulers.asyncio import AsyncIOScheduler
from apscheduler.triggers.cron import CronTrigger

from src.config import settings
from src.notifier.telegram_bot import telegram_bot
from src.scheduler.jobs import initialize_scanners, run_scan_cycle
from src.storage.mongodb import db

structlog.configure(
    processors=[
        structlog.contextvars.merge_contextvars,
        structlog.processors.add_log_level,
        structlog.processors.StackInfoRenderer(),
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.ExceptionRenderer(),
        structlog.dev.ConsoleRenderer(),
    ],
    wrapper_class=structlog.make_filtering_bound_logger(
        structlog.get_level_from_name(settings.log_level)
    ),
)

logger = structlog.get_logger()


async def scheduled_scan() -> None:
    try:
        await run_scan_cycle()
    except Exception:
        logger.exception("scheduled_scan.failed")


async def startup_scan() -> None:
    """Run an initial scan on startup after a short delay."""
    await asyncio.sleep(5)
    logger.info("startup_scan.running")
    try:
        await run_scan_cycle()
    except Exception:
        logger.exception("startup_scan.failed")


async def main() -> None:
    logger.info(
        "agent.starting",
        timezone=settings.timezone,
        morning=settings.morning_scan_time,
        evening=settings.evening_scan_time,
        llm=settings.llm_provider,
    )

    await db.connect()

    try:
        await initialize_scanners()
    except Exception:
        logger.exception("agent.scanner_init_failed")
        logger.warning("agent.continuing_without_scanners")

    try:
        await telegram_bot.start()
    except Exception:
        logger.exception("agent.telegram_start_failed")
        await db.close()
        sys.exit(1)

    morning_h, morning_m = map(int, settings.morning_scan_time.split(":"))
    evening_h, evening_m = map(int, settings.evening_scan_time.split(":"))

    scheduler = AsyncIOScheduler(timezone=settings.timezone)
    scheduler.add_job(
        scheduled_scan,
        CronTrigger(hour=morning_h, minute=morning_m),
        id="morning_scan",
        name="Morning scan",
        misfire_grace_time=3600,
    )
    scheduler.add_job(
        scheduled_scan,
        CronTrigger(hour=evening_h, minute=evening_m),
        id="evening_scan",
        name="Evening scan",
        misfire_grace_time=3600,
    )
    scheduler.start()
    logger.info("scheduler.started")

    asyncio.create_task(startup_scan())

    stop_event = asyncio.Event()

    loop = asyncio.get_running_loop()
    for sig in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(sig, stop_event.set)

    logger.info("agent.running")

    try:
        await stop_event.wait()
    except asyncio.CancelledError:
        pass

    logger.info("agent.shutting_down")
    scheduler.shutdown(wait=False)
    await telegram_bot.stop()
    await db.close()
    logger.info("agent.stopped")


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        sys.exit(0)
