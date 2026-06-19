# -*- coding: utf-8 -*-
"""WebsiteMapping model."""

from datetime import datetime

from sqlalchemy import String, Text
from sqlalchemy.orm import Mapped, mapped_column

from app.db.base import Base


class WebsiteMapping(Base):
    __tablename__ = "website_mappings"

    id: Mapped[str] = mapped_column(String(36), primary_key=True)
    account_id: Mapped[str] = mapped_column(String(36), nullable=False)
    node_id: Mapped[str | None] = mapped_column(String(36), nullable=True)
    tunnel_id: Mapped[str | None] = mapped_column(String(36), nullable=True)
    source_kind: Mapped[str | None] = mapped_column(String(50), nullable=True)
    source_external_id: Mapped[str | None] = mapped_column(String(255), nullable=True)
    canonical_key: Mapped[str | None] = mapped_column(String(255), nullable=True)
    runtime_key: Mapped[str | None] = mapped_column(String(255), nullable=True)
    panel_website_id: Mapped[str | None] = mapped_column(String(255), nullable=True)
    website_alias: Mapped[str | None] = mapped_column(String(255), nullable=True)
    primary_domain: Mapped[str | None] = mapped_column(String(255), nullable=True)
    domains_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    proxy_target: Mapped[str | None] = mapped_column(String(512), nullable=True)
    https_enabled: Mapped[bool] = mapped_column(default=False, nullable=False)
    https_port: Mapped[int | None] = mapped_column(nullable=True)
    http_config: Mapped[str | None] = mapped_column(Text, nullable=True)
    ssl_certificate_ref: Mapped[str | None] = mapped_column(String(255), nullable=True)
    proxy_enabled: Mapped[bool] = mapped_column(default=False, nullable=False)
    proxy_cache_enabled: Mapped[bool] = mapped_column(default=False, nullable=False)
    manual_override_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    status: Mapped[str] = mapped_column(String(20), default="active", nullable=False)
    last_synced_at: Mapped[datetime | None] = mapped_column(nullable=True)
    last_remote_hash: Mapped[str | None] = mapped_column(String(64), nullable=True)
    last_error_code: Mapped[str | None] = mapped_column(String(50), nullable=True)
    last_error_message: Mapped[str | None] = mapped_column(Text, nullable=True)
    created_at: Mapped[datetime] = mapped_column(default=datetime.utcnow)
    updated_at: Mapped[datetime] = mapped_column(default=datetime.utcnow, onupdate=datetime.utcnow)
    archived_at: Mapped[datetime | None] = mapped_column(nullable=True)