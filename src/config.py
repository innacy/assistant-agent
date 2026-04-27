from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    mongo_uri: str = "mongodb://localhost:27017"
    database_name: str = "idata-dev"

    telegram_bot_token: str = ""
    telegram_chat_id: str = ""

    google_credentials_path: str = "data/credentials.json"
    google_token_path: str = "data/token.json"

    timezone: str = "Asia/Kolkata"
    morning_scan_time: str = "07:00"
    evening_scan_time: str = "19:00"

    llm_provider: str = "ollama"
    llm_model: str = "llama3.1:8b"
    ollama_host: str = "http://ollama:11434"
    cloud_llm_fallback: bool = False
    openai_api_key: str = ""

    log_level: str = "INFO"

    model_config = {"env_file": ".env", "env_file_encoding": "utf-8"}


settings = Settings()
