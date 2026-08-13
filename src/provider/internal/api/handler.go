package api

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"errors"
	"encoding/pem"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/golang-jwt/jwt/v5"
	"github.com/quanttide/qtcloud-auth/internal/model"
	"gorm.io/gorm"
)

// Storer 持久化抽象.
type Storer interface {
	List(collection string) ([]byte, error)
	Create(collection string, data []byte) (string, error)
	Get(collection string, id string) ([]byte, error)
	Update(collection string, id string, data []byte) error
	Delete(collection string, id string) error
}

// AuthHandler OAuth 2.0 认证处理器.
// 验证码、令牌等数据统一走共享存储（FC 多实例可用），不保留实例本地状态。
type AuthHandler struct {
	store           Storer
	key             *rsa.PrivateKey // JWT RS256 签名私钥（签发）；验签用其公钥
	keyPEM          []byte          // 原始 PEM（go-oauth2 签发用）
	smsSender       SMSSender
	oauthTokenStore oauth2.TokenStore // 可选：缺省用内存 TokenStore（单实例）
	oauthMgr        oauth2.Manager
	oauthSrv        *server.Server
}

// NewAuthHandler 创建认证处理器。
// key PEM 为 RSA 私钥（RS256 签发与验签）；tokenStore 可选（共享存储，缺省内存）。
func NewAuthHandler(st Storer, keyPEM string, sender SMSSender, tokenStore ...oauth2.TokenStore) (*AuthHandler, error) {
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return nil, fmt.Errorf("auth: invalid JWT private key PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("auth: parse JWT private key: %w", err)
		}
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("auth: JWT key must be RSA")
	}
	h := &AuthHandler{
		store:     st,
		key:       rsaKey,
		keyPEM:    []byte(keyPEM),
		smsSender: sender,
	}
	if len(tokenStore) > 0 {
		h.oauthTokenStore = tokenStore[0]
	}
	return h, nil
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

// findUserByLoginIn 按 用户名/邮箱/手机号 任一方式查找（密码登录统一入口）。
func findUserByLoginIn(st Storer, login string) (*model.User, error) {
	data, err := st.List("auth/users")
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

func (h *AuthHandler) findUserByLogin(login string) (*model.User, error) {
	return findUserByLoginIn(h.store, login)
}

// findUserByID 按用户 ID 查询。
func (h *AuthHandler) findUserByID(id string) (*model.User, error) {
	data, err := h.store.Get("auth/users", id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var u model.User
	if err := json.Unmarshal(data, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// ── JWT 令牌验证（供中间件和 UserInfo 使用） ──

// parseToken 解析并验证 Bearer JWT（RS256，公钥验签）。
func (h *AuthHandler) parseToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return &h.key.PublicKey, nil
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
