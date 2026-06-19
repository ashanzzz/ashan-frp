# -*- coding: utf-8 -*-
"""Account and auth token Pydantic schemas."""

from datetime import datetime

from pydantic import BaseModel, ConfigDict


class AccountOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    login_name: str
    email: str | None
    display_name: str | None
    role: str
    status: str
    last_login_at: datetime | None
    created_at: datetime
    updated_at: datetime


class AuthTokenOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    account_id: str
    token_prefix: str | None
    token_name: str | None
    scopes_json: str | None
    issued_at: datetime | None
    expires_at: datetime | None
    revoked_at: datetime | None
    last_used_at: datetime | None
    created_ip: str | None
    user_agent: str | None
    created_at: datetime
    updated_at: datetime


class UpstreamCredentialOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    account_id: str
    provider: str
    name: str
    credential_type: str | None
    scopes_json: str | None
    token_expires_at: datetime | None
    status: str
    last_validated_at: datetime | None
    last_error_at: datetime | None
    last_error_message: str | None
    metadata_json: str | None
    created_at: datetime
    updated_at: datetime
