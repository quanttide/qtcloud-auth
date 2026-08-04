package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// oauthToken 令牌持久化行。
// access / refresh 为可空唯一索引键：RemoveBy* 只摘除对应键（对齐官方内存实现的
// access→basicID / refresh→basicID 分离键语义），行由过期清理回收。
type oauthToken struct {
	ID               int64      `gorm:"primaryKey;autoIncrement"`
	Access           string     `gorm:"uniqueIndex;default:NULL"`
	Refresh          string     `gorm:"uniqueIndex;default:NULL"`
	Data             string     // models.Token JSON（与 go-oauth2 官方内存实现一致）
	AccessExpiresAt  *time.Time // 按 key 维度模拟 TTL 过期清理
	RefreshExpiresAt *time.Time
}

// TokenStore 是 oauth2.TokenStore 的 GORM 实现（共享存储，FC 多实例可用）。
// 对齐官方 MemoryTokenStore 语义：Create 幂等（upsert）、按 key 过期即摘除并视为不存在。
type TokenStore struct {
	db *gorm.DB
}

// NewTokenStore 创建并迁移令牌表。
func NewTokenStore(db *gorm.DB) (*TokenStore, error) {
	if err := db.AutoMigrate(&oauthToken{}); err != nil {
		return nil, err
	}
	return &TokenStore{db: db}, nil
}

// Create 幂等写入令牌信息（access 冲突时整体覆盖，对齐内存实现的 set 语义）。
// 顺带清理已过期/无键记录，防止无界增长。
func (s *TokenStore) Create(_ context.Context, info oauth2.TokenInfo) error {
	jv, err := json.Marshal(info)
	if err != nil {
		return err
	}
	row := oauthToken{
		Access:  info.GetAccess(),
		Refresh: info.GetRefresh(),
		Data:    string(jv),
	}
	if d := info.GetAccessExpiresIn(); d != 0 {
		t := info.GetAccessCreateAt().Add(d)
		row.AccessExpiresAt = &t
	}
	if d := info.GetRefreshExpiresIn(); d != 0 {
		t := info.GetRefreshCreateAt().Add(d)
		row.RefreshExpiresAt = &t
	}

	now := time.Now()
	if err := s.db.Where(
		"(access_expires_at IS NOT NULL AND access_expires_at < ?) OR (refresh_expires_at IS NOT NULL AND refresh_expires_at < ?) OR (access IS NULL AND refresh IS NULL)",
		now, now,
	).Delete(&oauthToken{}).Error; err != nil {
		return err
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "access"}},
		UpdateAll: true,
	}).Create(&row).Error
}

// GetByAccess 按 access token 查令牌信息；已过期按 TTL 语义摘除该键并视为不存在。
func (s *TokenStore) GetByAccess(_ context.Context, access string) (oauth2.TokenInfo, error) {
	return s.getBy("access", access, "access_expires_at")
}

// GetByRefresh 按 refresh token 查令牌信息；已过期按 TTL 语义摘除该键并视为不存在。
func (s *TokenStore) GetByRefresh(_ context.Context, refresh string) (oauth2.TokenInfo, error) {
	return s.getBy("refresh", refresh, "refresh_expires_at")
}

// RemoveByAccess 摘除指定 access token 对应的键（不影响该行的 refresh 键）。
func (s *TokenStore) RemoveByAccess(_ context.Context, access string) error {
	return s.db.Model(&oauthToken{}).Where("access = ?", access).Update("access", nil).Error
}

// RemoveByRefresh 摘除指定 refresh token 对应的键（不影响该行的 access 键）。
func (s *TokenStore) RemoveByRefresh(_ context.Context, refresh string) error {
	return s.db.Model(&oauthToken{}).Where("refresh = ?", refresh).Update("refresh", nil).Error
}

// RemoveByCode / GetByCode：授权码流程未启用（无 /oauth/authorize 端点），保持空实现。
func (s *TokenStore) RemoveByCode(_ context.Context, code string) error { return nil }

func (s *TokenStore) GetByCode(_ context.Context, code string) (oauth2.TokenInfo, error) {
	return nil, nil
}

func (s *TokenStore) getBy(key, value, expiresCol string) (oauth2.TokenInfo, error) {
	var row oauthToken
	err := s.db.Where(key+" = ?", value).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// 按查询的 key 维度判断过期（access 过期不影响 refresh 使用，反之亦然）
	if t := s.expiresAt(&row, expiresCol); t != nil && t.Before(time.Now()) {
		_ = s.db.Model(&oauthToken{}).Where(key+" = ?", value).Update(key, nil).Error
		return nil, nil
	}

	var tm models.Token
	if err := json.Unmarshal([]byte(row.Data), &tm); err != nil {
		return nil, err
	}
	return &tm, nil
}

func (s *TokenStore) expiresAt(row *oauthToken, col string) *time.Time {
	switch col {
	case "access_expires_at":
		return row.AccessExpiresAt
	default:
		return row.RefreshExpiresAt
	}
}
