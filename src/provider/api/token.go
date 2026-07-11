package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/quanttide/qtcloud-auth/auth"
	"github.com/quanttide/qtcloud-auth/model"
)

// ── /oauth/token ──

// Token POST /oauth/token 统一认证入口.
// 支持 grant_type: password | sms_code | refresh_token.
func (h *AuthHandler) Token(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, "INVALID_REQUEST", "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		GrantType    string `json:"grant_type"`
		Username     string `json:"username"`
		Password     string `json:"password"`
		Phone        string `json:"phone"`
		Code         string `json:"code"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteOAuthError(w, "invalid_request", "invalid request body", http.StatusBadRequest)
		return
	}

	switch body.GrantType {
	case "password":
		h.tokenPassword(w, r, body.Username, body.Password)
	case "sms_code":
		h.tokenSMS(w, r, body.Phone, body.Code)
	case "refresh_token":
		h.tokenRefresh(w, r, body.RefreshToken)
	default:
		WriteOAuthError(w, "unsupported_grant_type", "supported: password, sms_code, refresh_token", http.StatusBadRequest)
	}
}

func (h *AuthHandler) tokenPassword(w http.ResponseWriter, _ *http.Request, username, password string) {
	if username == "" || password == "" {
		WriteOAuthError(w, "invalid_grant", "username and password are required", http.StatusBadRequest)
		return
	}

	user, err := h.findUserByUsername(username)
	if err != nil {
		slog.Error("find user", "error", err)
		WriteOAuthError(w, "server_error", "internal error", http.StatusInternalServerError)
		return
	}
	if user == nil || user.PasswordHash != hashPassword(username, password) {
		WriteOAuthError(w, "invalid_grant", "invalid username or password", http.StatusUnauthorized)
		return
	}

	resp := h.issueTokens(user.ID, user.RoleID, user.Phone)
	WriteJSON(w, resp, http.StatusOK)
}

func (h *AuthHandler) tokenSMS(w http.ResponseWriter, _ *http.Request, phone, code string) {
	if !isValidPhone(phone) || code == "" {
		WriteOAuthError(w, "invalid_grant", "phone and code are required", http.StatusBadRequest)
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
		// 自动注册
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

	resp := h.issueTokens(user.ID, user.RoleID, user.Phone)
	WriteJSON(w, resp, http.StatusOK)
}

func (h *AuthHandler) tokenRefresh(w http.ResponseWriter, _ *http.Request, refreshToken string) {
	if refreshToken == "" {
		WriteOAuthError(w, "invalid_grant", "refresh_token is required", http.StatusBadRequest)
		return
	}

	claims, err := auth.Verify(refreshToken, h.secret)
	if err != nil {
		WriteOAuthError(w, "invalid_grant", "invalid or expired refresh token", http.StatusUnauthorized)
		return
	}
	if claims["type"] != "refresh_token" {
		WriteOAuthError(w, "invalid_grant", "invalid token type", http.StatusUnauthorized)
		return
	}

	sub, _ := claims["sub"].(string)
	var role, phone string
	data, err := h.store.Get("auth/users", sub)
	if err == nil {
		var user model.User
		if json.Unmarshal(data, &user) == nil {
			role = user.RoleID
			phone = user.Phone
		}
	}

	resp := h.issueTokens(sub, role, phone)
	WriteJSON(w, resp, http.StatusOK)
}

// ── /userinfo ──

// UserInfo GET /userinfo 获取当前用户信息（OIDC 标准）.
func (h *AuthHandler) UserInfo(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(ClaimsKey).(map[string]any)
	if !ok {
		WriteOAuthError(w, "invalid_token", "unauthorized", http.StatusUnauthorized)
		return
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		WriteOAuthError(w, "invalid_token", "invalid token claims", http.StatusUnauthorized)
		return
	}

	data, err := h.store.Get("auth/users", sub)
	if err != nil {
		slog.Error("get user", "error", err)
		WriteOAuthError(w, "server_error", "user not found", http.StatusNotFound)
		return
	}

	var user model.User
	if err := json.Unmarshal(data, &user); err != nil {
		slog.Error("parse user", "error", err)
		WriteOAuthError(w, "server_error", "internal error", http.StatusInternalServerError)
		return
	}

	// OIDC 标准 claims，不暴露 password_hash
	info := map[string]any{
		"sub":            user.ID,
		"phone":          user.Phone,
		"phone_verified": user.PhoneVerified,
		"nickname":       user.Nickname,
		"picture":        user.Avatar,
		"updated_at":     user.CreatedAt,
	}
	WriteJSON(w, info, http.StatusOK)
}
