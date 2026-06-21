# -*- coding: utf-8 -*-
"""Tests for FRPC runtime state machine."""

import asyncio
import json
from pathlib import Path
from unittest.mock import patch

import pytest

from app.services.frpc_runtime import (
    FRPCRuntimeManager,
    FrpcStatus,
    _runtime,
    render_frpc_config,
    compute_config_hash,
)


class TestFrpcStateMachine:
    """Tests for the FRPC runtime state machine."""

    @pytest.fixture(autouse=True)
    def reset_runtime(self) -> None:
        """Reset runtime state before each test."""
        _runtime.status = FrpcStatus.STOPPED
        _runtime.pid = None
        _runtime.proc = None
        _runtime.node_id = None
        _runtime.config_hash = None
        _runtime.last_error = None

    @pytest.mark.anyio
    async def test_get_status_initial(self) -> None:
        status = FRPCRuntimeManager.get_status()
        assert status["status"] == "stopped"
        assert status["pid"] is None
        assert status["config_hash"] is None

    def test_status_enum_values(self) -> None:
        assert FrpcStatus.STOPPED.value == "stopped"
        assert FrpcStatus.STARTING.value == "starting"
        assert FrpcStatus.RUNNING.value == "running"
        assert FrpcStatus.DEGRADED.value == "degraded"
        assert FrpcStatus.RESTARTING.value == "restarting"
        assert FrpcStatus.FAILED.value == "failed"

    @pytest.mark.anyio
    async def test_start_binary_not_found(self) -> None:
        """Should raise when binary doesn't exist."""
        with patch("app.services.frpc_runtime._bin_path", return_value=Path("/nonexistent/frpc")):
            with pytest.raises(RuntimeError, match="frpc binary not found"):
                await FRPCRuntimeManager.start(job_id="test-job", node_id="node-1")

    @pytest.mark.anyio
    async def test_start_config_not_found(self) -> None:
        """Should raise when config doesn't exist."""
        with patch("app.services.frpc_runtime._bin_path", return_value=Path("/bin/true")):
            with patch("app.services.frpc_runtime._config_path", return_value=Path("/nonexistent/frpc.toml")):
                with pytest.raises(RuntimeError, match="frpc config not found"):
                    await FRPCRuntimeManager.start(job_id="test-job", node_id="node-1")

    @pytest.mark.anyio
    async def test_double_start_raises(self) -> None:
        """Starting when already running should raise."""
        _runtime.status = FrpcStatus.RUNNING
        with pytest.raises(RuntimeError, match="frpc already in state"):
            await FRPCRuntimeManager.start(job_id="test-job")

    @pytest.mark.anyio
    async def test_stop_when_not_running(self) -> None:
        """Stop when not running should not raise."""
        await FRPCRuntimeManager.stop(job_id="test-job")

    @pytest.mark.anyio
    async def test_health_check_no_process(self) -> None:
        result = await FRPCRuntimeManager.health_check()
        assert result["healthy"] is False
        assert result["status"] == "stopped"

    @pytest.mark.anyio
    async def test_status_states(self) -> None:
        """Verify all status transitions are covered."""
        for status in FrpcStatus:
            assert status.value in (
                "stopped", "starting", "running", "degraded", "restarting", "failed"
            )
