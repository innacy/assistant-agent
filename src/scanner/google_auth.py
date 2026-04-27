import os
from typing import Optional

import structlog
from google.auth.transport.requests import Request
from google.oauth2.credentials import Credentials
from google_auth_oauthlib.flow import InstalledAppFlow

from src.config import settings

logger = structlog.get_logger()

ALL_SCOPES = [
    "https://www.googleapis.com/auth/gmail.readonly",
    "https://www.googleapis.com/auth/calendar.readonly",
]

_cached_creds: Optional[Credentials] = None


def get_google_credentials() -> Optional[Credentials]:
    """Get or refresh Google OAuth2 credentials (shared across Gmail + Calendar)."""
    global _cached_creds

    if _cached_creds and _cached_creds.valid:
        return _cached_creds

    token_path = settings.google_token_path
    creds_path = settings.google_credentials_path
    creds = None

    if os.path.exists(token_path):
        creds = Credentials.from_authorized_user_file(token_path, ALL_SCOPES)

    if not creds or not creds.valid:
        if creds and creds.expired and creds.refresh_token:
            creds.refresh(Request())
            logger.info("google_auth.refreshed")
        else:
            if not os.path.exists(creds_path):
                logger.error(
                    "google_auth.no_credentials",
                    path=creds_path,
                    hint="Download credentials.json from Google Cloud Console",
                )
                return None
            flow = InstalledAppFlow.from_client_secrets_file(creds_path, ALL_SCOPES)
            creds = flow.run_local_server(port=0)
            logger.info("google_auth.new_consent_completed")

        with open(token_path, "w") as f:
            f.write(creds.to_json())

    _cached_creds = creds
    return creds
