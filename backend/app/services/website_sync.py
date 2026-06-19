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


def _normalize_domains(values: list[str]) -> list[str]:
    cleaned = [value.strip() for value in values if value and value.strip()]
    return sorted(dict.fromkeys(cleaned))


def _payload_hash(payload: dict[str, Any]) -> str:
    stable = json.dumps(payload, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(stable.encode()).hexdigest()


def _compute_delta(desired: dict[str, Any], observed: dict[str, Any]) -> dict[str, Any]:
    delta: dict[str, Any] = {}
    for key, value in desired.items():
        if observed.get(key) != value:
            delta[key] = value
    return delta


def _parse_https_port(http_config: Any) -> int | None:
    if isinstance(http_config, dict):
        listen = http_config.get("listen")
        return int(listen) if listen is not None else None
    if isinstance(http_config, str) and http_config:
        try:
            data = json.loads(http_config)
        except json.JSONDecodeError:
            return None
        return _parse_https_port(data)
    return None


class WebsiteSyncService:
    """Orchestrates incremental sync of website mappings to 1Panel."""

    def __init__(self, db: Session, adapter: OnePanelAdapter) -> None:
        self.db = db
        self.adapter = adapter

    async def sync_single_mapping(
        self,
        mapping_id: str,
        *,
        job_id: str | None = None,
    ) -> dict[str, Any]:
        mapping = self.db.query(WebsiteMapping).filter(WebsiteMapping.id == mapping_id).first()
        if not mapping:
            return {
                "status": "error",
                "code": "MAPPING_NOT_FOUND",
                "message": f"No such mapping: {mapping_id}",
            }

        desired_payload = self._build_desired_payload(mapping)
        desired_hash = _payload_hash(desired_payload)
        sync_state = self._get_or_create_sync_state(mapping)
        sync_state.last_job_id = job_id
        sync_state.last_attempt_at = datetime.utcnow()

        if self._is_manual_override(mapping):
            sync_state.status = "manual_override"
            sync_state.manual_override_at = datetime.utcnow()
            self.db.commit()
            return {"status": "skipped", "reason": "manual_override", "mapping_id": mapping_id}

        if (
            sync_state.desired_hash == desired_hash
            and sync_state.observed_hash == desired_hash
            and not sync_state.dirty
        ):
            sync_state.metadata_json = json.dumps({"last_result": "no_change"}, ensure_ascii=False)
            self.db.commit()
            return {"status": "ok", "result": "no_change", "mapping_id": mapping_id}

        sync_state.status = "running"
        self.db.commit()

        try:
            return await self._apply_sync(mapping, sync_state, desired_payload, desired_hash, job_id=job_id)
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

    async def full_sync(
        self,
        *,
        account_id: str | None = None,
        job_id: str | None = None,
    ) -> list[dict[str, Any]]:
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
        desired_payload: dict[str, Any],
        desired_hash: str,
        *,
        job_id: str | None,
    ) -> dict[str, Any]:
        if mapping.panel_website_id:
            observed = await self._fetch_remote_state(mapping.panel_website_id)
            if not _compute_delta(desired_payload, observed):
                self._mark_clean(
                    sync_state,
                    desired_hash,
                    observed,
                    mapping,
                    result="no_change",
                )
                return {"status": "ok", "result": "no_change", "mapping_id": mapping.id}

            await self._reconcile_remote_website(mapping.panel_website_id, desired_payload, observed)
        else:
            website_id = await self._create_remote_website(mapping, desired_payload)
            mapping.panel_website_id = website_id
            self.db.commit()
            observed = await self._fetch_remote_state(website_id)
            await self._reconcile_remote_website(website_id, desired_payload, observed)

        assert mapping.panel_website_id is not None
        observed_after = await self._fetch_remote_state(mapping.panel_website_id)
        remaining = _compute_delta(desired_payload, observed_after)
        if remaining:
            sync_state.status = "blocked"
            sync_state.dirty = True
            sync_state.last_error_code = "remote_not_converged"
            sync_state.last_error_message = json.dumps(remaining, ensure_ascii=False)
            sync_state.metadata_json = json.dumps(
                {"last_result": "blocked", "remaining_delta": remaining},
                ensure_ascii=False,
            )
            self.db.commit()
            return {
                "status": "error",
                "code": "REMOTE_NOT_CONVERGED",
                "mapping_id": mapping.id,
                "remaining_delta": remaining,
            }

        snapshot_id = await self._capture_snapshot(mapping, "post_sync", job_id=job_id)
        self._mark_clean(
            sync_state,
            desired_hash,
            observed_after,
            mapping,
            result="synced",
            snapshot_id=snapshot_id,
        )
        return {"status": "ok", "result": "synced", "mapping_id": mapping.id}

    def _build_desired_payload(self, mapping: WebsiteMapping) -> dict[str, Any]:
        domains: list[str]
        if mapping.domains_json:
            try:
                domains = json.loads(mapping.domains_json)
            except json.JSONDecodeError:
                domains = []
        else:
            domains = []

        payload: dict[str, Any] = {
            "alias": mapping.website_alias,
            "primaryDomain": mapping.primary_domain,
            "domains": _normalize_domains(domains),
            "proxyEnable": mapping.proxy_enabled,
            "proxyTarget": mapping.proxy_target,
            "proxyCache": mapping.proxy_cache_enabled,
            "httpsEnable": mapping.https_enabled,
        }
        if mapping.https_enabled and mapping.https_port and mapping.https_port != 443:
            payload["httpsPort"] = mapping.https_port
        return payload

    async def _create_remote_website(
        self,
        mapping: WebsiteMapping,
        payload: dict[str, Any],
    ) -> str:
        _ = mapping
        response = await self.adapter.create_website(payload)
        data = response.get("data", {}) if isinstance(response, dict) else {}
        website_id = data.get("id")
        if website_id is None:
            raise RuntimeError(f"1Panel did not return website id: {response}")
        return str(website_id)

    async def _reconcile_remote_website(
        self,
        website_id: str,
        desired: dict[str, Any],
        observed: dict[str, Any],
    ) -> None:
        base_changes = {
            key: desired[key]
            for key in ("alias", "primaryDomain")
            if observed.get(key) != desired.get(key)
        }
        if base_changes:
            await self.adapter.update_website(website_id, base_changes)

        await self._reconcile_domains(website_id, desired.get("domains", []), observed.get("domains", []))

        proxy_keys = ("proxyEnable", "proxyTarget", "proxyCache")
        if any(observed.get(key) != desired.get(key) for key in proxy_keys):
            await self.adapter.set_website_proxy(
                website_id,
                desired.get("proxyTarget") or "",
                enable=desired.get("proxyEnable", False),
                cache=desired.get("proxyCache", False),
            )

        https_keys = ("httpsEnable", "httpsPort")
        if any(observed.get(key) != desired.get(key) for key in https_keys):
            await self.adapter.set_website_https(
                website_id,
                enabled=desired.get("httpsEnable", False),
                port=desired.get("httpsPort"),
            )

    async def _reconcile_domains(
        self,
        website_id: str,
        desired_domains: list[str],
        observed_domains: list[str],
    ) -> None:
        desired_set = set(_normalize_domains(desired_domains))
        observed_set = set(_normalize_domains(observed_domains))
        for domain in sorted(desired_set - observed_set):
            await self.adapter.add_website_domain(website_id, domain)

    async def _fetch_remote_state(self, website_id: str) -> dict[str, Any]:
        website_response = await self.adapter.get_website(website_id)
        website_data = website_response.get("data", website_response) if isinstance(website_response, dict) else {}

        domain_entries = await self.adapter.get_website_domains(website_id)
        proxy_response = await self.adapter.get_website_proxy(website_id)
        proxy_data = proxy_response.get("data", proxy_response) if isinstance(proxy_response, dict) else {}
        https_response = await self.adapter.get_website_https(website_id)
        https_data = https_response.get("data", https_response) if isinstance(https_response, dict) else {}

        domains = _normalize_domains(
            [
                entry.get("domain") or entry.get("name") or entry.get("host") or ""
                for entry in domain_entries
                if isinstance(entry, dict)
            ]
        )
        https_port = (
            _parse_https_port(https_data.get("httpConfig"))
            or https_data.get("httpsPort")
            or website_data.get("httpsPort")
        )

        observed: dict[str, Any] = {
            "alias": website_data.get("alias"),
            "primaryDomain": website_data.get("primaryDomain"),
            "domains": domains,
            "proxyEnable": bool(proxy_data.get("enable", False)),
            "proxyTarget": proxy_data.get("proxyPass"),
            "proxyCache": bool(proxy_data.get("cache", False)),
            "httpsEnable": bool(https_data.get("enable", False)),
        }
        if https_port is not None:
            observed["httpsPort"] = int(https_port)
        return observed

    def _mark_clean(
        self,
        sync_state: SyncState,
        desired_hash: str,
        observed_payload: dict[str, Any],
        mapping: WebsiteMapping,
        *,
        result: str,
        snapshot_id: str | None = None,
    ) -> None:
        now = datetime.utcnow()
        observed_hash = _payload_hash(observed_payload)
        sync_state.desired_hash = desired_hash
        sync_state.observed_hash = observed_hash
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
        mapping.last_remote_hash = observed_hash
        self.db.commit()

    async def _capture_snapshot(
        self,
        mapping: WebsiteMapping,
        kind: str,
        *,
        job_id: str | None,
    ) -> str | None:
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
            content_hash=hashlib.sha256(
                json.dumps(remote, sort_keys=True).encode()
            ).hexdigest(),
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
        except json.JSONDecodeError:
            return False
        return bool(data.get("active", False))

    def _get_or_create_sync_state(self, mapping: WebsiteMapping) -> SyncState:
        sync_state = self.db.query(SyncState).filter(
            SyncState.subject_type == "website_mapping",
            SyncState.subject_id == mapping.id,
        ).first()
        if sync_state:
            return sync_state

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


def run_sync_job(
    db: Session,
    adapter: OnePanelAdapter,
    *,
    mapping_id: str | None = None,
    job_id: str | None = None,
) -> dict[str, Any]:
    service = WebsiteSyncService(db, adapter)

    async def _run() -> dict[str, Any]:
        if mapping_id:
            return await service.sync_single_mapping(mapping_id, job_id=job_id)
        return {"status": "ok", "results": await service.full_sync(job_id=job_id)}

    return asyncio.run(_run())
