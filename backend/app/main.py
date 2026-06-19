# -*- coding: utf-8 -*-
"""Ashan-FRP Backend - Application entry point."""

from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.api.routes import api_router
from app.core.config import settings
from app.db.session import engine, init_db
from app.services.job_runner import JobRunner


def create_application() -> FastAPI:
    job_runner = JobRunner()

    @asynccontextmanager
    async def lifespan(app: FastAPI):  # noqa: ARG001  # pragma: no cover lifeline
        # Startup
        init_db()
        await job_runner.start()
        yield
        # Shutdown
        await job_runner.stop()

    app = FastAPI(
        title="Ashan-FRP Backend",
        description="Tunnel management backend with FastAPI + SQLite + SSE",
        version="0.1.0",
        lifespan=lifespan,
    )

    app.add_middleware(
        CORSMiddleware,
        allow_origins=settings.ALLOWED_ORIGINS,
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    app.include_router(api_router, prefix="/api/v1")

    @app.get("/health")
    async def health() -> dict:
        return {"status": "ok"}

    @app.get("/version")
    async def version() -> dict:
        return {"version": "0.1.0", "name": "ashan-frp-backend"}

    return app


app = create_application()
