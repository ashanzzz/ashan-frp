# -*- coding: utf-8 -*-
"""Tests for frpc failover: switch_node and recover."""

import pytest
from unittest.mock import AsyncMock, MagicMock, patch
from app.services.frpc_runtime import FRPCRuntimeManager, FrpcStatus, _runtime


@pytest.fixture(autouse=True)
def reset_runtime():
    """Reset runtime state between tests."""
    _runtime.status = FrpcStatus.STOPPED
    _runtime.pid = None
    _runtime.proc = None
    _runtime.node_id = None
    _runtime.config_hash = None
    _runtime.started_at = None
    _runtime.stopped_at = None
    _runtime.last_check_at = None
    _runtime.last_error = None
    _runtime.last_heartbeat_at = None
    _runtime.recovery_attempts = 0
    _runtime.recovery_blocked = False
    yield


@pytest.mark.asyncio
async def test_switch_node_validates_node_exists():
    """switch_node raises when node doesn't exist."""
    with patch("app.services.frpc_runtime.SessionLocal") as mock_session:
        mock_db = MagicMock()
        mock_session.return_value = mock_db
        mock_db.query.return_value.filter.return_value.first.return_value = None

        with pytest.raises(RuntimeError, match="Node not found"):
            await FRPCRuntimeManager.switch_node("nonexistent")


@pytest.mark.asyncio
async def test_switch_node_validates_node_active():
    """switch_node raises when node is not active."""
    mock_node = MagicMock()
    mock_node.status = "inactive"

    with patch("app.services.frpc_runtime.SessionLocal") as mock_session:
        mock_db = MagicMock()
        mock_session.return_value = mock_db
        mock_db.query.return_value.filter.return_value.first.return_value = mock_node

        with pytest.raises(RuntimeError, match="not active"):
            await FRPCRuntimeManager.switch_node("node-1")


@pytest.mark.asyncio
async def test_switch_node_stops_running_before_switch():
    """switch_node stops current process before switching."""
    mock_node = MagicMock()
    mock_node.id = "node-2"
    mock_node.status = "active"

    mock_proc = AsyncMock()
    mock_proc.returncode = None
    _runtime.proc = mock_proc
    _runtime.status = FrpcStatus.RUNNING
    _runtime.node_id = "node-1"

    with patch("app.services.frpc_runtime.SessionLocal") as mock_session:
        mock_db = MagicMock()
        mock_session.return_value = mock_db
        mock_db.query.return_value.filter.return_value.first.return_value = mock_node

        with patch.object(FRPCRuntimeManager, "stop", new_callable=AsyncMock) as mock_stop, \
             patch.object(FRPCRuntimeManager, "start", new_callable=AsyncMock) as mock_start:
            await FRPCRuntimeManager.switch_node("node-2")

            mock_stop.assert_called_once()
            mock_start.assert_called_once()
            assert _runtime.node_id == "node-2"


@pytest.mark.asyncio
async def test_recover_healthy_skips():
    """recover returns immediately when already healthy."""
    with patch.object(FRPCRuntimeManager, "health_check", new_callable=AsyncMock) as mock_hc:
        mock_hc.return_value = {"healthy": True, "status": "running"}
        result = await FRPCRuntimeManager.recover()

        assert result["status"] == "healthy"
        assert result["action"] == "none"


@pytest.mark.asyncio
async def test_recover_restart_succeeds():
    """recover tries restart first when unhealthy."""
    _runtime.node_id = "node-1"

    with patch.object(FRPCRuntimeManager, "health_check", new_callable=AsyncMock) as mock_hc, \
         patch.object(FRPCRuntimeManager, "restart", new_callable=AsyncMock) as mock_restart:
        # First call: unhealthy, second call (after restart): healthy
        mock_hc.side_effect = [
            {"healthy": False, "status": "failed"},
            {"healthy": True, "status": "running"},
        ]

        result = await FRPCRuntimeManager.recover()

        mock_restart.assert_called_once()
        assert result["status"] == "recovered"
        assert result["action"] == "restart"


@pytest.mark.asyncio
async def test_recover_blocks_after_max_attempts():
    """recover blocks after exceeding max_recovery_attempts."""
    _runtime.recovery_attempts = 3

    with patch.object(FRPCRuntimeManager, "health_check", new_callable=AsyncMock) as mock_hc:
        mock_hc.return_value = {"healthy": False, "status": "failed"}

        with pytest.raises(RuntimeError, match="Recovery blocked"):
            await FRPCRuntimeManager.recover(max_recovery_attempts=3)

        assert _runtime.recovery_blocked is True


@pytest.mark.asyncio
async def test_recover_blocked_raises():
    """recover raises when already blocked."""
    _runtime.recovery_blocked = True

    with pytest.raises(RuntimeError, match="Recovery blocked"):
        await FRPCRuntimeManager.recover(max_recovery_attempts=3)


@pytest.mark.asyncio
async def test_recover_tries_switch_node_on_restart_failure():
    """recover tries switching node when restart fails."""
    _runtime.node_id = "node-1"
    _runtime.status = FrpcStatus.FAILED

    mock_alt_node = MagicMock()
    mock_alt_node.id = "node-2"
    mock_alt_node.status = "active"

    with patch.object(FRPCRuntimeManager, "health_check", new_callable=AsyncMock) as mock_hc, \
         patch.object(FRPCRuntimeManager, "restart", new_callable=AsyncMock) as mock_restart, \
         patch.object(FRPCRuntimeManager, "switch_node", new_callable=AsyncMock) as mock_switch, \
         patch("app.services.frpc_runtime.SessionLocal") as mock_session:
        mock_restart.side_effect = RuntimeError("restart failed")
        mock_hc.return_value = {"healthy": False, "status": "failed"}
        mock_switch.return_value = {"status": "switched", "node_id": "node-2"}

        mock_db = MagicMock()
        mock_session.return_value = mock_db
        mock_db.query.return_value.filter.return_value.first.return_value = mock_alt_node

        result = await FRPCRuntimeManager.recover()

        mock_restart.assert_called_once()
        mock_switch.assert_called_once_with("node-2", job_id=None)
        assert result["status"] == "switched"
