# -*- coding: utf-8 -*-
"""Regression tests for website sync, API endpoints, and the job runner."""

from __future__ import annotations

import asyncio
import json
import uuid
from datetime import datetime
from typing import Any

from fastapi.testclient import TestClient

from app.api.routes import api_router
from app.api.endpoints import website_mappings as website_mappings_endpoint
from app.api.endpoints import jobs as jobs_endpoint
from app.db.base import Base
from app.models import Account, Job, JobEvent, SyncState, WebsiteMapping
from app.services.job_runner import JobRunner
from app.services.website_sync import WebsiteSyncService
from fastapi import FastAPI
from sqlalchemy import create_engine
from sqlalchemy.orm import Session, sessionmaker
from sqlalchemy.pool import StaticPool


def build_session() -> Session:
    engine = create_engine(
        "sqlite://",
        connect_args={"check_same_thread": False},
        poolclass=StaticPool,
    )
    Base.metadata.create_all(engine)
    local = sessionmaker(bind=engine, autocommit=False, autoflush=False)
    return local()


def add_account(db: Session, account_id: str = "acct-1") -> None:
    db.add(Account(id=account_id, login_name=f"login-{account_id}"))
    db.commit()


def add_mapping(db: Session, mapping_id: str = "mapping-1", account_id: str = "acct-1") -> WebsiteMapping:
    mapping = WebsiteMapping(
        id=mapping_id,
        account_id=account_id,
        website_alias="Example",
        primary_domain="example.com",
        domains_json=json.dumps(["www.example.com"]),
        proxy_target="http://127.0.0.1:8080",
        proxy_enabled=True,
        https_enabled=True,
        https_port=8443,
        status="active",
    )
    db.add(mapping)
    db.commit()
    return mapping


class FakeAdapter:
    def __init__(self) -> None:
        self.calls: list[tuple[str, Any]] = []
        self.remote: dict[str, dict] = {}

    async def create_website(self, payload: dict[str, Any]) -> dict[str, Any]:
        self.calls.append(("create_website", payload))
        self.remote["101"] = payload.copy()
        return {"data": {"id": 101}}

    async def get_website(self, website_id: str) -> dict[str, Any]:
        self.calls.append(("get_website", website_id))
        return self.remote.get(str(website_id), {})

    async def update_website(self, website_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        self.calls.append(("update_website", website_id, payload))
        self.remote.setdefault(str(website_id), {}).update(payload)
        return {"data": {"id": int(website_id)}}


def test_sync_single_mapping_is_async_safe_and_records_snapshot_and_sync_state() -> None:
    db = build_session()
    add_account(db)
    mapping = add_mapping(db)
    adapter = FakeAdapter()
    service = WebsiteSyncService(db, adapter)

    async def run_inside_existing_loop() -> dict[str, Any]:
        return await service.sync_single_mapping(mapping.id, job_id="job-1")

    result = asyncio.run(run_inside_existing_loop())

    assert result["status"] == "ok"
    assert result["result"] == "synced"
    db.refresh(mapping)
    assert mapping.panel_website_id == "101"
    state = db.query(SyncState).filter_by(subject_type="website_mapping", subject_id=mapping.id).one()
    assert state.status == "synced"
    assert state.dirty is False
    assert state.last_job_id == "job-1"
    assert state.last_attempt_at is not None
    assert state.last_snapshot_id is not None
    assert json.loads(state.metadata_json)["last_result"] == "synced"


def test_override_endpoint_accepts_json_body_and_marks_sync_state_manual_override() -> None:
    db = build_session()
    add_account(db)
    mapping = add_mapping(db)
    app = FastAPI()
    app.include_router(api_router, prefix="/api")

    def override_db():
        try:
            yield db
        finally:
            pass

    app.dependency_overrides[website_mappings_endpoint.get_db] = override_db
    client = TestClient(app)

    response = client.post(
        f"/api/website-mappings/{mapping.id}/override",
        json={"active": True, "reason": "manual check", "requested_by": "operator"},
    )

    assert response.status_code == 200
    assert response.json()["override_active"] is True
    db.refresh(mapping)
    override = json.loads(mapping.manual_override_json)
    assert override["active"] is True
    assert override["reason"] == "manual check"
    state = db.query(SyncState).filter_by(subject_type="website_mapping", subject_id=mapping.id).one()
    assert state.status == "manual_override"
    assert state.manual_override_by == "operator"


def test_cancel_job_uses_canceled_status_and_event() -> None:
    db = build_session()
    add_account(db)
    job = Job(
        id=str(uuid.uuid4()),
        account_id="acct-1",
        job_type="sync_all_website_mappings",
        status="queued",
    )
    db.add(job)
    db.commit()
    app = FastAPI()
    app.include_router(api_router, prefix="/api")

    def override_db():
        try:
            yield db
        finally:
            pass

    app.dependency_overrides[jobs_endpoint.get_db] = override_db
    client = TestClient(app)

    response = client.post(f"/api/jobs/{job.id}/cancel")

    assert response.status_code == 200
    assert response.json()["status"] == "canceled"
    db.refresh(job)
    assert job.status == "canceled"
    event = db.query(JobEvent).filter_by(job_id=job.id).one()
    assert event.event_type == "job.canceled"


def test_job_runner_uses_retry_wait_and_thread_local_session(monkeypatch) -> None:
    class FakeJob:
        def __init__(self) -> None:
            self.id = "job-1"
            self.job_type = "sync_website_mapping"
            self.status = "queued"
            self.run_after = None
            self.priority = 0
            self.created_at = datetime.utcnow()
            self.locked_at = None
            self.locked_by = None
            self.started_at = None
            self.attempt_count = 0
            self.max_attempts = 2
            self.payload_json = json.dumps({"mapping_id": "mapping-1"})
            self.result_json = None
            self.error_code = None
            self.error_message = None
            self.completed_at = None

    class FakeQuery:
        def __init__(self, job: FakeJob | None = None) -> None:
            self.job = job

        def filter(self, *args, **kwargs):
            return self

        def order_by(self, *args, **kwargs):
            return self

        def first(self):
            return self.job

        def count(self):
            return 0

    class FakeDB:
        def __init__(self, job: FakeJob | None = None) -> None:
            self.job = job
            self.commits = 0
            self.added = []
            self.closed = False

        def query(self, model):
            return FakeQuery(self.job)

        def add(self, obj):
            self.added.append(obj)

        def commit(self):
            self.commits += 1

        def close(self):
            self.closed = True

    job = FakeJob()
    outer_db = FakeDB(job)
    session_instances = [outer_db]

    def fake_session_local():
        session = session_instances.pop(0)
        return session

    runner = JobRunner()
    monkeypatch.setattr("app.services.job_runner.SessionLocal", fake_session_local)

    def fail_if_outer_session_used(db: Session, actual_job: Job):
        raise RuntimeError("simulated failure")

    monkeypatch.setattr(runner, "_execute_job", fail_if_outer_session_used)

    asyncio.run(runner._tick())

    assert job.status == "retry_wait"
    assert job.run_after is not None
    assert job.locked_at is None
    assert job.locked_by is None
