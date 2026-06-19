# -*- coding: utf-8 -*-
"""Tunnels endpoint."""

from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.orm import Session

from app.db.session import get_db
from app.models import Tunnel

router = APIRouter()


@router.get("")
def list_tunnels(db: Session = Depends(get_db)) -> list:
    return db.query(Tunnel).all()


@router.get("/{tunnel_id}")
def get_tunnel(tunnel_id: str, db: Session = Depends(get_db)) -> dict:
    t = db.query(Tunnel).filter(Tunnel.id == tunnel_id).first()
    if not t:
        raise HTTPException(status_code=404, detail="Tunnel not found")
    return {
        "id": t.id,
        "node_id": t.node_id,
        "name": t.name,
        "tunnel_type": t.tunnel_type,
        "local_ip": t.local_ip,
        "local_port": t.local_port,
        "remote_port": t.remote_port,
        "desired_state": t.desired_state,
        "actual_state": t.actual_state,
    }
