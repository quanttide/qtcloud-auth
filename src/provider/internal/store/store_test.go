package store

import (
	"encoding/json"
	"testing"

	"github.com/quanttide/qtcloud-auth/internal/app"
	"github.com/quanttide/qtcloud-auth/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := app.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	return New(db)
}

// 覆盖 EnsureAdmin 流程：Create（无 ID）→ Update（带 ID）→ Get → List。
func TestUserCRUD(t *testing.T) {
	s := newTestStore(t)

	user := model.User{Username: "admin", PasswordHash: "hash", CreatedAt: "2026-08-04T00:00:00Z"}
	data, _ := json.Marshal(user)
	id, err := s.Create(ColUsers, data)
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("empty id")
	}

	// Update 回填 ID（对齐 api 层 EnsureAdmin 的 Create→Update 两段式）
	user.ID = id
	data, _ = json.Marshal(user)
	if err := s.Update(ColUsers, id, data); err != nil {
		t.Fatal(err)
	}

	got, err := s.Get(ColUsers, id)
	if err != nil {
		t.Fatal(err)
	}
	var u model.User
	if err := json.Unmarshal(got, &u); err != nil {
		t.Fatal(err)
	}
	if u.ID != id || u.Username != "admin" || u.PasswordHash != "hash" {
		t.Errorf("round trip mismatch: %+v", u)
	}

	// List 返回 JSON 数组
	list, err := s.List(ColUsers)
	if err != nil {
		t.Fatal(err)
	}
	var users []model.User
	if err := json.Unmarshal(list, &users); err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].ID != id {
		t.Errorf("list mismatch: %+v", users)
	}
}

// tokenSMS 流程：Create（无 ID）后不 Update，Get 必须仍返回完整记录。
func TestUserCreateWithoutUpdate(t *testing.T) {
	s := newTestStore(t)

	user := model.User{Username: "13800138000", Phone: "13800138000", CreatedAt: "2026-08-04T00:00:00Z"}
	data, _ := json.Marshal(user)
	id, err := s.Create(ColUsers, data)
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.Get(ColUsers, id)
	if err != nil {
		t.Fatal(err)
	}
	var u model.User
	if err := json.Unmarshal(got, &u); err != nil {
		t.Fatal(err)
	}
	if u.ID != id || u.Phone != "13800138000" {
		t.Errorf("round trip mismatch: %+v", u)
	}
}

// 验证码：Create 顺序即 List 顺序，Update 标记 Used 后可读回。
func TestCodeCRUD(t *testing.T) {
	s := newTestStore(t)

	for i := 0; i < 2; i++ {
		vc := model.VerificationCode{
			Phone:     "13800138000",
			Code:      "123456",
			ExpiresAt: "2026-08-04T01:00:00Z",
			CreatedAt: "2026-08-04T00:00:0" + string(rune('0'+i)) + "Z",
		}
		data, _ := json.Marshal(vc)
		if _, err := s.Create(ColCodes, data); err != nil {
			t.Fatal(err)
		}
	}

	list, err := s.List(ColCodes)
	if err != nil {
		t.Fatal(err)
	}
	var codes []model.VerificationCode
	if err := json.Unmarshal(list, &codes); err != nil {
		t.Fatal(err)
	}
	if len(codes) != 2 {
		t.Fatalf("want 2 codes, got %d", len(codes))
	}
	if codes[0].CreatedAt > codes[1].CreatedAt {
		t.Error("codes not ordered by created_at")
	}

	// 标记已使用（对齐 markCodeUsed）
	first := codes[0]
	first.Used = true
	data, _ := json.Marshal(first)
	if err := s.Update(ColCodes, first.ID, data); err != nil {
		t.Fatal(err)
	}

	got, err := s.Get(ColCodes, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	var updated model.VerificationCode
	if err := json.Unmarshal(got, &updated); err != nil {
		t.Fatal(err)
	}
	if !updated.Used {
		t.Error("used flag not persisted")
	}
}

// Role 的 []string Permissions 经 serializer:json 序列化后必须可读回。
func TestRolePermissions(t *testing.T) {
	s := newTestStore(t)

	role := model.Role{ID: "role-1", Name: "admin", Permissions: []string{"user:read", "user:write"}}
	data, _ := json.Marshal(role)
	id, err := s.Create(ColRoles, data)
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.Get(ColRoles, id)
	if err != nil {
		t.Fatal(err)
	}
	var r model.Role
	if err := json.Unmarshal(got, &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Permissions) != 2 || r.Permissions[0] != "user:read" {
		t.Errorf("permissions round trip mismatch: %+v", r.Permissions)
	}
}

func TestUnknownCollection(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.List("nope"); err == nil {
		t.Error("List: want error for unknown collection")
	}
	if _, err := s.Create("nope", []byte(`{}`)); err == nil {
		t.Error("Create: want error for unknown collection")
	}
	if _, err := s.Get("nope", "x"); err == nil {
		t.Error("Get: want error for unknown collection")
	}
	if err := s.Update("nope", "x", []byte(`{}`)); err == nil {
		t.Error("Update: want error for unknown collection")
	}
}

func TestGetNotFound(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Get(ColUsers, "missing"); err == nil {
		t.Error("want error for missing record")
	}
}

// Create 时 data 已携带 ID 则保留（幂等，重试安全）。
func TestCreatePreservesProvidedID(t *testing.T) {
	s := newTestStore(t)
	user := model.User{ID: "fixed-id", Username: "u"}
	data, _ := json.Marshal(user)
	id, err := s.Create(ColUsers, data)
	if err != nil {
		t.Fatal(err)
	}
	if id != "fixed-id" {
		t.Errorf("id = %q, want fixed-id", id)
	}
}
