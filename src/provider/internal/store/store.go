// Package store 提供基于 GORM 的 api.Storer 实现：每个集合一张表，ID 为 UUID 字符串主键。
// 集合约定与 api 层一致（auth/users、auth/roles、auth/verification_codes）。
package store

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-auth/internal/model"
)

// 集合名（与 api 层 Storer 约定一致）。
const (
	ColUsers = "auth/users"
	ColRoles = "auth/roles"
	ColCodes = "auth/verification_codes"
)

// Store 是 api.Storer 的 GORM 实现。
type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// List 返回集合全部记录（JSON 数组），按创建时间排序。
func (s *Store) List(collection string) ([]byte, error) {
	switch collection {
	case ColUsers:
		var items []model.User
		if err := s.db.Order("created_at, id").Find(&items).Error; err != nil {
			return nil, err
		}
		return json.Marshal(items)
	case ColRoles:
		var items []model.Role
		if err := s.db.Order("id").Find(&items).Error; err != nil {
			return nil, err
		}
		return json.Marshal(items)
	case ColCodes:
		var items []model.VerificationCode
		if err := s.db.Order("created_at, id").Find(&items).Error; err != nil {
			return nil, err
		}
		return json.Marshal(items)
	default:
		return nil, fmt.Errorf("store: unknown collection %q", collection)
	}
}

// Create 插入一条记录并返回其 ID（data 未携带 ID 时自动生成 UUID）。
func (s *Store) Create(collection string, data []byte) (string, error) {
	switch collection {
	case ColUsers:
		var u model.User
		if err := json.Unmarshal(data, &u); err != nil {
			return "", err
		}
		if u.ID == "" {
			u.ID = uuid.New().String()
		}
		if err := s.db.Create(&u).Error; err != nil {
			return "", err
		}
		return u.ID, nil
	case ColRoles:
		var r model.Role
		if err := json.Unmarshal(data, &r); err != nil {
			return "", err
		}
		if r.ID == "" {
			r.ID = uuid.New().String()
		}
		if err := s.db.Create(&r).Error; err != nil {
			return "", err
		}
		return r.ID, nil
	case ColCodes:
		var vc model.VerificationCode
		if err := json.Unmarshal(data, &vc); err != nil {
			return "", err
		}
		if vc.ID == "" {
			vc.ID = uuid.New().String()
		}
		if err := s.db.Create(&vc).Error; err != nil {
			return "", err
		}
		return vc.ID, nil
	default:
		return "", fmt.Errorf("store: unknown collection %q", collection)
	}
}

// Get 按 ID 返回单条记录（JSON）。
func (s *Store) Get(collection string, id string) ([]byte, error) {
	switch collection {
	case ColUsers:
		var u model.User
		if err := s.db.First(&u, "id = ?", id).Error; err != nil {
			return nil, err
		}
		return json.Marshal(u)
	case ColRoles:
		var r model.Role
		if err := s.db.First(&r, "id = ?", id).Error; err != nil {
			return nil, err
		}
		return json.Marshal(r)
	case ColCodes:
		var vc model.VerificationCode
		if err := s.db.First(&vc, "id = ?", id).Error; err != nil {
			return nil, err
		}
		return json.Marshal(vc)
	default:
		return nil, fmt.Errorf("store: unknown collection %q", collection)
	}
}

// Update 按 ID 覆盖整条记录（data 未携带 ID 时使用参数 id）。
func (s *Store) Update(collection string, id string, data []byte) error {
	switch collection {
	case ColUsers:
		var u model.User
		if err := json.Unmarshal(data, &u); err != nil {
			return err
		}
		if u.ID == "" {
			u.ID = id
		}
		return s.db.Save(&u).Error
	case ColRoles:
		var r model.Role
		if err := json.Unmarshal(data, &r); err != nil {
			return err
		}
		if r.ID == "" {
			r.ID = id
		}
		return s.db.Save(&r).Error
	case ColCodes:
		var vc model.VerificationCode
		if err := json.Unmarshal(data, &vc); err != nil {
			return err
		}
		if vc.ID == "" {
			vc.ID = id
		}
		return s.db.Save(&vc).Error
	default:
		return fmt.Errorf("store: unknown collection %q", collection)
	}
}

// Delete 按 ID 删除一条记录。
func (s *Store) Delete(collection string, id string) error {
	switch collection {
	case ColUsers:
		return s.db.Delete(&model.User{}, "id = ?", id).Error
	case ColRoles:
		return s.db.Delete(&model.Role{}, "id = ?", id).Error
	case ColCodes:
		return s.db.Delete(&model.VerificationCode{}, "id = ?", id).Error
	default:
		return fmt.Errorf("store: unknown collection %q", collection)
	}
}
