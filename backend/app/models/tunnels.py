# -*- coding: utf-8 -*-
"""Tunnel model."""

from datetime import datetime

from sqlalchemy import String, Text
from sqlalchemy.orm import Mapped, mapped_column

from app.db.base import Base


class Tunnel(Base):
    __tablename__ = "tunnels"

    id: Mapped[str] = mapped_column(String(36), primary_key=True)
    account_id: Mapped[str] = mapped_column(String(36), nullable=False)
    node_id: Mapped[str | None] = mapped_column(String(36), nullable=True)
    external_id: Mapped[str | None] = mapped_column(String(255), nullable=True)
    name: Mapped[str | None] = mapped_column(String(255), nullable=True)
    canonical_key: Mapped[str | None] = mapped_column(String(255), nullable=True)
    runtime_key: Mapped[str | None] = mapped_column(String(255), nullable=True)
    tunnel_type: Mapped[str | None] = mapped_column(String(20), nullable=True)
    local_ip: Mapped[str | None] = mapped_column(String(45), nullable=True)
    local_port: Mapped[int | None] = mapped_column(nullable=True)
    remote_port: Mapped[int | None] = mapped_column(nullable=True)
    dns_domain_cname: Mapped[str | None] = mapped_column(String(255), nullable=True)
    dns_proxied: Mapped[bool] = mapped_column(default=False, nullable=False)
    desired_state: Mapped[str | None] = mapped_column(String(20), default="active", nullable=True)
    actual_state: Mapped[str | None] = mapped_column(String(20), nullable=True)
    state_reason: Mapped[str | None] = mapped_column(Text, nullable=True)
    desired_hash: Mapped[str | None] = mapped_column(String(64), nullable=True)
    observed_hash: Mapped[str | None] = mapped_column(String(64), nullable=True)
    last_applied_snapshot_id: Mapped[str | None] = mapped_column(String(36), nullable=True)
    last_applied_at: Mapped[datetime | None] = mapped_column(nullable=True)
    last_error_code: Mapped[str | None] = mapped_column(String(50), nullable=True)
    last_error_message: Mapped[str | None] = mapped_column(Text, nullable=True)
    manual_override_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    created_at: Mapped[datetime] = mapped_column(default=datetime.utcnow)
    updated_at: Mapped[datetime] = mapped_column(default=datetime.utcnow, onupdate=datetime.utcnow)
    archived_at: Mapped[datetime | None] = mapped_column(nullable=True)