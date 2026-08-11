package api

// 运维管理端点（admin）：删除用户等操作。
// fail-closed：未配置 ADMIN_TOKEN 或请求头不匹配一律 403。

import (
	"log/slog"
	"net/http"
)

// AdminHandler 运维管理 API。
type AdminHandler struct {
	store      Storer
	adminToken string
}

func NewAdminHandler(st Storer, adminToken string) *AdminHandler {
	return &AdminHandler{store: st, adminToken: adminToken}
}

// Register 注册路由。
func (h *AdminHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("DELETE /admin/users/{login}", h.handleDeleteUser)
}

// handleDeleteUser 按 用户名/邮箱/手机号 删除用户（运维：密码重置/清理）。
func (h *AdminHandler) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if h.adminToken == "" || r.Header.Get("X-Admin-Token") != h.adminToken {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	login := r.PathValue("login")

	user, err := findUserByLoginIn(h.store, login)
	if err != nil {
		slog.Error("admin delete: find user", "login", login, "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}
	if err := h.store.Delete("auth/users", user.ID); err != nil {
		slog.Error("admin delete: delete user", "login", login, "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	slog.Info("admin deleted user", "login", login, "id", user.ID)
	w.WriteHeader(http.StatusNoContent)
}
