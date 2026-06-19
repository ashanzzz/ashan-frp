# -*- coding: utf-8 -*-
"""Jobs endpoint with job events support."""

from datetime import datetime
import json
import uuid

from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.orm import Session

from app.db.session import get_db
from app.models import Job, JobEvent

router = APIRouter()


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
def list_jobs(db: Session = Depends(get_db)) -> list:
    return db.query(Job).order_by(Job.created_at.desc()).all()


@router.get("/{job_id}")
def get_job(job_id: str, db: Session = Depends(get_db)) -> dict:
    j = db.query(Job).filter(Job.id == job_id).first()
    if not j:
        raise HTTPException(status_code=404, detail="Job not found")
    return {
        "id": j.id,
        "job_type": j.job_type,
        "target_type": j.target_type,
        "target_id": j.target_id,
        "status": j.status,
        "priority": j.priority,
        "attempt_count": j.attempt_count,
        "max_attempts": j.max_attempts,
        "error_code": j.error_code,
        "error_message": j.error_message,
        "created_at": j.created_at,
        "started_at": j.started_at,
        "completed_at": j.completed_at,
    }


@router.get("/{job_id}/events")
def list_job_events(job_id: str, db: Session = Depends(get_db)) -> list:
    events = db.query(JobEvent).filter(JobEvent.job_id == job_id).order_by(JobEvent.sequence_no).all()
    return [
        {
            "id": e.id,
            "sequence_no": e.sequence_no,
            "event_type": e.event_type,
            "level": e.level,
            "message": e.message,
            "payload_json": e.payload_json,
            "created_at": e.created_at,
        }
        for e in events
    ]


@router.post("/{job_id}/cancel")
def cancel_job(job_id: str, db: Session = Depends(get_db)) -> dict:
    j = db.query(Job).filter(Job.id == job_id).first()
    if not j:
        raise HTTPException(status_code=404, detail="Job not found")
    if j.status not in ("queued", "running", "retry_wait"):
        raise HTTPException(status_code=400, detail=f"Cannot cancel job in status: {j.status}")

    j.status = "canceled"
    j.completed_at = datetime.utcnow()
    j.result_json = json.dumps({"status": "canceled"}, ensure_ascii=False)
    db.commit()
    _add_job_event(db, job_id, "job.canceled", "Job canceled by operator")
    return {"status": "canceled", "job_id": job_id}
