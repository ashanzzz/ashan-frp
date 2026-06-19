# -*- coding: utf-8 -*-
"""Jobs, Settings, SyncState, AuditLog, Snapshot Pydantic schemas."""

from datetime import datetime

from pydantic import BaseModel, ConfigDict


class JobOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    account_id: str
    job_type: str
    target_type: str | None
    target_id: str | None
    idempotency_key: str | None
    priority: int
    status: str
    run_after: datetime | None
    locked_at: datetime | None
    locked_by: str | None
    attempt_count: int
    max_attempts: int
    payload_json: str | None
    result_json: str | None
    error_code: str | None
    error_message: str | None
    requested_by_account_id: str | None
    created_at: datetime
    updated_at: datetime
    started_at: datetime | None
    completed_at: datetime | None
    archived_at: datetime | None


class JobEventOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    job_id: str
    sequence_no: int
    event_type: str
    level: str
    message: str | None
    payload_json: str | None
    trace_id: str | None
    created_by: str | None
    created_at: datetime


class SettingOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    account_id: str | None
    scope_type: str
    scope_id: str | None
    key: str
    value_json: str | None
    value_type: str | None
    description: str | None
    updated_by: str | None
    updated_at: datetime
    created_at: datetime


class SyncStateOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    account_id: str
    subject_type: str
    subject_id: str
    desired_hash: str | None
    observed_hash: str | None
    last_snapshot_id: str | None
    last_job_id: str | None
    status: str
    conflict_reason: str | None
    dirty: bool
    retry_count: int
    next_retry_at: datetime | None
    locked_until: datetime | None
    last_success_at: datetime | None
    last_attempt_at: datetime | None
    last_error_code: str | None
    last_error_message: str | None
    manual_override_at: datetime | None
    manual_override_by: str | None
    metadata_json: str | None
    updated_at: datetime


class AuditLogOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    account_id: str | None
    actor_type: str | None
    actor_id: str | None
    action: str
    subject_type: str | None
    subject_id: str | None
    request_id: str | None
    job_id: str | None
    before_json: str | None
    after_json: str | None
    diff_json: str | None
    source_ip: str | None
    user_agent: str | None
    severity: str
    created_at: datetime


class SnapshotOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    account_id: str | None
    source_system: str
    source_ref: str | None
    subject_type: str | None
    subject_id: str | None
    snapshot_kind: str | None
    content_json: str | None
    content_hash: str | None
    captured_at: datetime
    captured_by_job_id: str | None
    retention_class: str | None
    expires_at: datetime | None
    created_at: datetime


class OverviewOut(BaseModel):
    total_nodes: int
    total_tunnels: int
    total_website_mappings: int
    total_jobs_queued: int
    total_jobs_running: int
    total_jobs_failed: int
    total_settings: int
