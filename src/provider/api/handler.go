package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/quanttide/qtcloud-auth/auth"
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
	secret       string
	smsSender    SMSSender
	codeStore    map[string][]model.VerificationCode
	codeLastSent map[string]time.Time
	codeMu       sync.Mutex
}

func NewAuthHandler(st Storer, secret string, sender SMSSender) *AuthHandler {
	return &AuthHandler{
		store:        st,
		secret:       secret,
		smsSender:    sender,
		codeStore:    make(map[string][]model.VerificationCode),
		codeLastSent: make(map[string]time.Time),
	}
}

// authResponse OAuth 2.0 标准 token 响应.
type authResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

func hashPassword(username, password string) string {
	h := sha256.Sum256([]byte(username + ":" + password))
	return hex.EncodeToString(h[:])
}

// internal claims
const (
	accessTokenTTL  = 1 * time.Hour
	refreshTokenTTL = 30 * 24 * time.Hour
)

func (h *AuthHandler) issueTokens(sub, role, phone string) authResponse {
	now := time.Now()
	accessClaims := map[string]any{
		"sub":   sub,
		"role":  role,
		"phone": phone,
		"exp":   now.Add(accessTokenTTL).Unix(),
	}
	accessToken, _ := auth.Sign(accessClaims, h.secret)

	refreshClaims := map[string]any{
		"sub":  sub,
		"type": "refresh_token",
		"exp":  now.Add(refreshTokenTTL).Unix(),
	}
	refreshToken, _ := auth.Sign(refreshClaims, h.secret)

	return authResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTokenTTL.Seconds()),
		RefreshToken: refreshToken,
	}
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
