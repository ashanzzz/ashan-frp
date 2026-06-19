# -*- coding: utf-8 -*-
"""Common Pydantic schemas."""

from datetime import datetime
from typing import Any

from pydantic import BaseModel


class HealthResponse(BaseModel):
    status: str


class VersionResponse(BaseModel):
    version: str
    name: str
