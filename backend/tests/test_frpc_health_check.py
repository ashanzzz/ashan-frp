# -*- coding: utf-8 -*-
"""Tests for frpc health check."""

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
async def test_health_check_no_process():
    """Health check returns unhealthy when no process is running."""
    result = await FRPCRuntimeManager.health_check()
    assert result["healthy"] is False
    assert result["status"] == "stopped"


@pytest.mark.asyncio
async def test_health_check_process_alive():
    """Health check returns healthy when process is alive."""
    mock_proc = AsyncMock()
    mock_proc.returncode = None
    _runtime.proc = mock_proc
    _runtime.status = FrpcStatus.RUNNING
    _runtime.last_heartbeat_at = __import__("datetime").datetime.utcnow()

    with patch.object(FRPCRuntimeManager, "_scan_recent_log_errors", return_value=False), \
         patch.object(FRPCRuntimeManager, "_check_config_hash_consistency", return_value=True):
        result = await FRPCRuntimeManager.health_check()

    assert result["healthy"] is True
    assert result["status"] == "running"


@pytest.mark.asyncio
async def test_health_check_process_exited():
    """Health check returns unhealthy when process has exited."""
    mock_proc = AsyncMock()
    mock_proc.returncode = 1
    _runtime.proc = mock_proc
    _runtime.status = FrpcStatus.RUNNING

    result = await FRPCRuntimeManager.health_check()
    assert result["healthy"] is False


@pytest.mark.asyncio
async def test_health_check_log_errors():
    """Health check detects log errors."""
    mock_proc = AsyncMock()
    mock_proc.returncode = None
    _runtime.proc = mock_proc
    _runtime.status = FrpcStatus.RUNNING
    _runtime.last_heartbeat_at = __import__("datetime").datetime.utcnow()

    with patch.object(FRPCRuntimeManager, "_scan_recent_log_errors", return_value=True), \
         patch.object(FRPCRuntimeManager, "_check_config_hash_consistency", return_value=True):
        result = await FRPCRuntimeManager.health_check()

    assert result["healthy"] is False
    assert result["status"] == "degraded"


@pytest.mark.asyncio
async def test_health_check_config_hash_mismatch():
    """Health check detects config hash mismatch."""
    mock_proc = AsyncMock()
    mock_proc.returncode = None
    _runtime.proc = mock_proc
    _runtime.status = FrpcStatus.RUNNING
    _runtime.last_heartbeat_at = __import__("datetime").datetime.utcnow()

    with patch.object(FRPCRuntimeManager, "_scan_recent_log_errors", return_value=False), \
         patch.object(FRPCRuntimeManager, "_check_config_hash_consistency", return_value=False):
        result = await FRPCRuntimeManager.health_check()

    assert result["healthy"] is False


@pytest.mark.asyncio
async def test_health_check_stale_heartbeat():
    """Health check detects stale heartbeat."""
    from datetime import datetime, timedelta
    mock_proc = AsyncMock()
    mock_proc.returncode = None
    _runtime.proc = mock_proc
    _runtime.status = FrpcStatus.RUNNING
    _runtime.last_heartbeat_at = datetime.utcnow() - timedelta(seconds=300)

    with patch.object(FRPCRuntimeManager, "_scan_recent_log_errors", return_value=False), \
         patch.object(FRPCRuntimeManager, "_check_config_hash_consistency", return_value=True):
        result = await FRPCRuntimeManager.health_check()

    assert result["healthy"] is False
    assert result["signals"]["heartbeat_fresh"] is False


@pytest.mark.asyncio
async def test_scan_recent_log_errors_no_file():
    """_scan_recent_log_errors returns False when no log file exists."""
    result = await FRPCRuntimeManager._scan_recent_log_errors()
    assert result is False


@pytest.mark.asyncio
async def test_check_config_hash_no_file():
    """_check_config_hash_consistency returns True when no config file and stopped."""
    _runtime.status = FrpcStatus.STOPPED
    _runtime.config_hash = None
    result = await FRPCRuntimeManager._check_config_hash_consistency()
    assert result is True
