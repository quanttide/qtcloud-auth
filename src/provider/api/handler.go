package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/golang-jwt/jwt/v5"
	"github.com/quanttide/qtcloud-auth/model"
)

// Storer 持久化抽象.
type Storer interface {
	List(collection string) ([]byte, error)
	Create(collection string, data []byte) (string, error)
	Get(collection string, id string) ([]byte, error)
	Update(collection string, id string, data []byte) error
}

// AuthHandler OAuth 2.0 认证处理器.
type AuthHandler struct {
	store        Storer
	secret       []byte                          // JWT 签名密钥
	smsSender    SMSSender
	codeStore    map[string][]model.VerificationCode
	codeLastSent map[string]time.Time
	codeMu       sync.Mutex
	oauthMgr     oauth2.Manager
	oauthSrv     *server.Server
}

func NewAuthHandler(st Storer, secret string, sender SMSSender) *AuthHandler {
	return &AuthHandler{
		store:        st,
		secret:       []byte(secret),
		smsSender:    sender,
		codeStore:    make(map[string][]model.VerificationCode),
		codeLastSent: make(map[string]time.Time),
	}
}

// hashPassword SHA256(username + ":" + password).
func hashPassword(username, password string) string {
	h := sha256.Sum256([]byte(username + ":" + password))
	return hex.EncodeToString(h[:])
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

	user := model.User{
		Username:     "admin",
		PasswordHash: hashPassword("admin", password),
		CreatedAt:    time.Now().Format(time.RFC3339),
	}
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	id, err := h.store.Create("auth/users", data)
	if err != nil {
		return err
	}
	user.ID = id
	data, err = json.Marshal(user)
	if err != nil {
		return err
	}
	if err := h.store.Update("auth/users", id, data); err != nil {
		return err
	}
	slog.Info("admin user created")
	return nil
}
