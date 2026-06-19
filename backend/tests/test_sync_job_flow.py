from __future__ import annotations

import asyncio
import json
import uuid
from datetime import datetime, timedelta
from typing import Any

from fastapi import FastAPI
from fastapi.testclient import TestClient
from sqlalchemy import create_engine
from sqlalchemy.orm import Session, sessionmaker
from sqlalchemy.pool import StaticPool

from app.api.endpoints import jobs as jobs_endpoint
from app.api.endpoints import website_mappings as website_mappings_endpoint
from app.api.routes import api_router
from app.db.base import Base
from app.models import Account, Job, JobEvent, SyncState, WebsiteMapping
from app.services.job_runner import JobRunner
from app.services.website_sync import WebsiteSyncService


def build_session_factory() -> sessionmaker[Session]:
    engine = create_engine(
        "sqlite://",
        connect_args={"check_same_thread": False},
        poolclass=StaticPool,
    )
    Base.metadata.create_all(engine)
    return sessionmaker(bind=engine, autocommit=False, autoflush=False)


def build_session() -> Session:
    return build_session_factory()()


def add_account(db: Session, account_id: str = "acct-1") -> None:
    db.add(Account(id=account_id, login_name=f"login-{account_id}"))
    db.commit()


def add_mapping(
    db: Session,
    mapping_id: str = "mapping-1",
    account_id: str = "acct-1",
) -> WebsiteMapping:
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
        self.remote_base: dict[str, dict[str, Any]] = {}
        self.remote_domains: dict[str, list[dict[str, Any]]] = {}
        self.remote_proxy: dict[str, dict[str, Any]] = {}
        self.remote_https: dict[str, dict[str, Any]] = {}

    async def create_website(self, payload: dict[str, Any]) -> dict[str, Any]:
        self.calls.append(("create_website", payload))
        website_id = "101"
        self.remote_base[website_id] = {
            "alias": payload.get("alias"),
            "primaryDomain": payload.get("primaryDomain"),
        }
        self.remote_domains[website_id] = [
            {"domain": domain} for domain in payload.get("domains", [])
        ]
        self.remote_proxy[website_id] = {
            "enable": payload.get("proxyEnable", False),
            "proxyPass": payload.get("proxyTarget"),
            "cache": payload.get("proxyCache", False),
        }
        https_config: dict[str, Any] = {"enable": payload.get("httpsEnable", False)}
        if payload.get("httpsPort") is not None:
            https_config["httpConfig"] = json.dumps(
                {"listen": payload["httpsPort"]},
                ensure_ascii=False,
                separators=(",", ":"),
            )
        self.remote_https[website_id] = https_config
        return {"data": {"id": int(website_id)}}

    async def get_website(self, website_id: str) -> dict[str, Any]:
        self.calls.append(("get_website", website_id))
        return {"data": {"id": int(website_id), **self.remote_base.get(str(website_id), {})}}

    async def update_website(self, website_id: str, payload: dict[str, Any]) -> dict[str, Any]:
        self.calls.append(("update_website", website_id, payload))
        self.remote_base.setdefault(str(website_id), {}).update(
            {
                "alias": payload.get("alias", self.remote_base.get(str(website_id), {}).get("alias")),
                "primaryDomain": payload.get(
                    "primaryDomain",
                    self.remote_base.get(str(website_id), {}).get("primaryDomain"),
                ),
            }
        )
        return {"data": {"id": int(website_id)}}

    async def get_website_domains(self, website_id: str) -> list[dict[str, Any]]:
        self.calls.append(("get_website_domains", website_id))
        return self.remote_domains.get(str(website_id), [])

    async def add_website_domain(self, website_id: str, domain: str) -> dict[str, Any]:
        self.calls.append(("add_website_domain", website_id, domain))
        self.remote_domains.setdefault(str(website_id), []).append({"domain": domain})
        return {"ok": True}

    async def remove_website_domain(self, website_id: str, domain_id: str) -> dict[str, Any]:
        self.calls.append(("remove_website_domain", website_id, domain_id))
        return {"ok": True}

    async def get_website_proxy(self, website_id: str) -> dict[str, Any]:
        self.calls.append(("get_website_proxy", website_id))
        return {"data": self.remote_proxy.get(str(website_id), {})}

    async def set_website_proxy(self, website_id: str, target: str, **extra: Any) -> dict[str, Any]:
        self.calls.append(("set_website_proxy", website_id, target, extra))
        self.remote_proxy[str(website_id)] = {
            "enable": extra.get("enable", True),
            "proxyPass": target,
            "cache": extra.get("cache", False),
        }
        return {"ok": True}

    async def get_website_https(self, website_id: str) -> dict[str, Any]:
        self.calls.append(("get_website_https", website_id))
        return {"data": self.remote_https.get(str(website_id), {})}

    async def set_website_https(
        self,
        website_id: str,
        *,
        enabled: bool,
        port: int | None = None,
        **_: Any,
    ) -> dict[str, Any]:
        self.calls.append(("set_website_https", website_id, enabled, port))
        payload: dict[str, Any] = {"enable": enabled}
        if port is not None:
            payload["httpConfig"] = json.dumps(
                {"listen": port},
                ensure_ascii=False,
                separators=(",", ":"),
            )
        self.remote_https[str(website_id)] = payload
        return {"ok": True}


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


def test_sync_existing_mapping_uses_normalized_observed_state_and_skips_update() -> None:
    db = build_session()
    add_account(db)
    mapping = add_mapping(db)
    mapping.panel_website_id = "101"
    db.commit()

    adapter = FakeAdapter()
    adapter.remote_base["101"] = {
        "alias": "Example",
        "primaryDomain": "example.com",
    }
    adapter.remote_domains["101"] = [{"domain": "www.example.com"}]
    adapter.remote_proxy["101"] = {
        "enable": True,
        "proxyPass": "http://127.0.0.1:8080",
        "cache": False,
    }
    adapter.remote_https["101"] = {
        "enable": True,
        "httpConfig": json.dumps({"listen": 8443}, ensure_ascii=False, separators=(",", ":")),
    }
    service = WebsiteSyncService(db, adapter)

    result = asyncio.run(service.sync_single_mapping(mapping.id, job_id="job-2"))

    assert result["status"] == "ok"
    assert result["result"] == "no_change"
    assert not any(call[0] == "update_website" for call in adapter.calls)
    assert not any(call[0] == "set_website_proxy" for call in adapter.calls)
    assert not any(call[0] == "set_website_https" for call in adapter.calls)


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


def test_job_runner_reclaims_due_retry_wait_jobs(monkeypatch) -> None:
    session_factory = build_session_factory()
    db = session_factory()
    add_account(db)
    job = Job(
        id=str(uuid.uuid4()),
        account_id="acct-1",
        job_type="sync_all_website_mappings",
        status="retry_wait",
        run_after=datetime.utcnow() - timedelta(seconds=5),
        payload_json=json.dumps({}),
        max_attempts=3,
    )
    db.add(job)
    db.commit()

    runner = JobRunner()
    monkeypatch.setattr("app.services.job_runner.SessionLocal", session_factory)

    async def succeed(_: Session, __: Job) -> dict[str, Any]:
        return {"status": "ok"}

    monkeypatch.setattr(runner, "_execute_job", succeed)

    asyncio.run(runner._tick())

    fresh = session_factory()
    stored = fresh.get(Job, job.id)
    assert stored is not None
    assert stored.status == "succeeded"
    assert json.loads(stored.result_json) == {"status": "ok"}


def test_job_runner_does_not_overwrite_canceled_job(monkeypatch) -> None:
    session_factory = build_session_factory()
    db = session_factory()
    add_account(db)
    job = Job(
        id=str(uuid.uuid4()),
        account_id="acct-1",
        job_type="sync_all_website_mappings",
        status="queued",
        payload_json=json.dumps({}),
        max_attempts=3,
    )
    db.add(job)
    db.commit()

    runner = JobRunner()
    monkeypatch.setattr("app.services.job_runner.SessionLocal", session_factory)

    async def cancel_in_separate_session(_: Session, actual_job: Job) -> dict[str, Any]:
        other = session_factory()
        try:
            target = other.get(Job, actual_job.id)
            assert target is not None
            target.status = "canceled"
            target.completed_at = datetime.utcnow()
            target.result_json = json.dumps({"status": "canceled"}, ensure_ascii=False)
            other.commit()
        finally:
            other.close()
        return {"status": "ignored"}

    monkeypatch.setattr(runner, "_execute_job", cancel_in_separate_session)

    asyncio.run(runner._tick())

    fresh = session_factory()
    stored = fresh.get(Job, job.id)
    assert stored is not None
    assert stored.status == "canceled"
    assert json.loads(stored.result_json) == {"status": "canceled"}
