package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/errors"
	"github.com/go-oauth2/oauth2/v4/generates"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/models"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/go-oauth2/oauth2/v4/store"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/quanttide/qtcloud-auth/internal/model"
)

// SetupOAuth 初始化 go-oauth2 Manager 和 Server.
func (h *AuthHandler) SetupOAuth() *server.Server {
	// Manager
	mgr := manage.NewDefaultManager()
	// access_token 7 天（默认 2h 过短，用户频繁重登）；refresh_token 30 天
	mgr.SetAuthorizeCodeTokenCfg(&manage.Config{
		AccessTokenExp:  time.Hour * 24 * 7,
		RefreshTokenExp: time.Hour * 24 * 30,
		IsGenerateRefresh: true,
	})
	mgr.MapAccessGenerate(generates.NewJWTAccessGenerate("", h.keyPEM, jwt.SigningMethodRS256))

	ts, _ := store.NewMemoryTokenStore()
	if h.oauthTokenStore != nil {
		ts = h.oauthTokenStore
	}
	mgr.MapTokenStorage(ts)

	// Client store
	cs := store.NewClientStore()
	_ = cs.Set("qtcloud-auth", &models.Client{
		ID:     "qtcloud-auth",
		Public: true,
	})
	mgr.MapClientStorage(cs)

	h.oauthMgr = mgr

	// Server
	srv := server.NewServer(server.NewConfig(), mgr)

	// 密码授权：委托给我们的验证逻辑（支持 用户名/邮箱/手机号 任一登录）
	srv.PasswordAuthorizationHandler = func(ctx context.Context, clientID, username, password string) (string, error) {
		user, err := h.findUserByLogin(username)
		if err != nil {
			return "", err
		}
		if user == nil {
			return "", errors.ErrInvalidGrant
		}
		if !verifyPassword(password, user.PasswordHash) {
			return "", errors.ErrInvalidGrant
		}
		return user.ID, nil
	}

	// 公开客户端：固定返回 qtcloud-auth
	srv.ClientInfoHandler = func(r *http.Request) (string, string, error) {
		return "qtcloud-auth", "", nil
	}

	// 错误处理
	srv.InternalErrorHandler = func(err error) (re *errors.Response) {
		slog.Error("oauth internal", "error", err)
		return nil
	}
	srv.ResponseErrorHandler = func(re *errors.Response) {
		slog.Debug("oauth response error", "code", re.Error, "desc", re.Description)
	}

	h.oauthSrv = srv
	return srv
}

// Token POST /oauth/token 统一认证入口.
func (h *AuthHandler) Token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		WriteOAuthError(w, "invalid_request", "cannot parse form", http.StatusBadRequest)
		return
	}

	switch r.Form.Get("grant_type") {
	case "password", "refresh_token":
		h.oauthSrv.HandleTokenRequest(w, r)

	case "sms_code":
		h.tokenSMS(w, r)

	default:
		WriteOAuthError(w, "unsupported_grant_type",
			"supported: password, sms_code, refresh_token", http.StatusBadRequest)
	}
}

// tokenSMS 手机验证码登录/注册.
func (h *AuthHandler) tokenSMS(w http.ResponseWriter, r *http.Request) {
	phone := r.Form.Get("phone")
	code := r.Form.Get("code")

	if !isValidPhone(phone) || code == "" {
		WriteOAuthError(w, "invalid_request", "phone and code are required", http.StatusBadRequest)
		return
	}

	vc := h.findValidCode(phone, code)
	if vc == nil {
		WriteOAuthError(w, "invalid_grant", "invalid or expired verification code", http.StatusUnauthorized)
		return
	}
	h.markCodeUsed(phone, code)

	user, err := h.findUserByPhone(phone)
	if err != nil {
		slog.Error("find user by phone", "error", err)
		WriteOAuthError(w, "server_error", "internal error", http.StatusInternalServerError)
		return
	}

	if user == nil {
		user = &model.User{
			Username:      phone,
			Phone:         phone,
			PhoneVerified: true,
			Nickname:      maskPhone(phone),
			CreatedAt:     time.Now().Format(time.RFC3339),
		}
		data, mErr := json.Marshal(user)
		if mErr != nil {
			slog.Error("marshal user", "error", mErr)
			WriteOAuthError(w, "server_error", "internal error", http.StatusInternalServerError)
			return
		}
		id, cErr := h.store.Create("auth/users", data)
		if cErr != nil {
			slog.Error("create user", "error", cErr)
			WriteOAuthError(w, "server_error", "internal error", http.StatusInternalServerError)
			return
		}
		user.ID = id
	} else if !user.PhoneVerified {
		user.PhoneVerified = true
		data, _ := json.Marshal(user)
		_ = h.store.Update("auth/users", user.ID, data)
	}

	// 用 go-oauth2 Manager 签发令牌
	tgr := &oauth2.TokenGenerateRequest{
		ClientID: "qtcloud-auth",
		UserID:   user.ID,
	}
	ti, err := h.oauthMgr.GenerateAccessToken(r.Context(), oauth2.PasswordCredentials, tgr)
	if err != nil {
		slog.Error("generate access token", "error", err)
		WriteOAuthError(w, "server_error", "failed to generate token", http.StatusInternalServerError)
		return
	}

	WriteJSON(w, h.oauthSrv.GetTokenData(ti), http.StatusOK)
}

// Register POST /oauth/register 用户名/密码注册（支持可选 email/phone），成功即签发 JWT.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		Nickname string `json:"nickname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteOAuthError(w, "invalid_request", "invalid request body", http.StatusBadRequest)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	req.Phone = strings.TrimSpace(req.Phone)

	if len(req.Username) < 3 || len(req.Username) > 64 {
		WriteOAuthError(w, "invalid_request", "username must be 3-64 characters", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 6 {
		WriteOAuthError(w, "invalid_request", "password must be at least 6 characters", http.StatusBadRequest)
		return
	}
	if req.Email != "" && !isValidEmail(req.Email) {
		WriteOAuthError(w, "invalid_request", "invalid email", http.StatusBadRequest)
		return
	}
	if req.Phone != "" && !isValidPhone(req.Phone) {
		WriteOAuthError(w, "invalid_request", "invalid phone number", http.StatusBadRequest)
		return
	}

	// 唯一性：用户名/邮箱/手机号任一冲突即拒绝
	existing, err := h.findUserByLogin(req.Username)
	if err != nil {
		slog.Error("register check username", "error", err)
		WriteOAuthError(w, "server_error", "internal error", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		WriteOAuthError(w, "invalid_request", "username already registered", http.StatusConflict)
		return
	}
	if req.Email != "" {
		existing, err = h.findUserByLogin(req.Email)
		if err != nil {
			slog.Error("register check email", "error", err)
			WriteOAuthError(w, "server_error", "internal error", http.StatusInternalServerError)
			return
		}
		if existing != nil {
			WriteOAuthError(w, "invalid_request", "email already registered", http.StatusConflict)
			return
		}
	}
	if req.Phone != "" {
		existing, err = h.findUserByLogin(req.Phone)
		if err != nil {
			slog.Error("register check phone", "error", err)
			WriteOAuthError(w, "server_error", "internal error", http.StatusInternalServerError)
			return
		}
		if existing != nil {
			WriteOAuthError(w, "invalid_request", "phone already registered", http.StatusConflict)
			return
		}
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		slog.Error("hash password", "error", err)
		WriteOAuthError(w, "server_error", "internal error", http.StatusInternalServerError)
		return
	}
	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Username
	}
	user := model.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		Phone:        req.Phone,
		Nickname:     nickname,
		CreatedAt:    time.Now().Format(time.RFC3339),
	}
	data, mErr := json.Marshal(user)
	if mErr != nil {
		slog.Error("marshal user", "error", mErr)
		WriteOAuthError(w, "server_error", "internal error", http.StatusInternalServerError)
		return
	}
	id, cErr := h.store.Create("auth/users", data)
	if cErr != nil {
		slog.Error("create user", "error", cErr)
		WriteOAuthError(w, "server_error", "internal error", http.StatusInternalServerError)
		return
	}
	user.ID = id

	// 注册成功即签发令牌（与验证码登录体验一致）
	tgr := &oauth2.TokenGenerateRequest{
		ClientID: "qtcloud-auth",
		UserID:   user.ID,
	}
	ti, tErr := h.oauthMgr.GenerateAccessToken(r.Context(), oauth2.PasswordCredentials, tgr)
	if tErr != nil {
		slog.Error("generate access token", "error", tErr)
		WriteOAuthError(w, "server_error", "failed to generate token", http.StatusInternalServerError)
		return
	}
	WriteJSON(w, h.oauthSrv.GetTokenData(ti), http.StatusOK)
}

// UserInfo GET /userinfo 获取当前用户信息（OIDC 标准 claims）.
func (h *AuthHandler) UserInfo(w http.ResponseWriter, r *http.Request) {
	tokenStr := extractBearerToken(r)
	if tokenStr == "" {
		WriteOAuthError(w, "invalid_token", "missing authorization header", http.StatusUnauthorized)
		return
	}

	claims, err := h.parseToken(tokenStr)
	if err != nil {
		WriteOAuthError(w, "invalid_token", "invalid or expired token", http.StatusUnauthorized)
		return
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		WriteOAuthError(w, "invalid_token", "invalid token claims", http.StatusUnauthorized)
		return
	}

	data, err := h.store.Get("auth/users", sub)
	if err != nil {
		WriteOAuthError(w, "server_error", "user not found", http.StatusNotFound)
		return
	}

	var user model.User
	if err := json.Unmarshal(data, &user); err != nil {
		slog.Error("parse user", "error", err)
		WriteOAuthError(w, "server_error", "internal error", http.StatusInternalServerError)
		return
	}

	WriteJSON(w, map[string]any{
		"sub":            user.ID,
		"username":       user.Username,
		"email":          user.Email,
		"phone":          user.Phone,
		"phone_verified": user.PhoneVerified,
		"nickname":       user.Nickname,
		"picture":        user.Avatar,
		"updated_at":     user.CreatedAt,
	}, http.StatusOK)
}

// extractBearerToken 从请求头提取 Bearer token.
func extractBearerToken(r *http.Request) string {
	ah := r.Header.Get("Authorization")
	if len(ah) < 7 || ah[:7] != "Bearer " {
		return ""
	}
	return ah[7:]
}
