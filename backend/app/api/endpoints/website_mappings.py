# -*- coding: utf-8 -*-
"""Website Mappings endpoint with sync and job trigger support."""

import json
import uuid
from datetime import datetime

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.db.session import get_db
from app.models import Job, JobEvent, SyncState, WebsiteMapping

router = APIRouter()


class ManualOverrideRequest(BaseModel):
    active: bool = True
    reason: str | None = None
    requested_by: str | None = None


def _add_job_event(db: Session, job_id: str, event_type: str, message: str | None = None) -> None:
    count = db.query(JobEvent).filter(JobEvent.job_id == job_id).count()
    db.add(
        JobEvent(
            id=str(uuid.uuid4()),
            job_id=job_id,
            sequence_no=count + 1,
            event_type=event_type,
            message=message,
        )
    )
    db.commit()


@router.get("")
def list_website_mappings(db: Session = Depends(get_db)) -> list:
    return db.query(WebsiteMapping).all()


@router.get("/{mapping_id}")
def get_website_mapping(mapping_id: str, db: Session = Depends(get_db)) -> dict:
    m = db.query(WebsiteMapping).filter(WebsiteMapping.id == mapping_id).first()
    if not m:
        raise HTTPException(status_code=404, detail="Website mapping not found")
    return {
        "id": m.id,
        "node_id": m.node_id,
        "tunnel_id": m.tunnel_id,
        "website_alias": m.website_alias,
        "primary_domain": m.primary_domain,
        "proxy_target": m.proxy_target,
        "https_enabled": m.https_enabled,
        "proxy_enabled": m.proxy_enabled,
        "status": m.status,
    }


@router.post("/{mapping_id}/sync")
def trigger_sync(mapping_id: str, db: Session = Depends(get_db)) -> dict:
    mapping = db.query(WebsiteMapping).filter(WebsiteMapping.id == mapping_id).first()
    if not mapping:
        raise HTTPException(status_code=404, detail="Website mapping not found")

    job = Job(
        id=str(uuid.uuid4()),
        account_id=mapping.account_id,
        job_type="sync_website_mapping",
        target_type="website_mapping",
        target_id=mapping_id,
        payload_json=json.dumps({"mapping_id": mapping_id}, ensure_ascii=False),
        status="queued",
        priority=0,
        max_attempts=5,
        idempotency_key=f"sync_website_mapping:{mapping_id}",
    )
    db.add(job)
    db.commit()
    _add_job_event(db, job.id, "job.queued", "Website mapping sync queued")

    sync_state = db.query(SyncState).filter(
        SyncState.subject_type == "website_mapping",
        SyncState.subject_id == mapping_id,
    ).first()
    if sync_state:
        sync_state.last_job_id = job.id
        sync_state.status = "dirty"
        sync_state.dirty = True
        sync_state.last_attempt_at = datetime.utcnow()
        db.commit()

    return {"status": "queued", "job_id": job.id, "mapping_id": mapping_id}


@router.post("/sync-all")
def trigger_sync_all(db: Session = Depends(get_db)) -> dict:
    job = Job(
        id=str(uuid.uuid4()),
        account_id="system",
        job_type="sync_all_website_mappings",
        status="queued",
        priority=0,
        max_attempts=3,
        idempotency_key="sync_all_website_mappings",
    )
    db.add(job)
    db.commit()
    _add_job_event(db, job.id, "job.queued", "Full website mapping sync queued")
    return {"status": "queued", "job_id": job.id}


@router.post("/{mapping_id}/override")
def set_manual_override(
    mapping_id: str,
    request: ManualOverrideRequest,
    db: Session = Depends(get_db),
) -> dict:
    mapping = db.query(WebsiteMapping).filter(WebsiteMapping.id == mapping_id).first()
    if not mapping:
        raise HTTPException(status_code=404, detail="Website mapping not found")

    override_data = {
        "active": request.active,
        "reason": request.reason,
        "requested_by": request.requested_by,
        "set_at": datetime.utcnow().isoformat(),
    }
    mapping.manual_override_json = json.dumps(override_data, ensure_ascii=False)

    sync_state = db.query(SyncState).filter(
        SyncState.subject_type == "website_mapping",
        SyncState.subject_id == mapping_id,
    ).first()
    if not sync_state:
        sync_state = SyncState(
            id=str(uuid.uuid4()),
            account_id=mapping.account_id,
            subject_type="website_mapping",
            subject_id=mapping.id,
            dirty=not request.active,
            status="manual_override" if request.active else "dirty",
            manual_override_by=request.requested_by,
            manual_override_at=datetime.utcnow() if request.active else None,
        )
        db.add(sync_state)
    else:
        sync_state.status = "manual_override" if request.active else "dirty"
        sync_state.manual_override_by = request.requested_by
        sync_state.manual_override_at = datetime.utcnow() if request.active else None
        sync_state.dirty = not request.active
    db.commit()

    return {"status": "ok", "mapping_id": mapping_id, "override_active": request.active}


@router.get("/{mapping_id}/sync-state")
def get_sync_state(mapping_id: str, db: Session = Depends(get_db)) -> dict:
    sync_state = db.query(SyncState).filter(
        SyncState.subject_type == "website_mapping",
        SyncState.subject_id == mapping_id,
    ).first()
    if not sync_state:
        raise HTTPException(status_code=404, detail="Sync state not found")

    return {
        "subject_id": sync_state.subject_id,
        "status": sync_state.status,
        "desired_hash": sync_state.desired_hash,
        "observed_hash": sync_state.observed_hash,
        "dirty": sync_state.dirty,
        "retry_count": sync_state.retry_count,
        "last_success_at": sync_state.last_success_at,
        "last_error_code": sync_state.last_error_code,
        "last_error_message": sync_state.last_error_message,
    }
