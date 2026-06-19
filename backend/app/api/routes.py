# -*- coding: utf-8 -*-
"""API route handlers."""

from fastapi import APIRouter

from app.api.endpoints import jobs, nodes, overview, settings, sse, tunnels, website_mappings

api_router = APIRouter()

api_router.include_router(overview.router, prefix="/overview", tags=["overview"])
api_router.include_router(settings.router, prefix="/settings", tags=["settings"])
api_router.include_router(nodes.router, prefix="/nodes", tags=["nodes"])
api_router.include_router(tunnels.router, prefix="/tunnels", tags=["tunnels"])
api_router.include_router(website_mappings.router, prefix="/website-mappings", tags=["website-mappings"])
api_router.include_router(jobs.router, prefix="/jobs", tags=["jobs"])
api_router.include_router(sse.router, prefix="/sse", tags=["sse"])
