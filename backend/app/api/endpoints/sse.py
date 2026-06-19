# -*- coding: utf-8 -*-
"""SSE (Server-Sent Events) endpoint skeleton."""

import asyncio
import json
import time
from datetime import datetime

from fastapi import APIRouter, Request
from fastapi.responses import StreamingResponse

router = APIRouter()


async def _event_generator(request: Request):
    """Generate SSE events for the given channel."""
    count = 0
    while True:
        if await request.is_disconnected():
            break
        event = {
            "schema_version": 1,
            "channel": "health.system",
            "kind": "health.ok",
            "cursor": f"v1|health|{int(time.time())}|{count}",
            "level": "info",
            "message": "System heartbeat",
            "payload": {"sequence": count},
            "created_at": datetime.utcnow().isoformat(),
        }
        yield f"data: {json.dumps(event)}\n\n"
        count += 1
        await asyncio.sleep(5)


@router.get("/{channel}")
def sse_stream(channel: str, request: Request) -> StreamingResponse:
    """SSE endpoint for real-time event stream."""
    return StreamingResponse(
        _event_generator(request),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
        },
    )
