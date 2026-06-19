# -*- coding: utf-8 -*-
"""Nodes endpoint."""

from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.orm import Session

from app.db.session import get_db
from app.models import Node

router = APIRouter()


@router.get("")
def list_nodes(db: Session = Depends(get_db)) -> list:
    return db.query(Node).all()


@router.get("/{node_id}")
def get_node(node_id: str, db: Session = Depends(get_db)) -> dict:
    node = db.query(Node).filter(Node.id == node_id).first()
    if not node:
        raise HTTPException(status_code=404, detail="Node not found")
    return {
        "id": node.id,
        "provider": node.provider,
        "canonical_name": node.canonical_name,
        "display_name": node.display_name,
        "endpoint_url": node.endpoint_url,
        "region": node.region,
        "status": node.status,
        "health_status": node.health_status,
        "last_seen_at": node.last_seen_at,
    }
