package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/store"
	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/quanttide/qtcloud-auth/model"
)

// ── Mocks ──

type mockStorer struct {
	users     []model.User
	listErr   error
	createErr error
	getErr    error
	updateErr error
}

func (m *mockStorer) List(string) ([]byte, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return json.Marshal(m.users)
}

func (m *mockStorer) Create(_ string, data []byte) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	var u model.User
	if err := json.Unmarshal(data, &u); err != nil {
		return "", err
	}
	u.ID = fmt.Sprintf("u%d", len(m.users)+1)
	m.users = append(m.users, u)
	return u.ID, nil
}

func (m *mockStorer) Get(_ string, id string) ([]byte, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, u := range m.users {
		if u.ID == id {
			return json.Marshal(u)
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockStorer) Update(_ string, id string, data []byte) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	for i := range m.users {
		if m.users[i].ID == id {
			return json.Unmarshal(data, &m.users[i])
		}
	}
	return fmt.Errorf("not found")
}

type mockSMSSender struct{ err error }

func (m *mockSMSSender) Send(_ context.Context, _, _ string) error {
	return m.err
}

// corruptStorer 模拟 List 返回不可解析数据.
type corruptStorer struct{}

func (c *corruptStorer) List(string) ([]byte, error) { return []byte(`not json`), nil }

func (c *corruptStorer) Create(string, []byte) (string, error) { return "", nil }

func (c *corruptStorer) Get(string, string) ([]byte, error) { return []byte(`not json`), nil }

func (c *corruptStorer) Update(string, string, []byte) error { return nil }

// ── 辅助 ──

func newTestHandler() *AuthHandler {
	h := NewAuthHandler(&mockStorer{}, "test-secret", &mockSMSSender{})
	h.SetupOAuth()
	return h
}

func loginGetToken(t *testing.T, h *AuthHandler) string {
	t.Helper()
	_ = h.EnsureAdmin("admin")
	form := url.Values{"grant_type": {"password"}, "username": {"admin"}, "password": {"admin"}}
	r := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Token(w, r)

	var resp struct {
		AccessToken string `json:"access_token"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.AccessToken == "" {
		t.Fatal("loginGetToken: no access token")
	}
	return resp.AccessToken
}

// getRefreshToken 登录并返回 refresh_token.
func getRefreshToken(t *testing.T, h *AuthHandler) string {
	t.Helper()
	_ = h.EnsureAdmin("admin")
	form := url.Values{"grant_type": {"password"}, "username": {"admin"}, "password": {"admin"}}
	r := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Token(w, r)

	var resp struct {
		RefreshToken string `json:"refresh_token"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	return resp.RefreshToken
}

// ── 单元测试：工具函数 ──

func TestIsValidPhone(t *testing.T) {
	tests := []struct {
		phone string
		want  bool
	}{
		{"13800138000", true},
		{"15912345678", true},
		{"10000000000", false},
		{"1380013800", false},
		{"138001380000", false},
		{"", false},
		{"abc", false},
		{"13800a13800", false},
	}
	for _, tt := range tests {
		got := isValidPhone(tt.phone)
		if got != tt.want {
			t.Errorf("isValidPhone(%q) = %v, want %v", tt.phone, got, tt.want)
		}
	}
}

func TestMaskPhone(t *testing.T) {
	tests := []struct {
		phone string
		want  string
	}{
		{"13800138000", "138****8000"},
		{"12345", "12345"},
		{"", ""},
	}
	for _, tt := range tests {
		got := maskPhone(tt.phone)
		if got != tt.want {
			t.Errorf("maskPhone(%q) = %q, want %q", tt.phone, got, tt.want)
		}
	}
}

func TestGenerateCode(t *testing.T) {
	code, err := generateCode()
	if err != nil {
		t.Fatalf("generateCode() error: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("len(code) = %d, want 6", len(code))
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			t.Errorf("non-digit %c in code", c)
		}
	}
}

func TestHashPassword(t *testing.T) {
	h1 := hashPassword("admin", "secret")
	h2 := hashPassword("admin", "secret")
	h3 := hashPassword("admin", "wrong")
	if h1 != h2 {
		t.Error("same input must produce same hash")
	}
	if h1 == h3 {
		t.Error("different passwords must produce different hashes")
	}
	if len(h1) != 64 {
		t.Errorf("hash len = %d, want 64", len(h1))
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name string
		auth string
		want string
	}{
		{"valid", "Bearer mytoken123", "mytoken123"},
		{"no prefix", "Basic xyz", ""},
		{"empty", "", ""},
		{"short", "Bea", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("Authorization", tt.auth)
			got := extractBearerToken(r)
			if got != tt.want {
				t.Errorf("extractBearerToken = %q, want %q", got, tt.want)
			}
		})
	}
}

// ── 单元测试：响应写入 ──

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, map[string]string{"msg": "ok"}, http.StatusCreated)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["msg"] != "ok" {
		t.Errorf("body[msg] = %q, want ok", body["msg"])
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, "SOME_ERR", "something went wrong", http.StatusBadRequest)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
	var resp ErrorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error.Code != "SOME_ERR" || resp.Error.Message != "something went wrong" {
		t.Errorf("got %+v", resp)
	}
}

func TestWriteOAuthError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteOAuthError(w, "invalid_grant", "bad creds", http.StatusUnauthorized)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d", w.Code)
	}
	var resp OAuthErrorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "invalid_grant" || resp.Description != "bad creds" {
		t.Errorf("got %+v", resp)
	}
}

// ── AuthHandler 创建 ──

func TestNewAuthHandler(t *testing.T) {
	st := &mockStorer{}
	h := NewAuthHandler(st, "mysecret", &mockSMSSender{})
	if h == nil {
		t.Fatal("NewAuthHandler returned nil")
	}
	if string(h.secret) != "mysecret" {
		t.Errorf("secret = %q, want mysecret", string(h.secret))
	}
}

// ── parseToken ──

func TestParseToken_Valid(t *testing.T) {
	h := newTestHandler()
	token := loginGetToken(t, h)

	claims, err := h.parseToken(token)
	if err != nil {
		t.Fatalf("parseToken error: %v", err)
	}
	if claims["sub"] == "" {
		t.Error("claims[sub] is empty")
	}
}

func TestParseToken_Invalid(t *testing.T) {
	h := newTestHandler()

	_, err := h.parseToken("invalid.token.here")
	if err == nil {
		t.Error("expected error for invalid token")
	}

	_, err = h.parseToken("")
	if err == nil {
		t.Error("expected error for empty token")
	}
}

func TestParseToken_WrongAlg(t *testing.T) {
	h := newTestHandler()
	_, err := h.parseToken("eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig")
	if err == nil {
		t.Error("expected error for wrong algorithm")
	}
}

func TestParseToken_NoClaims(t *testing.T) {
	h := newTestHandler()
	_, err := h.parseToken("eyJhbGciOiJIUzI1NiJ9.A.p")
	if err == nil {
		t.Error("expected error for malformed token")
	}
}

// ── 用户查找 ──

func TestFindUserByUsername(t *testing.T) {
	st := &mockStorer{users: []model.User{
		{ID: "u1", Username: "alice", PasswordHash: "hash1"},
	}}
	h := NewAuthHandler(st, "secret", &mockSMSSender{})
	h.SetupOAuth()

	u, err := h.findUserByUsername("alice")
	if err != nil {
		t.Fatalf("findUserByUsername error: %v", err)
	}
	if u == nil || u.ID != "u1" {
		t.Errorf("got %+v", u)
	}

	u, err = h.findUserByUsername("bob")
	if err != nil {
		t.Fatalf("findUserByUsername error: %v", err)
	}
	if u != nil {
		t.Errorf("expected nil for non-existent user, got %+v", u)
	}
}

func TestFindUserByUsername_StoreError(t *testing.T) {
	st := &mockStorer{listErr: fmt.Errorf("store down")}
	h := NewAuthHandler(st, "secret", &mockSMSSender{})
	h.SetupOAuth()

	_, err := h.findUserByUsername("alice")
	if err == nil {
		t.Error("expected error from store")
	}
}

func TestFindUserByUsername_BadJSON(t *testing.T) {
	h := NewAuthHandler(&mockStorer{}, "secret", &mockSMSSender{})
	h.SetupOAuth()

	// 用 corruptStorer 的 List 返回坏 json
	h.store = &corruptStorer{}
	_, err := h.findUserByUsername("alice")
	if err == nil {
		t.Error("expected Unmarshal error")
	}
}

func TestFindUserByPhone(t *testing.T) {
	st := &mockStorer{users: []model.User{
		{ID: "u1", Phone: "13800138000"},
	}}
	h := NewAuthHandler(st, "secret", &mockSMSSender{})
	h.SetupOAuth()

	u, err := h.findUserByPhone("13800138000")
	if err != nil {
		t.Fatalf("findUserByPhone error: %v", err)
	}
	if u == nil || u.ID != "u1" {
		t.Errorf("got %+v", u)
	}

	u, err = h.findUserByPhone("13900000000")
	if err != nil {
		t.Fatalf("findUserByPhone error: %v", err)
	}
	if u != nil {
		t.Errorf("expected nil for non-existent phone, got %+v", u)
	}
}

func TestFindUserByPhone_StoreError(t *testing.T) {
	st := &mockStorer{listErr: fmt.Errorf("store down")}
	h := NewAuthHandler(st, "secret", &mockSMSSender{})
	h.SetupOAuth()

	_, err := h.findUserByPhone("13800138000")
	if err == nil {
		t.Error("expected error from store")
	}
}

func TestFindUserByPhone_BadJSON(t *testing.T) {
	h := NewAuthHandler(&mockStorer{}, "secret", &mockSMSSender{})
	h.SetupOAuth()
	h.store = &corruptStorer{}

	_, err := h.findUserByPhone("13800138000")
	if err == nil {
		t.Error("expected Unmarshal error")
	}
}

// ── EnsureAdmin ──

func TestEnsureAdmin_Creates(t *testing.T) {
	h := newTestHandler()

	err := h.EnsureAdmin("admin123")
	if err != nil {
		t.Fatalf("EnsureAdmin error: %v", err)
	}

	u, _ := h.findUserByUsername("admin")
	if u == nil {
		t.Fatal("admin user not found")
	}
	if u.PasswordHash != hashPassword("admin", "admin123") {
		t.Errorf("password hash mismatch: %q vs %q", u.PasswordHash, hashPassword("admin", "admin123"))
	}
}

func TestEnsureAdmin_Idempotent(t *testing.T) {
	h := newTestHandler()

	if err := h.EnsureAdmin("pw"); err != nil {
		t.Fatalf("first EnsureAdmin: %v", err)
	}
	if err := h.EnsureAdmin("pw"); err != nil {
		t.Fatalf("second EnsureAdmin: %v", err)
	}

	u, _ := h.findUserByUsername("admin")
	if u == nil {
		t.Fatal("admin user not found after second call")
	}
}

func TestEnsureAdmin_CreateError(t *testing.T) {
	st := &mockStorer{createErr: fmt.Errorf("disk full")}
	h := NewAuthHandler(st, "secret", &mockSMSSender{})
	h.SetupOAuth()

	err := h.EnsureAdmin("pw")
	if err == nil {
		t.Error("expected error from store.Create")
	}
}

func TestEnsureAdmin_UpdateError(t *testing.T) {
	st := &mockStorer{updateErr: fmt.Errorf("disk full")}
	h := NewAuthHandler(st, "secret", &mockSMSSender{})
	h.SetupOAuth()

	err := h.EnsureAdmin("pw")
	if err == nil {
		t.Error("expected error from store.Update")
	}
}

func TestEnsureAdmin_ListError(t *testing.T) {
	st := &mockStorer{listErr: fmt.Errorf("store down")}
	h := NewAuthHandler(st, "secret", &mockSMSSender{})
	h.SetupOAuth()

	err := h.EnsureAdmin("pw")
	if err == nil {
		t.Error("expected error from store.List")
	}
}

// ── SendCode ──

func TestSendCode_Success(t *testing.T) {
	h := newTestHandler()
	body := `{"phone":"13800138000"}`
	r := httptest.NewRequest("POST", "/oauth/sms/send", bytes.NewReader([]byte(body)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SendCode(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["message"] != "code sent" {
		t.Errorf("message = %q", resp["message"])
	}
}

func TestSendCode_InvalidPhone(t *testing.T) {
	h := newTestHandler()
	tests := []string{
		`{"phone":"12345"}`,
		`{"phone":""}`,
		`{"phone":"abc"}`,
		`invalid json`,
	}
	for _, body := range tests {
		r := httptest.NewRequest("POST", "/oauth/sms/send", bytes.NewReader([]byte(body)))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.SendCode(w, r)
		if w.Code == http.StatusOK {
			t.Errorf("expected non-200 for body=%q, got %d", body, w.Code)
		}
	}
}

func TestSendCode_RateLimit(t *testing.T) {
	h := newTestHandler()

	body := `{"phone":"13800138000"}`
	r1 := httptest.NewRequest("POST", "/oauth/sms/send", bytes.NewReader([]byte(body)))
	r1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	h.SendCode(w1, r1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request failed: %d", w1.Code)
	}

	r2 := httptest.NewRequest("POST", "/oauth/sms/send", bytes.NewReader([]byte(body)))
	r2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	h.SendCode(w2, r2)
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d, body=%s", w2.Code, w2.Body.String())
	}
}

func TestSendCode_SMSSenderError(t *testing.T) {
	h := NewAuthHandler(&mockStorer{}, "secret", &mockSMSSender{err: fmt.Errorf("sms down")})
	h.SetupOAuth()

	body := `{"phone":"13800138000"}`
	r := httptest.NewRequest("POST", "/oauth/sms/send", bytes.NewReader([]byte(body)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SendCode(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d, body=%s", w.Code, w.Body.String())
	}
}

// ── OAuth Token ──

func TestToken_UnsupportedGrantType(t *testing.T) {
	h := newTestHandler()
	form := url.Values{"grant_type": {"client_credentials"}}
	r := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Token(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var errResp OAuthErrorResponse
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp.Error != "unsupported_grant_type" {
		t.Errorf("error = %q", errResp.Error)
	}
}

func TestToken_Password_Success(t *testing.T) {
	h := newTestHandler()
	_ = h.EnsureAdmin("admin")

	form := url.Values{"grant_type": {"password"}, "username": {"admin"}, "password": {"admin"}}
	r := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Token(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}
	json.NewDecoder(w.Body).Decode(&tokenResp)
	if tokenResp.AccessToken == "" {
		t.Error("access_token is empty")
	}
	if tokenResp.TokenType != "Bearer" {
		t.Errorf("token_type = %q", tokenResp.TokenType)
	}
	if tokenResp.ExpiresIn == 0 {
		t.Error("expires_in is 0")
	}
	if tokenResp.RefreshToken == "" {
		t.Error("refresh_token is empty")
	}
}

func TestToken_Password_WrongCreds(t *testing.T) {
	h := newTestHandler()
	_ = h.EnsureAdmin("admin")

	form := url.Values{"grant_type": {"password"}, "username": {"admin"}, "password": {"wrong"}}
	r := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Token(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
}

// 密码授权时 store.List 失败 → PasswordAuthorizationHandler 返回 error
func TestToken_Password_StoreError(t *testing.T) {
	st := &mockStorer{listErr: fmt.Errorf("down")}
	h := NewAuthHandler(st, "secret", &mockSMSSender{})
	h.SetupOAuth()

	form := url.Values{"grant_type": {"password"}, "username": {"admin"}, "password": {"x"}}
	r := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Token(w, r)

	// go-oauth2 把 PasswordAuthorizationHandler 的错误包装成 server_error
	// 但我们无法区分具体错误码，只要不是 200 就行
	if w.Code == http.StatusOK {
		t.Errorf("expected error status, got 200: body=%s", w.Body.String())
	}
}

func TestToken_Refresh_Success(t *testing.T) {
	h := newTestHandler()
	rt := getRefreshToken(t, h)

	form2 := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {rt}}
	r2 := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form2.Encode()))
	r2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	h.Token(w2, r2)

	if w2.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, body=%s", w2.Code, w2.Body.String())
	}
	var second struct {
		AccessToken string `json:"access_token"`
	}
	json.NewDecoder(w2.Body).Decode(&second)
	if second.AccessToken == "" {
		t.Error("no access_token from refresh")
	}
}

func TestToken_SMSCode_Success(t *testing.T) {
	h := newTestHandler()

	body := `{"phone":"13800138000"}`
	r1 := httptest.NewRequest("POST", "/oauth/sms/send", bytes.NewReader([]byte(body)))
	r1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	h.SendCode(w1, r1)
	if w1.Code != http.StatusOK {
		t.Fatalf("send code failed: %d", w1.Code)
	}

	h.codeMu.Lock()
	codes := h.codeStore["13800138000"]
	h.codeMu.Unlock()
	if len(codes) == 0 {
		t.Fatal("no code stored")
	}
	smsCode := codes[len(codes)-1].Code

	form := url.Values{"grant_type": {"sms_code"}, "phone": {"13800138000"}, "code": {smsCode}}
	r2 := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	r2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	h.Token(w2, r2)

	if w2.Code != http.StatusOK {
		t.Fatalf("sms login status = %d, body=%s", w2.Code, w2.Body.String())
	}
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	json.NewDecoder(w2.Body).Decode(&tokenResp)
	if tokenResp.AccessToken == "" {
		t.Error("access_token is empty")
	}
	if tokenResp.RefreshToken == "" {
		t.Error("refresh_token is empty")
	}

	u, _ := h.findUserByPhone("13800138000")
	if u == nil {
		t.Fatal("user not auto-created")
	}
	if !u.PhoneVerified {
		t.Error("phone not marked verified")
	}
}

func TestToken_SMSCode_WrongCode(t *testing.T) {
	h := newTestHandler()

	form := url.Values{"grant_type": {"sms_code"}, "phone": {"13800138000"}, "code": {"000000"}}
	r := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Token(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestToken_SMSCode_InvalidPhone(t *testing.T) {
	h := newTestHandler()
	form := url.Values{"grant_type": {"sms_code"}, "phone": {"abc"}, "code": {"123456"}}
	r := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Token(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestToken_SMSCode_EmptyCode(t *testing.T) {
	h := newTestHandler()
	form := url.Values{"grant_type": {"sms_code"}, "phone": {"13800138000"}, "code": {""}}
	r := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.Token(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// tokenSMS 错误路径：store.Create 失败
func TestTokenSMS_CreateUserError(t *testing.T) {
	st := &mockStorer{createErr: fmt.Errorf("disk full")}
	h := NewAuthHandler(st, "secret", &mockSMSSender{})
	h.SetupOAuth()

	body := `{"phone":"13800138000"}`
	r1 := httptest.NewRequest("POST", "/oauth/sms/send", bytes.NewReader([]byte(body)))
	r1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	h.SendCode(w1, r1)

	h.codeMu.Lock()
	code := h.codeStore["13800138000"][0].Code
	h.codeMu.Unlock()

	form := url.Values{"grant_type": {"sms_code"}, "phone": {"13800138000"}, "code": {code}}
	r2 := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	r2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	h.Token(w2, r2)

	if w2.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w2.Code)
	}
}

// tokenSMS 错误路径：findUserByPhone 错误
func TestTokenSMS_FindUserError(t *testing.T) {
	st := &mockStorer{listErr: fmt.Errorf("store down")}
	h := NewAuthHandler(&mockStorer{}, "secret", &mockSMSSender{})
	h.SetupOAuth()

	body := `{"phone":"13800138000"}`
	r1 := httptest.NewRequest("POST", "/oauth/sms/send", bytes.NewReader([]byte(body)))
	r1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	h.SendCode(w1, r1)

	h.codeMu.Lock()
	code := h.codeStore["13800138000"][0].Code
	h.codeMu.Unlock()

	h.store = st

	form := url.Values{"grant_type": {"sms_code"}, "phone": {"13800138000"}, "code": {code}}
	r2 := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	r2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	h.Token(w2, r2)

	if w2.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w2.Code)
	}
}

// tokenSMS 错误路径：手机号已存在但 store.Update 失败
func TestTokenSMS_UpdatePhoneVerifiedError(t *testing.T) {
	st := &mockStorer{
		users: []model.User{
			{ID: "u1", Phone: "13800138000", PhoneVerified: false},
		},
		updateErr: fmt.Errorf("disk full"),
	}
	h := NewAuthHandler(st, "secret", &mockSMSSender{})
	h.SetupOAuth()

	body := `{"phone":"13800138000"}`
	r1 := httptest.NewRequest("POST", "/oauth/sms/send", bytes.NewReader([]byte(body)))
	r1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	h.SendCode(w1, r1)

	h.codeMu.Lock()
	code := h.codeStore["13800138000"][0].Code
	h.codeMu.Unlock()

	form := url.Values{"grant_type": {"sms_code"}, "phone": {"13800138000"}, "code": {code}}
	r2 := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	r2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	h.Token(w2, r2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 despite update error, got %d: %s", w2.Code, w2.Body.String())
	}
}

// tokenSMS 错误路径：GenerateAccessToken 失败
// 重建一个不含 client store 的 manager 来触发
func TestTokenSMS_GenerateTokenError(t *testing.T) {
	h := NewAuthHandler(&mockStorer{}, "secret", &mockSMSSender{})
	// 用不含 client 的 manager 覆盖 oauthMgr
	mgr := manage.NewDefaultManager()
	ts, _ := store.NewMemoryTokenStore()
	mgr.MapTokenStorage(ts)
	// 不设置 client store → GenerateAccessToken 会因找不到 client 而失败
	h.oauthMgr = mgr
	h.SetupOAuth()
	// 重新用正确的 server，但 mgr 没有 client store
	_ = h.oauthSrv

	// 发验证码
	body := `{"phone":"13800138000"}`
	r1 := httptest.NewRequest("POST", "/oauth/sms/send", bytes.NewReader([]byte(body)))
	r1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	h.SendCode(w1, r1)

	h.codeMu.Lock()
	code := h.codeStore["13800138000"][0].Code
	h.codeMu.Unlock()

	// 用空的 oauthMgr 覆盖（没有 client store 的 manager）
	h.oauthMgr = &emptyManager{}

	form := url.Values{"grant_type": {"sms_code"}, "phone": {"13800138000"}, "code": {code}}
	r2 := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	r2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	h.Token(w2, r2)

	if w2.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 from GenerateAccessToken, got %d: %s", w2.Code, w2.Body.String())
	}
}

// emptyManager 模拟 GenerateAccessToken 失败的 Manager.
type emptyManager struct{}

func (e *emptyManager) GetClient(ctx context.Context, clientID string) (info oauth2.ClientInfo, err error) {
	return nil, fmt.Errorf("no client")
}
func (e *emptyManager) GenerateAuthToken(ctx context.Context, rt oauth2.ResponseType, tgr *oauth2.TokenGenerateRequest) (oauth2.TokenInfo, error) {
	return nil, fmt.Errorf("no token")
}
func (e *emptyManager) GenerateAccessToken(ctx context.Context, gt oauth2.GrantType, tgr *oauth2.TokenGenerateRequest) (oauth2.TokenInfo, error) {
	return nil, fmt.Errorf("no access token")
}
func (e *emptyManager) RefreshAccessToken(ctx context.Context, tgr *oauth2.TokenGenerateRequest) (oauth2.TokenInfo, error) {
	return nil, fmt.Errorf("no token")
}
func (e *emptyManager) RemoveAccessToken(ctx context.Context, access string) error {
	return nil
}
func (e *emptyManager) RemoveRefreshToken(ctx context.Context, refresh string) error {
	return nil
}
func (e *emptyManager) LoadAccessToken(ctx context.Context, access string) (oauth2.TokenInfo, error) {
	return nil, fmt.Errorf("no token")
}
func (e *emptyManager) LoadRefreshToken(ctx context.Context, refresh string) (oauth2.TokenInfo, error) {
	return nil, fmt.Errorf("no token")
}

// ── UserInfo ──

func TestUserInfo_Success(t *testing.T) {
	h := newTestHandler()
	token := loginGetToken(t, h)

	r2 := httptest.NewRequest("GET", "/userinfo", nil)
	r2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	h.UserInfo(w2, r2)

	if w2.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w2.Code, w2.Body.String())
	}
	var info map[string]any
	json.NewDecoder(w2.Body).Decode(&info)
	if info["sub"] == "" {
		t.Error("userinfo sub is empty")
	}
	if _, ok := info["password_hash"]; ok {
		t.Error("password_hash should not be exposed")
	}
}

func TestUserInfo_NoToken(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest("GET", "/userinfo", nil)
	w := httptest.NewRecorder()
	h.UserInfo(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestUserInfo_InvalidToken(t *testing.T) {
	h := newTestHandler()
	r := httptest.NewRequest("GET", "/userinfo", nil)
	r.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()
	h.UserInfo(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestUserInfo_UserNotFound(t *testing.T) {
	h := newTestHandler()
	token := loginGetToken(t, h)

	h.store = &mockStorer{}

	r2 := httptest.NewRequest("GET", "/userinfo", nil)
	r2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	h.UserInfo(w2, r2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: body=%s", w2.Code, w2.Body.String())
	}
}

// UserInfo 错误路径：json.Unmarshal 失败
func TestUserInfo_UnmarshalError(t *testing.T) {
	h := newTestHandler()
	token := loginGetToken(t, h)

	h.store = &corruptStorer{}

	r := httptest.NewRequest("GET", "/userinfo", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.UserInfo(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// UserInfo 错误路径：claims 中没有 sub
func TestUserInfo_NoSubClaim(t *testing.T) {
	h := newTestHandler()

	// 构造一个没有 sub 的 claims 注入到 context
	claims := jwtlib.MapClaims{"foo": "bar"}
	r := httptest.NewRequest("GET", "/userinfo", nil)
	ctx := context.WithValue(r.Context(), ClaimsKey, claims)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	h.UserInfo(w, r)

	// UserInfo 用 parseToken 解析，但这里 context 中的 claims 没有 sub
	// 但实际上 UserInfo 会重新从 Authorization header 解析 token
	// 所以这个测试方式不 work，需要改为传给 parseToken 一个无 sub 的 token

	// 正确做法：用 golang-jwt 签一个没有 sub 的 token
	noSubToken, _ := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, jwtlib.MapClaims{
		"foo": "bar",
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(h.secret)

	r2 := httptest.NewRequest("GET", "/userinfo", nil)
	r2.Header.Set("Authorization", "Bearer "+noSubToken)
	w2 := httptest.NewRecorder()
	h.UserInfo(w2, r2)

	if w2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing sub, got %d: body=%s", w2.Code, w2.Body.String())
	}
}

// ── findValidCode / markCodeUsed ──

func TestFindValidCode(t *testing.T) {
	h := newTestHandler()
	vc := model.VerificationCode{
		Phone:     "13800138000",
		Code:      "123456",
		ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339),
		Used:      false,
	}
	h.codeStore["13800138000"] = append(h.codeStore["13800138000"], vc)

	found := h.findValidCode("13800138000", "123456")
	if found == nil {
		t.Fatal("valid code not found")
	}
	if found.Code != "123456" {
		t.Errorf("code = %q", found.Code)
	}

	h.markCodeUsed("13800138000", "123456")
	found2 := h.findValidCode("13800138000", "123456")
	if found2 != nil {
		t.Error("used code should not be found")
	}

	expired := model.VerificationCode{
		Phone:     "13800138000",
		Code:      "654321",
		ExpiresAt: time.Now().Add(-time.Hour).Format(time.RFC3339),
	}
	h.codeStore["13800138000"] = append(h.codeStore["13800138000"], expired)
	found3 := h.findValidCode("13800138000", "654321")
	if found3 != nil {
		t.Error("expired code should not be found")
	}

	found4 := h.findValidCode("13800138000", "000000")
	if found4 != nil {
		t.Error("non-existent code should not be found")
	}
}

// findValidCode 错误路径：不可解析的 ExpiresAt
func TestFindValidCode_BadExpiresAt(t *testing.T) {
	h := newTestHandler()
	vc := model.VerificationCode{
		Phone:     "13800138000",
		Code:      "123456",
		ExpiresAt: "not-a-date",
		Used:      false,
	}
	h.codeStore["13800138000"] = append(h.codeStore["13800138000"], vc)

	found := h.findValidCode("13800138000", "123456")
	if found != nil {
		t.Error("expected nil for bad ExpiresAt format")
	}
}

// ── AuthMiddleware ──

func TestAuthMiddleware_Valid(t *testing.T) {
	h := newTestHandler()
	token := loginGetToken(t, h)

	called := false
	mw := h.AuthMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		called = true
		if r.Context().Value(ClaimsKey) == nil {
			t.Error("ClaimsKey not set in context")
		}
	}))
	r2 := httptest.NewRequest("GET", "/protected", nil)
	r2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	mw.ServeHTTP(w2, r2)

	if !called {
		t.Error("next handler was not called")
	}
}

func TestAuthMiddleware_NoToken(t *testing.T) {
	h := newTestHandler()
	called := false
	mw := h.AuthMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	r := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, r)

	if called {
		t.Error("next handler should not be called")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	h := newTestHandler()
	called := false
	mw := h.AuthMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	r := httptest.NewRequest("GET", "/protected", nil)
	r.Header.Set("Authorization", "Bearer bad.token.here")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, r)

	if called {
		t.Error("next handler should not be called")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d", w.Code)
	}
}

// ── ConsoleSender ──

func TestConsoleSender(t *testing.T) {
	s := &ConsoleSender{}
	err := s.Send(context.Background(), "13800138000", "123456")
	if err != nil {
		t.Fatalf("ConsoleSender.Send error: %v", err)
	}
}

// ── PhoneVerified 更新路径 ──

func TestTokenSMS_UpdatesPhoneVerified(t *testing.T) {
	st := &mockStorer{users: []model.User{
		{ID: "u1", Username: "13800138000", Phone: "13800138000", PasswordHash: "x"},
	}}
	h := NewAuthHandler(st, "secret", &mockSMSSender{})
	h.SetupOAuth()

	body := `{"phone":"13800138000"}`
	r1 := httptest.NewRequest("POST", "/oauth/sms/send", bytes.NewReader([]byte(body)))
	r1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	h.SendCode(w1, r1)

	h.codeMu.Lock()
	code := h.codeStore["13800138000"][len(h.codeStore["13800138000"])-1].Code
	h.codeMu.Unlock()

	form := url.Values{"grant_type": {"sms_code"}, "phone": {"13800138000"}, "code": {code}}
	r2 := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	r2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	h.Token(w2, r2)

	if w2.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w2.Code, w2.Body.String())
	}

	u, _ := h.findUserByPhone("13800138000")
	if u == nil || !u.PhoneVerified {
		t.Error("phone_verified should be true after sms login")
	}
}

// ── 边缘 case ──

func TestMarkCodeUsed_NotFound(t *testing.T) {
	h := newTestHandler()
	h.markCodeUsed("13800138000", "000000") // should not panic
}
