# -*- coding: utf-8 -*-
"""Regression tests for the 1Panel HTTP client and adapter route mapping."""

from __future__ import annotations

import hashlib

import pytest

from app.adapters.onepanel import OnePanelAdapter
from app.adapters.onepanel_client import OnePanelClient


class FakeResponse:
    def __init__(self, payload: dict):
        self.payload = payload

    def raise_for_status(self) -> None:
        return None

    def json(self) -> dict:
        return self.payload


class RecordingAsyncClient:
    def __init__(self, payload: dict | None = None) -> None:
        self.payload = payload or {"status": "ok"}
        self.calls: list[dict] = []

    async def request(self, method: str, url: str, **kwargs):
        self.calls.append({"method": method, "url": url, **kwargs})
        return FakeResponse(self.payload)

    async def aclose(self) -> None:
        return None


class RecordingOnePanelClient:
    def __init__(self) -> None:
        self.calls: list[tuple[str, str, dict]] = []

    async def get(self, path: str, *, params: dict | None = None):
        self.calls.append(("GET", path, {"params": params}))
        return {"data": {"id": 7}}

    async def post(self, path: str, *, json: dict | None = None):  # noqa: A002
        self.calls.append(("POST", path, {"json": json}))
        return {"data": {"items": [], "id": 7}}


@pytest.mark.asyncio
async def test_onepanel_client_uses_api_v2_and_header_signature(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr("app.adapters.onepanel_client.time.time", lambda: 1_700_000_000)
    transport = RecordingAsyncClient({"ok": True})
    client = OnePanelClient("http://panel.example/", "secret")
    client._client = transport  # test seam for the underlying httpx client

    result = await client.get("/health/check")

    assert result == {"ok": True}
    assert len(transport.calls) == 1
    call = transport.calls[0]
    assert call["method"] == "GET"
    assert call["url"] == "http://panel.example/api/v2/health/check"
    assert "signature=" not in call["url"]
    assert "t=" not in call["url"]
    assert call["headers"]["1Panel-Timestamp"] == "1700000000"
    expected_token = hashlib.md5(b"1panelsecret1700000000").hexdigest()  # noqa: S324
    assert call["headers"]["1Panel-Token"] == expected_token


@pytest.mark.asyncio
async def test_onepanel_adapter_uses_real_1panel_container_and_website_routes() -> None:
    client = RecordingOnePanelClient()
    adapter = OnePanelAdapter(client=client)

    await adapter.test_connection()
    await adapter.list_containers(page=2, page_size=50)
    await adapter.get_container("my-container")
    await adapter.list_websites(page=3, page_size=25)
    await adapter.update_website("42", {"alias": "example"})
    await adapter.delete_website("42")
    await adapter.get_website_domains("42")
    await adapter.add_website_domain("42", "www.example.com")
    await adapter.remove_website_domain("42", "99")
    await adapter.set_website_proxy("42", "http://127.0.0.1:8080", name="default")

    assert client.calls == [
        ("GET", "/health/check", {"params": None}),
        (
            "POST",
            "/containers/search",
            {
                "json": {
                    "page": 2,
                    "pageSize": 50,
                    "name": "",
                    "state": "all",
                    "filters": "",
                    "orderBy": "created_at",
                    "order": "null",
                }
            },
        ),
        ("POST", "/containers/info", {"json": {"name": "my-container"}}),
        (
            "POST",
            "/websites/search",
            {"json": {"page": 3, "pageSize": 25, "name": "", "websiteGroupId": 0}},
        ),
        ("POST", "/websites/update", {"json": {"id": 42, "alias": "example"}}),
        ("POST", "/websites/del", {"json": {"id": 42, "deleteApp": False, "forceDelete": False}}),
        ("GET", "/websites/domains/42", {"params": None}),
        ("POST", "/websites/domains", {"json": {"websiteID": 42, "domains": ["www.example.com"]}}),
        ("POST", "/websites/domains/del", {"json": {"id": 99}}),
        (
            "POST",
            "/websites/proxies/update",
            {
                "json": {
                    "websiteID": 42,
                    "name": "default",
                    "enable": True,
                    "proxyPass": "http://127.0.0.1:8080",
                }
            },
        ),
    ]
