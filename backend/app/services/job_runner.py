# -*- coding: utf-8 -*-
"""Job Runner — async periodic task executor with real handlers."""

import asyncio
import json
import random
import uuid
from datetime import datetime, timedelta
from typing import Any

from sqlalchemy.orm import Session

from app.adapters.onepanel import OnePanelAdapter
from app.adapters.onepanel_client import OnePanelClient
from app.core.config import settings
from app.db.session import SessionLocal
from app.models import Job, JobEvent


def _add_event(
    db: Session,
    job_id: str,
    event_type: str,
    message: str | None = None,
    level: str = "info",
    payload: dict[str, Any] | None = None,
) -> None:
    count = db.query(JobEvent).filter(JobEvent.job_id == job_id).count()
    event = JobEvent(
        id=str(uuid.uuid4()),
        job_id=job_id,
        sequence_no=count + 1,
        event_type=event_type,
        level=level,
        message=message,
        payload_json=json.dumps(payload, ensure_ascii=False) if payload else None,
    )
    db.add(event)
    db.commit()


class JobRunner:
    """Background runner that polls for queued jobs and executes them."""

    def __init__(self) -> None:
        self._task: asyncio.Task | None = None
        self._running = False

    async def start(self) -> None:
        self._running = True
        self._task = asyncio.create_task(self._poll_loop())

    async def stop(self) -> None:
        self._running = False
        if self._task:
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass

    async def _poll_loop(self) -> None:
        while self._running:
            try:
                await self._tick()
            except Exception:  # noqa: BLE001
                pass
            await asyncio.sleep(settings.JOB_RUNNER_INTERVAL)

    async def _tick(self) -> None:
        db = SessionLocal()
        try:
            now = datetime.utcnow()
            job = (
                db.query(Job)
                .filter(Job.status == "queued", (Job.run_after == None) | (Job.run_after <= now))  # noqa: E711
                .order_by(Job.priority.desc(), Job.created_at)
                .first()
            )
            if not job:
                return

            job.status = "running"
            job.locked_at = now
            job.locked_by = "runner-1"
            job.started_at = now
            job.attempt_count += 1
            db.commit()
            _add_event(db, job.id, "job.running", "Job execution started")

            try:
                result = await self._execute_job(db, job)
                job.status = "succeeded"
                job.result_json = json.dumps(result, ensure_ascii=False)
                job.completed_at = datetime.utcnow()
                _add_event(db, job.id, "job.succeeded", "Job completed successfully")
            except Exception as exc:  # noqa: BLE001
                job.error_code = "execution_error"
                job.error_message = str(exc)
                if job.attempt_count < job.max_attempts:
                    job.status = "retry_wait"
                    job.run_after = self.compute_retry_after(job.attempt_count)
                    _add_event(
                        db,
                        job.id,
                        "job.retry_wait",
                        f"Retry scheduled at {job.run_after}",
                        level="warning",
                        payload={"attempt": job.attempt_count, "next_retry": job.run_after.isoformat()},
                    )
                else:
                    job.status = "failed"
                    job.completed_at = datetime.utcnow()
                    _add_event(db, job.id, "job.failed", str(exc), level="error")
            finally:
                job.locked_at = None
                job.locked_by = None
                db.commit()
        finally:
            db.close()

    async def _execute_job(self, db: Session, job: Any) -> dict[str, Any]:
        handlers = {
            "sync_website_mapping": self._handle_sync_website_mapping,
            "sync_all_website_mappings": self._handle_sync_all_website_mappings,
        }
        handler = handlers.get(job.job_type)
        if not handler:
            raise RuntimeError(f"No handler for job_type: {job.job_type}")
        return await handler(db, job)

    async def _handle_sync_website_mapping(self, db: Session, job: Any) -> dict[str, Any]:
        _ = db
        payload = json.loads(job.payload_json) if job.payload_json else {}
        mapping_id = payload.get("mapping_id")
        if not mapping_id:
            raise ValueError("sync_website_mapping job requires mapping_id in payload")
        return await asyncio.to_thread(self._run_sync_single, mapping_id, job.id)

    async def _handle_sync_all_website_mappings(self, db: Session, job: Any) -> dict[str, Any]:
        _ = db
        return await asyncio.to_thread(self._run_sync_all, job.id)

    def _run_sync_single(self, mapping_id: str, job_id: str) -> dict[str, Any]:
        db = SessionLocal()
        adapter = self._build_adapter()
        try:
            from app.services.website_sync import run_sync_job

            return run_sync_job(db, adapter, mapping_id=mapping_id, job_id=job_id)
        finally:
            db.close()
            if adapter.client is not None:
                asyncio.run(adapter.client.close())

    def _run_sync_all(self, job_id: str) -> dict[str, Any]:
        db = SessionLocal()
        adapter = self._build_adapter()
        try:
            from app.services.website_sync import run_sync_job

            return run_sync_job(db, adapter, job_id=job_id)
        finally:
            db.close()
            if adapter.client is not None:
                asyncio.run(adapter.client.close())

    @staticmethod
    def _build_adapter() -> OnePanelAdapter:
        client = OnePanelClient(base_url=settings.ONEPANEL_BASE_URL, api_key=settings.ONEPANEL_API_KEY)
        return OnePanelAdapter(client=client)

    @staticmethod
    def compute_retry_after(attempt_count: int) -> datetime:
        backoff = min(
            settings.JOB_RETRY_MAX_SECONDS,
            settings.JOB_RETRY_BASE_SECONDS * (2 ** (attempt_count - 1)),
        )
        jitter = random.uniform(0, backoff)  # noqa: S311
        return datetime.utcnow() + timedelta(seconds=jitter)
