# -*- coding: utf-8 -*-
"""Nodes, Tunnels, WebsiteMappings Pydantic schemas."""

from datetime import datetime

from pydantic import BaseModel, ConfigDict


class NodeOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    account_id: str
    provider: str
    external_id: str | None
    canonical_name: str | None
    display_name: str | None
    node_type: str | None
    endpoint_url: str | None
    region: str | None
    status: str
    health_status: str | None
    ban_until: datetime | None
    last_seen_at: datetime | None
    last_success_at: datetime | None
    last_error_code: str | None
    last_error_message: str | None
    metadata_json: str | None
    created_at: datetime
    updated_at: datetime
    archived_at: datetime | None


class TunnelOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    account_id: str
    node_id: str | None
    external_id: str | None
    name: str | None
    canonical_key: str | None
    runtime_key: str | None
    tunnel_type: str | None
    local_ip: str | None
    local_port: int | None
    remote_port: int | None
    dns_domain_cname: str | None
    dns_proxied: bool
    desired_state: str | None
    actual_state: str | None
    state_reason: str | None
    desired_hash: str | None
    observed_hash: str | None
    last_applied_snapshot_id: str | None
    last_applied_at: datetime | None
    last_error_code: str | None
    last_error_message: str | None
    manual_override_json: str | None
    created_at: datetime
    updated_at: datetime
    archived_at: datetime | None


class WebsiteMappingOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    account_id: str
    node_id: str | None
    tunnel_id: str | None
    source_kind: str | None
    source_external_id: str | None
    canonical_key: str | None
    runtime_key: str | None
    panel_website_id: str | None
    website_alias: str | None
    primary_domain: str | None
    domains_json: str | None
    proxy_target: str | None
    https_enabled: bool
    https_port: int | None
    http_config: str | None
    ssl_certificate_ref: str | None
    proxy_enabled: bool
    proxy_cache_enabled: bool
    manual_override_json: str | None
    status: str
    last_synced_at: datetime | None
    last_remote_hash: str | None
    last_error_code: str | None
    last_error_message: str | None
    created_at: datetime
    updated_at: datetime
    archived_at: datetime | None
