package api

// 运维管理端点（admin）：删除用户、设置用户角色。
// fail-closed：两条鉴权路径都不通过一律 403。
//
// 鉴权路径：
//  1. Bearer JWT → 用户角色含 "admin" 权限（推荐：每个管理员独立账号，可单独撤销）
//  2. X-Admin-Token 共享口令（兼容旧部署，仅当 ADMIN_TOKEN 已配置时生效）

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/quanttide/qtcloud-auth/internal/model"
)

// AdminRoleID 内置管理员角色；AdminPermission 为运维端点所需权限。
const (
	AdminRoleID     = "admin"
	AdminPermission = "admin"
)

// AdminHandler 运维管理 API。
type AdminHandler struct {
	store      Storer
	auth       *AuthHandler // JWT 验签与用户查找（角色鉴权路径）
	adminToken string       // 兼容路径：共享口令，空则禁用
}

func NewAdminHandler(st Storer, auth *AuthHandler, adminToken string) *AdminHandler {
	return &AdminHandler{store: st, auth: auth, adminToken: adminToken}
}

// Register 注册路由。
func (h *AdminHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("DELETE /admin/users/{login}", h.handleDeleteUser)
	mux.HandleFunc("PUT /admin/users/{login}/role", h.handleSetRole)
}

// authorize 鉴权并返回操作者标识（管理员用户名或 "admin-token"）。
// 已写响应表示失败（403），调用方直接 return。
func (h *AdminHandler) authorize(w http.ResponseWriter, r *http.Request) (string, bool) {
	// 路径 1：管理员账号 JWT
	if tokenStr := extractBearerToken(r); tokenStr != "" && h.auth != nil {
		if claims, err := h.auth.parseToken(tokenStr); err == nil {
			if sub, _ := claims["sub"].(string); sub != "" {
				if user, err := h.auth.findUserByID(sub); err == nil && user != nil && h.hasAdminPermission(user) {
					return user.Username, true
				}
			}
		}
	}
	// 路径 2：共享 ADMIN_TOKEN（兼容旧部署）
	if h.adminToken != "" && r.Header.Get("X-Admin-Token") == h.adminToken {
		return "admin-token", true
	}
	http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	return "", false
}

// hasAdminPermission 判断用户角色是否含 "admin" 权限。
func (h *AdminHandler) hasAdminPermission(user *model.User) bool {
	if user.RoleID == "" {
		return false
	}
	data, err := h.store.Get("auth/roles", user.RoleID)
	if err != nil {
		return false
	}
	var role model.Role
	if err := json.Unmarshal(data, &role); err != nil {
		return false
	}
	for _, p := range role.Permissions {
		if p == AdminPermission {
			return true
		}
	}
	return false
}

// handleDeleteUser 按 用户名/邮箱/手机号 删除用户。
func (h *AdminHandler) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.authorize(w, r)
	if !ok {
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
	slog.Info("admin deleted user", "actor", actor, "login", login, "id", user.ID)
	w.WriteHeader(http.StatusNoContent)
}

// handleSetRole PUT /admin/users/{login}/role 设置用户角色（body: {"role":"admin"}；role 为空表示撤销）。
func (h *AdminHandler) handleSetRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.authorize(w, r)
	if !ok {
		return
	}
	login := r.PathValue("login")
	// 防自锁：不允许改自己的角色（否则最后一步撤销自己的 admin 会把自己锁在门外）
	if login == actor {
		http.Error(w, `{"error":"cannot modify own role"}`, http.StatusForbidden)
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	user, err := findUserByLoginIn(h.store, login)
	if err != nil {
		slog.Error("admin set role: find user", "login", login, "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}
	if req.Role != "" {
		if _, err := h.store.Get("auth/roles", req.Role); err != nil {
			http.Error(w, `{"error":"role not found"}`, http.StatusBadRequest)
			return
		}
	}

	user.RoleID = req.Role
	data, mErr := json.Marshal(user)
	if mErr != nil {
		slog.Error("admin set role: marshal user", "login", login, "error", mErr)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if err := h.store.Update("auth/users", user.ID, data); err != nil {
		slog.Error("admin set role: update user", "login", login, "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	slog.Info("admin set role", "actor", actor, "login", login, "role", req.Role)
	w.WriteHeader(http.StatusNoContent)
}
