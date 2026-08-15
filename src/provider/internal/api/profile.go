package api

// 更新个人资料：POST /userinfo（JWT 鉴权，MVP 仅支持昵称更新）。
// 用户名/邮箱/手机号是登录凭证，改动涉及唯一性与登录标识，本期不允许。
// 注：路由用 POST 而非 PATCH——传统版 API 网关不支持 PATCH 方法（网关实测报 unauthorized method）。

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// UpdateUserInfo 更新当前登录用户资料（昵称）。
func (h *AuthHandler) UpdateUserInfo(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFrom(r)
	if userID == "" {
		WriteOAuthError(w, "invalid_token", "missing or invalid authorization header", http.StatusUnauthorized)
		return
	}
	var req struct {
		Nickname string `json:"nickname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteOAuthError(w, "invalid_request", "invalid request body", http.StatusBadRequest)
		return
	}
	req.Nickname = strings.TrimSpace(req.Nickname)
	if req.Nickname == "" {
		WriteOAuthError(w, "invalid_request", "nickname is required", http.StatusBadRequest)
		return
	}
	if len(req.Nickname) > 64 {
		WriteOAuthError(w, "invalid_request", "nickname must be at most 64 characters", http.StatusBadRequest)
		return
	}
	user, err := h.findUserByID(userID)
	if err != nil {
		slog.Error("update profile: find user", "error", err)
		WriteOAuthError(w, "server_error", "internal error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		WriteOAuthError(w, "invalid_token", "user not found", http.StatusUnauthorized)
		return
	}
	user.Nickname = req.Nickname
	data, _ := json.Marshal(user)
	if err := h.store.Update("auth/users", user.ID, data); err != nil {
		slog.Error("update profile: update", "error", err)
		WriteOAuthError(w, "server_error", "internal error", http.StatusInternalServerError)
		return
	}
	slog.Info("profile updated", "user", user.ID, "nickname", req.Nickname)
	WriteJSON(w, map[string]any{
		"sub":      user.ID,
		"nickname": user.Nickname,
	}, http.StatusOK)
}
