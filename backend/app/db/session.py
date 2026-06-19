# -*- coding: utf-8 -*-
"""Database session management and initialization."""

import os
from pathlib import Path

from sqlalchemy import create_engine, event
from sqlalchemy.orm import sessionmaker

from app.core.config import settings
from app.db.base import Base

# Ensure the parent directory exists (the .db file's folder)
db_path = settings.DATABASE_URL.replace("sqlite:///", "")
parent_dir = Path(db_path).parent
parent_dir.mkdir(parents=True, exist_ok=True)

engine = create_engine(
    settings.DATABASE_URL,
    connect_args={"check_same_thread": False} if "sqlite" in settings.DATABASE_URL else {},
    echo=False,
)

SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)


@event.listens_for(engine, "connect")
def _set_sqlite_pragma(dbapi_connection, connection_record):  # noqa: ARG001
    """Enable WAL mode and foreign keys for SQLite."""
    if "sqlite" in settings.DATABASE_URL:
        cursor = dbapi_connection.cursor()
        cursor.execute("PRAGMA journal_mode=WAL")
        cursor.execute("PRAGMA foreign_keys=ON")
        cursor.close()


def get_db():
    """Dependency that yields a database session."""
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()


def init_db():
    """Create all tables if they don't exist."""
    # Import all models so they are registered with Base's metadata
    import app.models  # noqa: F401
    Base.metadata.create_all(bind=engine)


def drop_db():
    """Drop all tables. For development/testing only."""
    import app.models  # noqa: F401 (keep unused-import warning silenced)
    Base.metadata.drop_all(bind=engine)