"""Integration tests for the OAuth 2.0 token endpoint."""

from __future__ import annotations

import httpx
import pytest


class TestTokenEndpoint:
    """/oauth/token — unified authentication endpoint."""

    @pytest.mark.integration
    async def test_password_grant_success(self, client: httpx.AsyncClient):
        resp = await client.post(
            "/oauth/token",
            data={
                "grant_type": "password",
                "username": "testuser",
                "password": "testpass",
            },
        )
        assert resp.status_code == 200
        body = resp.json()
        assert "access_token" in body
        assert body["token_type"] == "Bearer"
        assert body["expires_in"] > 0
        assert "refresh_token" in body

    @pytest.mark.integration
    async def test_password_grant_invalid_credentials(self, client: httpx.AsyncClient):
        resp = await client.post(
            "/oauth/token",
            data={
                "grant_type": "password",
                "username": "testuser",
                "password": "wrongpass",
            },
        )
        assert resp.status_code == 401

    @pytest.mark.integration
    async def test_sms_code_grant(self, client: httpx.AsyncClient):
        resp = await client.post(
            "/oauth/token",
            data={
                "grant_type": "sms_code",
                "phone": "13800138000",
                "code": "123456",
            },
        )
        assert resp.status_code == 200
        body = resp.json()
        assert "access_token" in body

    @pytest.mark.integration
    async def test_refresh_token_grant(self, client: httpx.AsyncClient, token: str):
        resp = await client.post(
            "/oauth/token",
            data={
                "grant_type": "refresh_token",
                "refresh_token": token,
            },
        )
        assert resp.status_code == 200
        body = resp.json()
        assert "access_token" in body

    @pytest.mark.integration
    async def test_unsupported_grant_type(self, client: httpx.AsyncClient):
        resp = await client.post(
            "/oauth/token",
            data={"grant_type": "client_credentials"},
        )
        assert resp.status_code == 400


class TestSmsEndpoint:
    """/oauth/sms/send — send verification code."""

    @pytest.mark.integration
    async def test_send_sms_success(self, client: httpx.AsyncClient):
        resp = await client.post(
            "/oauth/sms/send",
            json={"phone": "13800138000"},
        )
        assert resp.status_code == 200
        assert resp.json() == {"message": "code sent"}

    @pytest.mark.integration
    async def test_send_sms_missing_phone(self, client: httpx.AsyncClient):
        resp = await client.post("/oauth/sms/send", json={})
        assert resp.status_code == 400

    @pytest.mark.integration
    async def test_send_sms_invalid_phone(self, client: httpx.AsyncClient):
        resp = await client.post(
            "/oauth/sms/send",
            json={"phone": "invalid"},
        )
        assert resp.status_code == 400


class TestUserInfoEndpoint:
    """/userinfo — OIDC user info."""

    @pytest.mark.integration
    async def test_userinfo_success(self, client: httpx.AsyncClient, access_token: str):
        resp = await client.get(
            "/userinfo",
            headers={"Authorization": f"Bearer {access_token}"},
        )
        assert resp.status_code == 200
        body = resp.json()
        assert "sub" in body
        assert "phone" in body

    @pytest.mark.integration
    async def test_userinfo_no_token(self, client: httpx.AsyncClient):
        resp = await client.get("/userinfo")
        assert resp.status_code == 401

    @pytest.mark.integration
    async def test_userinfo_invalid_token(self, client: httpx.AsyncClient):
        resp = await client.get(
            "/userinfo",
            headers={"Authorization": "Bearer invalid-token"},
        )
        assert resp.status_code == 401
