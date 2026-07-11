"""Shared fixtures for qtcloud-auth integration tests."""

from __future__ import annotations

from collections.abc import AsyncIterator

import httpx
import os

import pytest


@pytest.fixture
def base_url() -> str:
    """Base URL of the running qtcloud-auth service.
    Override via TEST_BASE_URL env var."""
    return os.environ.get("TEST_BASE_URL", "http://localhost:8080")


@pytest.fixture
async def client(base_url: str) -> AsyncIterator[httpx.AsyncClient]:
    """HTTP client pre-configured for the qtcloud-auth service."""
    async with httpx.AsyncClient(base_url=base_url) as c:
        yield c


@pytest.fixture
async def access_token(client: httpx.AsyncClient) -> str:
    """Obtain a valid access token via password grant."""
    resp = await client.post(
        "/oauth/token",
        data={
            "grant_type": "password",
            "username": "admin",
            "password": "123456",
        },
    )
    resp.raise_for_status()
    return resp.json()["access_token"]


@pytest.fixture
async def refresh_token(client: httpx.AsyncClient) -> str:
    """Obtain a valid refresh token via password grant."""
    resp = await client.post(
        "/oauth/token",
        data={
            "grant_type": "password",
            "username": "admin",
            "password": "123456",
        },
    )
    resp.raise_for_status()
    return resp.json()["refresh_token"]


# Allow test_oauth to use 'token' as alias for 'refresh_token'
@pytest.fixture
async def token(refresh_token: str) -> str:
    return refresh_token
