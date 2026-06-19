# -*- coding: utf-8 -*-
"""Website sync service — desired vs observed reconciliation with 1Panel."""

from __future__ import annotations

import asyncio
import hashlib
import json
import uuid
from datetime import datetime
from typing import Any

from sqlalchemy.orm import Session

from app.adapters.onepanel import OnePanelAdapter
from app.models import Snapshot, SyncState, WebsiteMapping


def _website_desired_hash(mapping: WebsiteMapping) -> str:
    fields = [
        mapping.website_alias or "",
        mapping.primary_domain or "",
        mapping.domains_json or "[]",
        mapping.proxy_target or "",
        str(mapping.https_enabled),
        str(mapping.https_port or 443),
        str(mapping.proxy_enabled),
        str(mapping.proxy_cache_enabled),
    ]
    return hashlib.sha256("|".join(fields).encode()).hexdigest()


def _compute_delta(desired: dict[str, Any], observed: dict[str, Any]) -> dict[str, Any]:
    delta: dict[str, Any] = {}
    for key, value in desired.items():
        if observed.get(key) != value:
            delta[key] = value
    return delta


class WebsiteSyncService:
    """Orchestrates incremental sync of website mappings to 1Panel."""

    def __init__(self, db: Session, adapter: OnePanelAdapter) -> None:
        self.db = db
        self.adapter = adapter

    async def sync_single_mapping(self, mapping_id: str, *, job_id: str | None = None) -> dict[str, Any]:
        mapping = self.db.query(WebsiteMapping).filter(WebsiteMapping.id == mapping_id).first()
        if not mapping:
            return {"status": "error", "code": "MAPPING_NOT_FOUND", "message": f"No such mapping: {mapping_id}"}

        sync_state = self._get_or_create_sync_state(mapping)
        sync_state.last_job_id = job_id
        sync_state.last_attempt_at = datetime.utcnow()

        if self._is_manual_override(mapping):
            sync_state.status = "manual_override"
            sync_state.manual_override_at = datetime.utcnow()
            self.db.commit()
            return {"status": "skipped", "reason": "manual_override", "mapping_id": mapping_id}

        desired_hash = _website_desired_hash(mapping)
        if sync_state.desired_hash == desired_hash and not sync_state.dirty:
            sync_state.metadata_json = json.dumps({"last_result": "no_change"}, ensure_ascii=False)
            self.db.commit()
            return {"status": "ok", "result": "no_change", "mapping_id": mapping_id}

        sync_state.status = "running"
        self.db.commit()

        try:
            result = await self._apply_sync(mapping, sync_state, desired_hash, job_id=job_id)
        except Exception as exc:  # noqa: BLE001
            sync_state.status = "retry_wait"
            sync_state.retry_count += 1
            sync_state.last_error_code = "sync_exception"
            sync_state.last_error_message = str(exc)
            sync_state.metadata_json = json.dumps({"last_result": "error"}, ensure_ascii=False)
            self.db.commit()
            return {
                "status": "error",
                "code": "SYNC_EXCEPTION",
                "message": str(exc),
                "mapping_id": mapping_id,
            }

        return result

    async def full_sync(self, *, account_id: str | None = None, job_id: str | None = None) -> list[dict[str, Any]]:
        query = self.db.query(WebsiteMapping).filter(WebsiteMapping.status == "active")
        if account_id:
            query = query.filter(WebsiteMapping.account_id == account_id)
        results: list[dict[str, Any]] = []
        for mapping in query.all():
            results.append(await self.sync_single_mapping(mapping.id, job_id=job_id))
        return results

    async def _apply_sync(
        self,
        mapping: WebsiteMapping,
        sync_state: SyncState,
        desired_hash: str,
        *,
        job_id: str | None,
    ) -> dict[str, Any]:
        desired_payload = self._build_desired_payload(mapping)

        if mapping.panel_website_id:
            observed = await self._fetch_remote_state(mapping.panel_website_id)
            delta = _compute_delta(desired_payload, observed)
            if not delta:
                self._mark_clean(sync_state, desired_hash, mapping, result="no_change")
                return {"status": "ok", "result": "no_change", "mapping_id": mapping.id}
            await self._update_remote_website(mapping, delta)
        else:
            website_id = await self._create_remote_website(mapping, desired_payload)
            mapping.panel_website_id = website_id
            self.db.commit()

        snapshot_id = await self._capture_snapshot(mapping, "post_sync", job_id=job_id)
        self._mark_clean(sync_state, desired_hash, mapping, result="synced", snapshot_id=snapshot_id)
        return {"status": "ok", "result": "synced", "mapping_id": mapping.id}

    def _build_desired_payload(self, mapping: WebsiteMapping) -> dict[str, Any]:
        payload: dict[str, Any] = {
            "alias": mapping.website_alias,
            "primaryDomain": mapping.primary_domain,
            "proxyEnable": mapping.proxy_enabled,
            "proxyTarget": mapping.proxy_target,
            "httpsEnable": mapping.https_enabled,
            "proxyCache": mapping.proxy_cache_enabled,
        }
        if mapping.https_enabled and mapping.https_port and mapping.https_port != 443:
            payload["httpsPort"] = mapping.https_port
        if mapping.domains_json:
            try:
                payload["domains"] = json.loads(mapping.domains_json)
            except json.JSONDecodeError:
                payload["domains"] = []
        else:
            payload["domains"] = []
        return payload

    async def _create_remote_website(self, mapping: WebsiteMapping, payload: dict[str, Any]) -> str:
        _ = mapping
        resp = await self.adapter.create_website(payload)
        data = resp.get("data", {}) if isinstance(resp, dict) else {}
        website_id = data.get("id")
        if website_id is None:
            raise RuntimeError(f"1Panel did not return website id: {resp}")
        return str(website_id)

    async def _update_remote_website(self, mapping: WebsiteMapping, delta: dict[str, Any]) -> None:
        if not mapping.panel_website_id:
            raise ValueError("Cannot update remote website without panel_website_id")
        await self.adapter.update_website(mapping.panel_website_id, delta)

    async def _fetch_remote_state(self, website_id: str) -> dict[str, Any]:
        resp = await self.adapter.get_website(website_id)
        return resp if isinstance(resp, dict) else {}

    def _mark_clean(
        self,
        sync_state: SyncState,
        desired_hash: str,
        mapping: WebsiteMapping,
        *,
        result: str,
        snapshot_id: str | None = None,
    ) -> None:
        now = datetime.utcnow()
        sync_state.desired_hash = desired_hash
        sync_state.observed_hash = desired_hash
        sync_state.status = "synced"
        sync_state.dirty = False
        sync_state.last_success_at = now
        sync_state.retry_count = 0
        sync_state.next_retry_at = None
        sync_state.last_error_code = None
        sync_state.last_error_message = None
        sync_state.last_snapshot_id = snapshot_id
        sync_state.metadata_json = json.dumps({"last_result": result}, ensure_ascii=False)
        mapping.last_synced_at = now
        mapping.last_remote_hash = desired_hash
        self.db.commit()

    async def _capture_snapshot(self, mapping: WebsiteMapping, kind: str, *, job_id: str | None) -> str | None:
        if not mapping.panel_website_id:
            return None
        try:
            remote = await self.adapter.get_website(mapping.panel_website_id)
        except Exception:  # noqa: BLE001
            return None
        snapshot = Snapshot(
            id=str(uuid.uuid4()),
            account_id=mapping.account_id,
            source_system="1panel",
            source_ref=mapping.panel_website_id,
            subject_type="website_mapping",
            subject_id=mapping.id,
            snapshot_kind=kind,
            content_json=json.dumps(remote, ensure_ascii=False),
            content_hash=hashlib.sha256(json.dumps(remote, sort_keys=True).encode()).hexdigest(),
            captured_by_job_id=job_id,
        )
        self.db.add(snapshot)
        self.db.commit()
        return snapshot.id

    def _is_manual_override(self, mapping: WebsiteMapping) -> bool:
        if not mapping.manual_override_json:
            return False
        try:
            data = json.loads(mapping.manual_override_json)
            return bool(data.get("active", False))
        except json.JSONDecodeError:
            return False

    def _get_or_create_sync_state(self, mapping: WebsiteMapping) -> SyncState:
        sync_state = self.db.query(SyncState).filter(
            SyncState.subject_type == "website_mapping",
            SyncState.subject_id == mapping.id,
        ).first()
        if not sync_state:
            sync_state = SyncState(
                id=str(uuid.uuid4()),
                account_id=mapping.account_id,
                subject_type="website_mapping",
                subject_id=mapping.id,
                status="dirty",
                dirty=True,
            )
            self.db.add(sync_state)
            self.db.commit()
        return sync_state


def run_sync_job(db: Session, adapter: OnePanelAdapter, *, mapping_id: str | None = None, job_id: str | None = None) -> dict[str, Any]:
    service = WebsiteSyncService(db, adapter)

    async def _run() -> dict[str, Any]:
        if mapping_id:
            return await service.sync_single_mapping(mapping_id, job_id=job_id)
        return {"status": "ok", "results": await service.full_sync(job_id=job_id)}

    return asyncio.run(_run())
