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
                "username": "admin",
                "password": "123456",
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
                "username": "admin",
                "password": "wrongpass",
            },
        )
        assert resp.status_code == 401

    @pytest.mark.integration
    async def test_sms_code_grant(self, client: httpx.AsyncClient):
        # 先发验证码（SMS_TEST_CODE=123456 时验证码固定）
        resp_send = await client.post("/oauth/sms/send", json={"phone": "13900139000"})
        assert resp_send.status_code == 200

        resp = await client.post(
            "/oauth/token",
            data={
                "grant_type": "sms_code",
                "phone": "13900139000",
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


# ── Register（用户名/密码注册） ──

class TestRegisterEndpoint:
    @pytest.mark.integration
    async def test_register_success_and_login(self, client: httpx.AsyncClient):
        resp = await client.post(
            "/oauth/register",
            json={"username": "e2e_user", "password": "secret1", "email": "e2e@example.com"},
        )
        assert resp.status_code == 200
        assert "access_token" in resp.json()

        # 注册后即可用密码登录（用户名）
        login = await client.post(
            "/oauth/token",
            data={"grant_type": "password", "username": "e2e_user", "password": "secret1"},
        )
        assert login.status_code == 200
        assert "access_token" in login.json()

        # 也可用邮箱登录
        login_email = await client.post(
            "/oauth/token",
            data={"grant_type": "password", "username": "e2e@example.com", "password": "secret1"},
        )
        assert login_email.status_code == 200

    @pytest.mark.integration
    async def test_register_duplicate(self, client: httpx.AsyncClient):
        payload = {"username": "dup_user", "password": "secret1"}
        first = await client.post("/oauth/register", json=payload)
        assert first.status_code == 200
        second = await client.post("/oauth/register", json=payload)
        assert second.status_code == 409

    @pytest.mark.integration
    async def test_register_validation(self, client: httpx.AsyncClient):
        cases = [
            ({"username": "x", "password": "secret1"}, 400),          # username 太短
            ({"username": "valid_name", "password": "123"}, 400),     # password 太短
            ({"username": "bad_email", "password": "secret1", "email": "nope"}, 400),
            ({"username": "bad_phone", "password": "secret1", "phone": "123"}, 400),
        ]
        for payload, want in cases:
            resp = await client.post("/oauth/register", json=payload)
            assert resp.status_code == want, f"{payload}: {resp.status_code} != {want}"
