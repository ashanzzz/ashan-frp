# -*- coding: utf-8 -*-
"""Snapshot model."""

from datetime import datetime

from sqlalchemy import String, Text
from sqlalchemy.orm import Mapped, mapped_column

from app.db.base import Base


class Snapshot(Base):
    __tablename__ = "snapshots"

    id: Mapped[str] = mapped_column(String(36), primary_key=True)
    account_id: Mapped[str | None] = mapped_column(String(36), nullable=True)
    source_system: Mapped[str] = mapped_column(String(50), nullable=False)
    source_ref: Mapped[str | None] = mapped_column(String(255), nullable=True)
    subject_type: Mapped[str | None] = mapped_column(String(50), nullable=True)
    subject_id: Mapped[str | None] = mapped_column(String(36), nullable=True)
    snapshot_kind: Mapped[str | None] = mapped_column(String(50), nullable=True)
    content_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    content_hash: Mapped[str | None] = mapped_column(String(64), nullable=True)
    captured_at: Mapped[datetime] = mapped_column(default=datetime.utcnow)
    captured_by_job_id: Mapped[str | None] = mapped_column(String(36), nullable=True)
    retention_class: Mapped[str | None] = mapped_column(String(20), default="standard", nullable=True)
    expires_at: Mapped[datetime | None] = mapped_column(nullable=True)
    created_at: Mapped[datetime] = mapped_column(default=datetime.utcnow)
