# -*- coding: utf-8 -*-
"""FRPC runtime API endpoints."""

import json
import uuid
from datetime import datetime
from typing import Any

from fastapi import APIRouter, Body, Depends, HTTPException
from sqlalchemy.orm import Session

from app.core.config import settings
from app.db.session import get_db
from app.models import Job, JobEvent
from app.services.frpc_runtime import (
    FRPCRuntimeManager,
    FrpcStatus,
    render_frpc_config,
    compute_config_hash,
    write_frpc_config,
)

router = APIRouter()


def _add_job_event(db: Session, job_id: str, event_type: str, message: str | None = None) -> None:
    count = db.query(JobEvent).filter(JobEvent.job_id == job_id).count()
    event = JobEvent(
        id=str(uuid.uuid4()),
        job_id=job_id,
        sequence_no=count + 1,
        event_type=event_type,
        level="info",
        message=message,
    )
    db.add(event)
    db.commit()


def _create_job(db: Session, job_type: str, payload: dict, *, priority: int = 0) -> Job:
    job = Job(
        id=str(uuid.uuid4()),
        account_id="system",
        job_type=job_type,
        priority=priority,
        payload_json=json.dumps(payload, ensure_ascii=False),
    )
    db.add(job)
    db.commit()
    _add_job_event(db, job.id, f"job.{job_type}.created", f"Job created: {job_type}")
    return job


@router.get("/runtime")
def get_runtime_status() -> dict:
    return FRPCRuntimeManager.get_status()


@router.post("/runtime/start")
def start_runtime(
    node_id: str | None = None,
    db: Session = Depends(get_db),
) -> dict:
    job = _create_job(db, "frpc.start", {"node_id": node_id})
    return {"job_id": job.id, "status": job.status}


@router.post("/runtime/stop")
def stop_runtime(
    db: Session = Depends(get_db),
) -> dict:
    job = _create_job(db, "frpc.stop", {})
    return {"job_id": job.id, "status": job.status}


@router.post("/runtime/restart")
def restart_runtime(
    node_id: str | None = None,
    db: Session = Depends(get_db),
) -> dict:
    job = _create_job(db, "frpc.restart", {"node_id": node_id})
    return {"job_id": job.id, "status": job.status}


@router.post("/runtime/reload")
def reload_runtime(
    db: Session = Depends(get_db),
) -> dict:
    job = _create_job(db, "frpc.reload", {})
    return {"job_id": job.id, "status": job.status}


@router.post("/runtime/switch-node")
def switch_node(
    node_id: str,
    db: Session = Depends(get_db),
) -> dict:
    job = _create_job(db, "frpc.switch_node", {"node_id": node_id}, priority=10)
    return {"job_id": job.id, "status": job.status}


@router.get("/runtime/health-check")
def get_runtime_health() -> dict:
    """Get detailed health check of the frpc runtime."""
    import asyncio
    return asyncio.run(FRPCRuntimeManager.health_check())


@router.post("/runtime/recover")
def recover_runtime(
    db: Session = Depends(get_db),
) -> dict:
    """Attempt automatic recovery of the frpc runtime."""
    import asyncio
    try:
        result = asyncio.run(FRPCRuntimeManager.recover())
        job = _create_job(db, "frpc.recover", {})
        return {"job_id": job.id, "status": job.status, "recover_result": result}
    except RuntimeError as exc:
        raise HTTPException(status_code=409, detail=str(exc))


@router.get("/runtime/logs")
def get_runtime_logs(
    lines: int = 100,
) -> dict:
    """Return stdout/stderr logs."""
    from app.services.frpc_runtime import _work_dir

    lines = min(lines, 5000)
    result: dict[str, Any] = {"stdout": [], "stderr": []}
    logs_dir = _work_dir() / "logs"
    stdout_path = logs_dir / "stdout.log"
    stderr_path = logs_dir / "stderr.log"

    for key, path in [("stdout", stdout_path), ("stderr", stderr_path)]:
        if path.exists():
            with open(path, "r", encoding="utf-8") as f:
                result[key] = f.read().splitlines()[-lines:]

    return result


@router.get("/runtime/config")
def get_runtime_config() -> dict:
    from app.services.frpc_runtime import _config_path

    path = _config_path()
    if not path.exists():
        return {"exists": False, "content": None, "hash": None}
    content = path.read_text(encoding="utf-8")
    return {
        "exists": True,
        "content": content,
        "hash": compute_config_hash(content),
    }


@router.post("/runtime/config/render")
def render_runtime_config(
    data: dict = Body(...),
    db: Session = Depends(get_db),
) -> dict:
    """Render frpc config and optionally write it atomically.

    Expects body with: server_addr, server_port, token, tunnels, log_level
    """
    server_addr = data.get("server_addr", "127.0.0.1")
    server_port = data.get("server_port", 7000)
    token = data.get("token", "")
    tunnels = data.get("tunnels", [])
    log_level = data.get("log_level", settings.FRPC_LOG_LEVEL)

    content = render_frpc_config(
        server_addr=server_addr,
        server_port=server_port,
        token=token,
        tunnels=tunnels,
        log_level=log_level,
    )
    return {
        "content_preview": content[:500],
        "hash": compute_config_hash(content),
        "status": "rendered",
    }
