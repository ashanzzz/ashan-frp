# -*- coding: utf-8 -*-
"""Settings endpoint."""

from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.orm import Session

from app.db.session import get_db
from app.models import Setting

router = APIRouter()


@router.get("")
def list_settings(db: Session = Depends(get_db)) -> list:
    return db.query(Setting).all()


@router.post("")
def create_setting(key: str, value_json: str, db: Session = Depends(get_db)) -> dict:
    from datetime import datetime
    import uuid
    s = Setting(id=str(uuid.uuid4()), key=key, value_json=value_json)
    db.add(s)
    db.commit()
    db.refresh(s)
    return {"id": s.id, "key": s.key, "value_json": s.value_json}


@router.get("/{setting_id}")
def get_setting(setting_id: str, db: Session = Depends(get_db)) -> dict:
    s = db.query(Setting).filter(Setting.id == setting_id).first()
    if not s:
        raise HTTPException(status_code=404, detail="Setting not found")
    return {"id": s.id, "key": s.key, "value_json": s.value_json, "updated_at": s.updated_at}
