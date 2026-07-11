package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/quanttide/qtcloud-auth/api"
	"github.com/tidwall/buntdb"
)

func main() {
	// ── 存储 ──
	db, err := buntdb.Open(":memory:")
	if err != nil {
		slog.Error("open db", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	st := &buntdbStorer{db: db}

	// ── 认证处理器 ──
	secret := getEnv("JWT_SECRET", "quanttide-auth-secret")
	handler := api.NewAuthHandler(st, secret, &api.ConsoleSender{})
	handler.SetupOAuth()

	// 种子管理员（密码从环境变量读取，默认 123456）
	adminPass := getEnv("ADMIN_PASSWORD", "123456")
	if err := handler.EnsureAdmin(adminPass); err != nil {
		slog.Error("ensure admin", "error", err)
		os.Exit(1)
	}

	// ── 路由 ──
	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth/token", handler.Token)
	mux.HandleFunc("POST /oauth/sms/send", handler.SendCode)
	mux.HandleFunc("GET /userinfo", handler.AuthMiddleware(http.HandlerFunc(handler.UserInfo)).ServeHTTP)

	// ── 启动 ──
	addr := getEnv("LISTEN_ADDR", ":8080")
	slog.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ── buntdb Storer 实现 ──

type buntdbStorer struct {
	db *buntdb.DB
}

func (s *buntdbStorer) List(collection string) ([]byte, error) {
	var result []byte
	err := s.db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get(collection)
		if err == buntdb.ErrNotFound {
			result = []byte("[]")
			return nil
		}
		if err != nil {
			return err
		}
		result = []byte(val)
		return nil
	})
	return result, err
}

func (s *buntdbStorer) Create(collection string, data []byte) (string, error) {
	id := fmt.Sprintf("%s/%d", collection, time.Now().UnixNano())
	err := s.db.Update(func(tx *buntdb.Tx) error {
		// 读取已有列表
		raw, err := tx.Get(collection)
		if err != nil && err != buntdb.ErrNotFound {
			return err
		}
		var items []map[string]any
		if raw != "" {
			// 追加模式：保留原样，稍后更新
		}
		_ = items
		return tx.Set(collection, string(data), nil)
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *buntdbStorer) Get(collection string, id string) ([]byte, error) {
	var result []byte
	err := s.db.View(func(tx *buntdb.Tx) error {
		// 读取集合，再按 id 查找
		raw, err := tx.Get(collection)
		if err != nil {
			return err
		}
		_ = raw
		// 简化：直接按 id 存储
		val, err := tx.Get(id)
		if err != nil {
			return err
		}
		result = []byte(val)
		return nil
	})
	return result, err
}

func (s *buntdbStorer) Update(collection string, id string, data []byte) error {
	return s.db.Update(func(tx *buntdb.Tx) error {
		return tx.Set(id, string(data), nil)
	})
}
