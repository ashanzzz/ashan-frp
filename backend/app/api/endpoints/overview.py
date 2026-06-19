# -*- coding: utf-8 -*-
"""Overview / dashboard stats endpoint."""

from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

from app.db.session import get_db
from app.models import Job, Node, Setting, Tunnel, WebsiteMapping
from app.schemas import OverviewOut

router = APIRouter()


@router.get("", response_model=OverviewOut)
def get_overview(db: Session = Depends(get_db)) -> OverviewOut:  # noqa: B008
    return OverviewOut(
        total_nodes=db.query(Node).count(),
        total_tunnels=db.query(Tunnel).count(),
        total_website_mappings=db.query(WebsiteMapping).count(),
        total_jobs_queued=db.query(Job).filter(Job.status == "queued").count(),
        total_jobs_running=db.query(Job).filter(Job.status == "running").count(),
        total_jobs_failed=db.query(Job).filter(Job.status == "failed").count(),
        total_settings=db.query(Setting).count(),
    )
