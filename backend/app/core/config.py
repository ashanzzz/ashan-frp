# -*- coding: utf-8 -*-
"""Pydantic settings for the backend."""

from pathlib import Path

from pydantic_settings import BaseSettings, SettingsConfigDict

PROJECT_ROOT = Path(__file__).resolve().parent.parent


class Settings(BaseSettings):
    """Application settings, sourced from environment variables."""

    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="allow")

    DATABASE_URL: str = f"sqlite:///{PROJECT_ROOT.parent}/data/ashan_frp.db"
    SECRET_KEY: str = "dev-secret-change-me"
    ACCESS_TOKEN_EXPIRE_MINUTES: int = 60
    ALLOWED_ORIGINS: list[str] = ["http://localhost:3000", "http://localhost:5173"]

    JOB_RUNNER_INTERVAL: int = 5
    JOB_LEASE_TIMEOUT: int = 45
    JOB_HEARTBEAT_INTERVAL: int = 15
    JOB_RETRY_BASE_SECONDS: int = 5
    JOB_RETRY_MAX_SECONDS: int = 300
    JOB_MAX_ATTEMPTS: int = 5

    ONEPANEL_BASE_URL: str = "http://localhost:8080"
    ONEPANEL_API_KEY: str = ""

    FRPC_ENABLED: bool = True
    FRPC_BINARY_PATH: str = "/usr/bin/frpc"
    FRPC_WORK_DIR: str = str(PROJECT_ROOT.parent / "data" / "frpc")
    FRPC_LOG_LEVEL: str = "info"
    FRPC_HEALTHCHECK_INTERVAL: int = 60
    FRPC_RESTART_BACKOFF: int = 5


settings = Settings()
