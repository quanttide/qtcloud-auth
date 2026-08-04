package store

import (
	"context"
	"testing"
	"time"

	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/generates"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/models"
	oauthstore "github.com/go-oauth2/oauth2/v4/store"
	jwt "github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-auth/internal/app"
)

func newTestTokenStore(t *testing.T) (*TokenStore, *gorm.DB) {
	t.Helper()
	db, err := app.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	ts, err := NewTokenStore(db)
	if err != nil {
		t.Fatal(err)
	}
	return ts, db
}

func newToken(access, refresh string) *models.Token {
	ti := models.NewToken()
	ti.SetClientID("qtcloud-auth")
	ti.SetUserID("u1")
	now := time.Now()
	ti.SetAccess(access)
	ti.SetAccessCreateAt(now)
	ti.SetAccessExpiresIn(2 * time.Hour)
	ti.SetRefresh(refresh)
	ti.SetRefreshCreateAt(now)
	ti.SetRefreshExpiresIn(7 * 24 * time.Hour)
	return ti
}

func TestTokenStoreRoundTrip(t *testing.T) {
	ts, _ := newTestTokenStore(t)
	ctx := context.Background()
	ti := newToken("access-1", "refresh-1")
	if err := ts.Create(ctx, ti); err != nil {
		t.Fatal(err)
	}

	got, err := ts.GetByAccess(ctx, "access-1")
	if err != nil || got == nil {
		t.Fatalf("GetByAccess: %v, %v", got, err)
	}
	if got.GetUserID() != "u1" || got.GetAccess() != "access-1" || got.GetRefresh() != "refresh-1" {
		t.Errorf("access round trip mismatch: %+v", got)
	}

	got, err = ts.GetByRefresh(ctx, "refresh-1")
	if err != nil || got == nil {
		t.Fatalf("GetByRefresh: %v, %v", got, err)
	}
	if got.GetAccess() != "access-1" || got.GetUserID() != "u1" {
		t.Errorf("refresh round trip mismatch: %+v", got)
	}
}

func TestTokenStoreMissing(t *testing.T) {
	ts, _ := newTestTokenStore(t)
	ctx := context.Background()
	if got, err := ts.GetByAccess(ctx, "nope"); err != nil || got != nil {
		t.Errorf("GetByAccess missing: %v, %v; want nil,nil", got, err)
	}
	if got, err := ts.GetByRefresh(ctx, "nope"); err != nil || got != nil {
		t.Errorf("GetByRefresh missing: %v, %v; want nil,nil", got, err)
	}
}

// 同一 access 二次 Create 必须整体覆盖（同秒内相同 JWT 会碰撞主键，不得报错）。
func TestTokenStoreUpsert(t *testing.T) {
	ts, _ := newTestTokenStore(t)
	ctx := context.Background()
	if err := ts.Create(ctx, newToken("access-1", "refresh-1")); err != nil {
		t.Fatal(err)
	}
	if err := ts.Create(ctx, newToken("access-1", "refresh-1b")); err != nil {
		t.Fatal(err)
	}
	got, _ := ts.GetByAccess(ctx, "access-1")
	if got == nil || got.GetRefresh() != "refresh-1b" {
		t.Errorf("upsert failed: %+v", got)
	}
}

func TestTokenStoreRemove(t *testing.T) {
	ts, _ := newTestTokenStore(t)
	ctx := context.Background()
	if err := ts.Create(ctx, newToken("access-1", "refresh-1")); err != nil {
		t.Fatal(err)
	}
	if err := ts.RemoveByAccess(ctx, "access-1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := ts.GetByAccess(ctx, "access-1"); got != nil {
		t.Error("access should be removed")
	}
	// 键分离语义：摘除 access 不影响 refresh 键（对齐内存实现）
	if got, _ := ts.GetByRefresh(ctx, "refresh-1"); got == nil {
		t.Error("refresh should survive RemoveByAccess")
	}
	if err := ts.RemoveByRefresh(ctx, "refresh-1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := ts.GetByRefresh(ctx, "refresh-1"); got != nil {
		t.Error("refresh should be removed")
	}
}

// 过期语义：按查询的 key 维度判定（access 过期不影响 refresh 使用，反之亦然）。
func TestTokenStoreExpiry(t *testing.T) {
	ts, _ := newTestTokenStore(t)
	ctx := context.Background()

	// 过期 refresh：GetByRefresh 视为不存在并清理
	exp := newToken("access-exp", "refresh-exp")
	exp.SetRefreshCreateAt(time.Now().Add(-2 * time.Hour))
	exp.SetRefreshExpiresIn(time.Hour)
	if err := ts.Create(ctx, exp); err != nil {
		t.Fatal(err)
	}
	if got, _ := ts.GetByRefresh(ctx, "refresh-exp"); got != nil {
		t.Error("expired refresh should be evicted")
	}

	// 过期 access 但 refresh 未过期：GetByAccess 视为不存在，GetByRefresh 仍可用
	ok := newToken("access-exp2", "refresh-ok")
	ok.SetAccessCreateAt(time.Now().Add(-3 * time.Hour))
	ok.SetAccessExpiresIn(2 * time.Hour)
	if err := ts.Create(ctx, ok); err != nil {
		t.Fatal(err)
	}
	if got, _ := ts.GetByAccess(ctx, "access-exp2"); got != nil {
		t.Error("expired access should be evicted")
	}
	if got, _ := ts.GetByRefresh(ctx, "refresh-ok"); got == nil {
		t.Error("valid refresh must survive expired access")
	}
}

// 全链路：go-oauth2 manager 签发 → 刷新轮换 → 旧 refresh 作废、新 refresh 可用；
// 第二个 TokenStore 实例（模拟另一 FC 实例）读取同一 DB 也能完成刷新。
func TestTokenStoreManagerRefresh(t *testing.T) {
	ts, db := newTestTokenStore(t)
	ctx := context.Background()

	mgr := manage.NewDefaultManager()
	mgr.MapTokenStorage(ts)
	mgr.MapAccessGenerate(generates.NewJWTAccessGenerate("", []byte("secret"), jwt.SigningMethodHS256))
	cs := oauthstore.NewClientStore()
	if err := cs.Set("qtcloud-auth", &models.Client{ID: "qtcloud-auth", Public: true}); err != nil {
		t.Fatal(err)
	}
	mgr.MapClientStorage(cs)

	ti, err := mgr.GenerateAccessToken(ctx, oauth2.PasswordCredentials, &oauth2.TokenGenerateRequest{
		ClientID: "qtcloud-auth",
		UserID:   "u1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ti.GetAccess() == "" || ti.GetRefresh() == "" {
		t.Fatal("token generation incomplete")
	}

	// 另一个“实例”：同一 DB 上新建 store，从 ts1 的存储中读令牌
	ts2, err := NewTokenStore(db)
	if err != nil {
		t.Fatal(err)
	}
	mgr2 := manage.NewDefaultManager()
	mgr2.MapTokenStorage(ts2)
	mgr2.MapAccessGenerate(generates.NewJWTAccessGenerate("", []byte("secret"), jwt.SigningMethodHS256))
	mgr2.MapClientStorage(cs)
	// AccessTokenExp 与签发时（默认 2h）不同：即使同一秒内刷新，新 access JWT（exp 不同）也必然不同
	mgr2.SetRefreshTokenCfg(&manage.RefreshingConfig{
		AccessTokenExp:     3 * time.Hour,
		IsGenerateRefresh:  true,
		IsRemoveAccess:     true,
		IsRemoveRefreshing: true,
	})

	ti2, err := mgr2.RefreshAccessToken(ctx, &oauth2.TokenGenerateRequest{Refresh: ti.GetRefresh()})
	if err != nil {
		t.Fatalf("refresh across store instances failed: %v", err)
	}
	if ti2.GetAccess() == ti.GetAccess() {
		t.Error("refresh should issue a new access token")
	}

	// 新 access 可经另一实例的 manager 按 access 键加载（access 键跨 store 可见）
	loaded, err := mgr2.LoadAccessToken(ctx, ti2.GetAccess())
	if err != nil {
		t.Fatalf("new access token not loadable: %v", err)
	}
	if loaded.GetUserID() != "u1" {
		t.Errorf("loaded token mismatch: %+v", loaded)
	}

	// 轮换后：旧 refresh 作废，新 refresh 可用
	if got, _ := ts.GetByRefresh(ctx, ti.GetRefresh()); got != nil {
		t.Error("old refresh token should be revoked after rotation")
	}
	if got, err := ts.GetByRefresh(ctx, ti2.GetRefresh()); err != nil || got == nil {
		t.Errorf("new refresh token should be usable: %v, %v", got, err)
	}
}
