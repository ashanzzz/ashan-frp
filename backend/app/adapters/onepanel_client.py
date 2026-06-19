# -*- coding: utf-8 -*-
"""1Panel signed HTTP client."""

from __future__ import annotations

import hashlib
import time
from typing import Any

import httpx


class OnePanelClient:
    """Signed HTTP client for the 1Panel v2 OpenAPI.

    1Panel validates API requests with two headers::

        1Panel-Timestamp: <unix timestamp>
        1Panel-Token: md5("1panel" + api_key + timestamp)

    The API key itself is never sent as the token value.
    """

    def __init__(self, base_url: str, api_key: str, *, timeout: float = 30.0) -> None:
        cleaned = base_url.rstrip("/")
        self.base_url = cleaned[:-7] if cleaned.endswith("/api/v2") else cleaned
        self.api_key = api_key
        self.timeout = timeout
        self._client: httpx.AsyncClient | None = httpx.AsyncClient(timeout=timeout)

    @staticmethod
    def _sign(api_key: str, timestamp: int) -> str:
        return hashlib.md5(f"1panel{api_key}{timestamp}".encode()).hexdigest()  # noqa: S324

    def _build_url(self, path: str) -> str:
        normalized_path = path if path.startswith("/") else f"/{path}"
        return f"{self.base_url}/api/v2{normalized_path}"

    def _auth_headers(self) -> dict[str, str]:
        timestamp = int(time.time())
        return {
            "Content-Type": "application/json",
            "1Panel-Timestamp": str(timestamp),
            "1Panel-Token": self._sign(self.api_key, timestamp),
        }

    async def request(self, method: str, path: str, **kwargs: Any) -> dict[str, Any]:
        """Make a signed HTTP request to the 1Panel API."""
        if self._client is None:
            raise RuntimeError("Client is closed. Use as async context manager or call close().")
        response = await self._client.request(
            method,
            self._build_url(path),
            headers=self._auth_headers(),
            **kwargs,
        )
        response.raise_for_status()
        return response.json()

    async def get(self, path: str, *, params: dict[str, Any] | None = None) -> dict[str, Any]:
        return await self.request("GET", path, params=params)

    async def post(self, path: str, *, json: dict[str, Any] | None = None) -> dict[str, Any]:  # noqa: A002
        return await self.request("POST", path, json=json)

    async def close(self) -> None:
        if self._client:
            await self._client.aclose()
            self._client = None

    async def __aenter__(self) -> OnePanelClient:
        return self

    async def __aexit__(self, *args: Any) -> None:  # noqa: ANN401
        await self.close()
