# -*- coding: utf-8 -*-
"""SyncState model."""

from datetime import datetime

from sqlalchemy import Index, String, Text
from sqlalchemy.orm import Mapped, mapped_column

from app.db.base import Base


class SyncState(Base):
    __tablename__ = "sync_state"

    id: Mapped[str] = mapped_column(String(36), primary_key=True)
    account_id: Mapped[str] = mapped_column(String(36), nullable=False)
    subject_type: Mapped[str] = mapped_column(String(50), nullable=False)
    subject_id: Mapped[str] = mapped_column(String(36), nullable=False)
    desired_hash: Mapped[str | None] = mapped_column(String(64), nullable=True)
    observed_hash: Mapped[str | None] = mapped_column(String(64), nullable=True)
    last_snapshot_id: Mapped[str | None] = mapped_column(String(36), nullable=True)
    last_job_id: Mapped[str | None] = mapped_column(String(36), nullable=True)
    status: Mapped[str] = mapped_column(String(20), default="pending", nullable=False)
    conflict_reason: Mapped[str | None] = mapped_column(Text, nullable=True)
    dirty: Mapped[bool] = mapped_column(default=False, nullable=False)
    retry_count: Mapped[int] = mapped_column(default=0, nullable=False)
    next_retry_at: Mapped[datetime | None] = mapped_column(nullable=True)
    locked_until: Mapped[datetime | None] = mapped_column(nullable=True)
    last_success_at: Mapped[datetime | None] = mapped_column(nullable=True)
    last_attempt_at: Mapped[datetime | None] = mapped_column(nullable=True)
    last_error_code: Mapped[str | None] = mapped_column(String(50), nullable=True)
    last_error_message: Mapped[str | None] = mapped_column(Text, nullable=True)
    manual_override_at: Mapped[datetime | None] = mapped_column(nullable=True)
    manual_override_by: Mapped[str | None] = mapped_column(String(36), nullable=True)
    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    updated_at: Mapped[datetime] = mapped_column(default=datetime.utcnow, onupdate=datetime.utcnow)

    __table_args__ = (
        Index("ix_sync_state_account_status_retry", "account_id", "status", "next_retry_at"),
        Index("ix_sync_state_subject", "subject_type", "subject_id"),
    )