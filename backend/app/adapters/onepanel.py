# -*- coding: utf-8 -*-
"""1Panel API adapter — high-level wrapper over OnePanelClient.

Maps 1Panel DTOs to our normalized shapes. No business state is kept here;
no sync decisions are made here. Thin as a piece of paper.
"""

from __future__ import annotations

from typing import Any

from app.adapters.onepanel_client import OnePanelClient


class OnePanelAdapter:
    """High-level adapter for 1Panel API calls."""

    def __init__(self, client: OnePanelClient | None = None) -> None:
        self.client = client

    async def test_connection(self) -> dict:
        return await self.client.get("/health/check")  # type: ignore[union-attr]

    async def list_containers(self, page: int = 1, page_size: int = 100) -> dict:
        payload = {
            "page": page,
            "pageSize": page_size,
            "name": "",
            "state": "all",
            "filters": "",
            "orderBy": "created_at",
            "order": "null",
        }
        return await self.client.post("/containers/search", json=payload)  # type: ignore[union-attr]

    async def get_container(self, container_name: str) -> dict:
        return await self.client.post("/containers/info", json={"name": container_name})  # type: ignore[union-attr]

    async def list_container_names(self) -> list[str]:
        data = await self.list_containers()
        items = data.get("data", {}).get("items", [])
        return [c.get("name", "") for c in items if c]

    async def list_websites(self, page: int = 1, page_size: int = 100) -> dict:
        payload = {
            "page": page,
            "pageSize": page_size,
            "name": "",
            "websiteGroupId": 0,
        }
        return await self.client.post("/websites/search", json=payload)  # type: ignore[union-attr]

    async def get_website(self, website_id: str) -> dict:
        return await self.client.get(f"/websites/{website_id}")  # type: ignore[union-attr]

    async def create_website(self, payload: dict[str, Any]) -> dict:
        return await self.client.post("/websites", json=payload)  # type: ignore[union-attr]

    async def update_website(self, website_id: str, payload: dict[str, Any]) -> dict:
        body = {"id": int(website_id), **payload}
        return await self.client.post("/websites/update", json=body)  # type: ignore[union-attr]

    async def delete_website(self, website_id: str) -> dict:
        payload = {"id": int(website_id), "deleteApp": False, "forceDelete": False}
        return await self.client.post("/websites/del", json=payload)  # type: ignore[union-attr]

    async def get_website_domains(self, website_id: str) -> list[dict]:
        resp = await self.client.get(f"/websites/domains/{website_id}")  # type: ignore[union-attr]
        data = resp.get("data", resp) if isinstance(resp, dict) else resp
        return data if isinstance(data, list) else []

    async def add_website_domain(self, website_id: str, domain: str) -> dict:
        payload = {"websiteID": int(website_id), "domains": [domain]}
        return await self.client.post("/websites/domains", json=payload)  # type: ignore[union-attr]

    async def remove_website_domain(self, website_id: str, domain_id: str) -> dict:
        _ = website_id
        return await self.client.post("/websites/domains/del", json={"id": int(domain_id)})  # type: ignore[union-attr]

    async def get_website_proxy(self, website_id: str) -> dict:
        resp = await self.client.post("/websites/proxies", json={"websiteID": int(website_id)})  # type: ignore[union-attr]
        return resp if isinstance(resp, dict) else {}

    async def set_website_proxy(self, website_id: str, target: str, **extra: Any) -> dict:
        payload: dict[str, Any] = {
            "websiteID": int(website_id),
            "name": extra.get("name", "default"),
            "enable": extra.get("enable", True),
            "proxyPass": target,
        }
        for key in ("path", "proxyType", "cache", "remark"):
            if key in extra:
                payload[key] = extra[key]
        return await self.client.post("/websites/proxies/update", json=payload)  # type: ignore[union-attr]

    async def get_website_https(self, website_id: str) -> dict:
        resp = await self.client.get(f"/websites/{website_id}/https")  # type: ignore[union-attr]
        return resp if isinstance(resp, dict) else {}

    async def set_website_https(
        self,
        website_id: str,
        *,
        enabled: bool,
        port: int | None = None,
        cert_type: str | None = None,
        cert_id: str | None = None,
        **extra: Any,
    ) -> dict:
        payload: dict[str, Any] = {
            "websiteId": int(website_id),
            "enable": enabled,
        }
        if port is not None:
            payload["httpConfig"] = json_dumps_compact({"listen": port})
        if cert_type is not None:
            payload["type"] = cert_type
        if cert_id is not None:
            payload["websiteSSLId"] = cert_id
        payload.update(extra)
        return await self.client.post(f"/websites/{website_id}/https", json=payload)  # type: ignore[union-attr]


def json_dumps_compact(value: dict[str, Any]) -> str:
    import json

    return json.dumps(value, ensure_ascii=False, separators=(",", ":"))
