package api

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/golang-jwt/jwt/v5"
	"github.com/quanttide/qtcloud-auth/internal/model"
)

// Storer 持久化抽象.
type Storer interface {
	List(collection string) ([]byte, error)
	Create(collection string, data []byte) (string, error)
	Get(collection string, id string) ([]byte, error)
	Update(collection string, id string, data []byte) error
}

// AuthHandler OAuth 2.0 认证处理器.
// 验证码、令牌等数据统一走共享存储（FC 多实例可用），不保留实例本地状态。
type AuthHandler struct {
	store           Storer
	secret          []byte // JWT 签名密钥
	smsSender       SMSSender
	oauthTokenStore oauth2.TokenStore // 可选：缺省用内存 TokenStore（单实例）
	oauthMgr        oauth2.Manager
	oauthSrv        *server.Server
}

// NewAuthHandler 创建认证处理器。
// tokenStore 可选：传入则用于令牌/刷新令牌的共享存储，缺省为内存实现（测试、单实例）。
func NewAuthHandler(st Storer, secret string, sender SMSSender, tokenStore ...oauth2.TokenStore) *AuthHandler {
	h := &AuthHandler{
		store:     st,
		secret:    []byte(secret),
		smsSender: sender,
	}
	if len(tokenStore) > 0 {
		h.oauthTokenStore = tokenStore[0]
	}
	return h
}

// ── 用户查找 ──

func (h *AuthHandler) findUserByUsername(username string) (*model.User, error) {
	data, err := h.store.List("auth/users")
	if err != nil {
		return nil, err
	}
	var users []model.User
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, err
	}
	for _, u := range users {
		if u.Username == username {
			return &u, nil
		}
	}
	return nil, nil
}

func (h *AuthHandler) findUserByPhone(phone string) (*model.User, error) {
	data, err := h.store.List("auth/users")
	if err != nil {
		return nil, err
	}
	var users []model.User
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, err
	}
	for _, u := range users {
		if u.Phone == phone {
			return &u, nil
		}
	}
	return nil, nil
}

// findUserByLogin 按 用户名/邮箱/手机号 任一方式查找（密码登录统一入口）。
func (h *AuthHandler) findUserByLogin(login string) (*model.User, error) {
	data, err := h.store.List("auth/users")
	if err != nil {
		return nil, err
	}
	var users []model.User
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, err
	}
	for _, u := range users {
		if u.Username == login || u.Email == login || u.Phone == login {
			return &u, nil
		}
	}
	return nil, nil
}

// ── JWT 令牌验证（供中间件和 UserInfo 使用） ──

// parseToken 解析并验证 Bearer JWT.
func (h *AuthHandler) parseToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return h.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}

// ── 管理员种子 ──

func (h *AuthHandler) EnsureAdmin(password string) error {
	existing, err := h.findUserByUsername("admin")
	if err != nil {
		return err
	}
	if existing != nil {
		slog.Info("admin user already exists, skipping seed")
		return nil
	}

	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	user := model.User{
		Username:     "admin",
		PasswordHash: hash,
		CreatedAt:    time.Now().Format(time.RFC3339),
	}
	data, _ := json.Marshal(user)
	id, err := h.store.Create("auth/users", data)
	if err != nil {
		return err
	}
	user.ID = id
	data, _ = json.Marshal(user)
	if err := h.store.Update("auth/users", id, data); err != nil {
		return err
	}
	slog.Info("admin user created")
	return nil
}
