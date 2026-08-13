package api

// 修改密码：POST /oauth/password/change（JWT 鉴权，旧密码验证 + 新密码 Argon2id）。

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ChangePassword 修改当前登录用户密码。
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFrom(r)
	if userID == "" {
		WriteOAuthError(w, "invalid_token", "missing or invalid authorization header", http.StatusUnauthorized)
		return
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteOAuthError(w, "invalid_request", "invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.NewPassword) < 6 {
		WriteOAuthError(w, "invalid_request", "new password must be at least 6 characters", http.StatusBadRequest)
		return
	}
	user, err := h.findUserByID(userID)
	if err != nil {
		slog.Error("change password: find user", "error", err)
		WriteOAuthError(w, "server_error", "internal error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		WriteOAuthError(w, "invalid_token", "user not found", http.StatusUnauthorized)
		return
	}
	if !verifyPassword(req.OldPassword, user.PasswordHash) {
		WriteOAuthError(w, "invalid_grant", "old password incorrect", http.StatusUnauthorized)
		return
	}
	hash, err := hashPassword(req.NewPassword)
	if err != nil {
		slog.Error("change password: hash", "error", err)
		WriteOAuthError(w, "server_error", "internal error", http.StatusInternalServerError)
		return
	}
	user.PasswordHash = hash
	data, _ := json.Marshal(user)
	if err := h.store.Update("auth/users", user.ID, data); err != nil {
		slog.Error("change password: update", "error", err)
		WriteOAuthError(w, "server_error", "internal error", http.StatusInternalServerError)
		return
	}
	slog.Info("password changed", "user", user.ID)
	WriteJSON(w, map[string]any{"success": true}, http.StatusOK)
}
