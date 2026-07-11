package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/quanttide/qtcloud-auth/api"
	"github.com/tidwall/buntdb"
)

func main() {
	// ── 存储 ──
dbPath := getEnv("DB_PATH", ":memory:")
db, err := buntdb.Open(dbPath)
if err != nil {
	slog.Error("open db", "error", err)
	os.Exit(1)
}
defer db.Close()

slog.Info("database opened", "path", dbPath)

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

const idSeqKey = "sys:id_seq"

func (s *buntdbStorer) nextID() (string, error) {
	var id string
	err := s.db.Update(func(tx *buntdb.Tx) error {
		val, err := tx.Get(idSeqKey)
		n := 0
		if err == nil {
			fmt.Sscanf(val, "%d", &n)
		}
		n++
		id = fmt.Sprintf("u%d", n)
		tx.Set(idSeqKey, fmt.Sprintf("%d", n), nil)
		return nil
	})
	return id, err
}

func (s *buntdbStorer) List(collection string) ([]byte, error) {
	var result []byte
	err := s.db.View(func(tx *buntdb.Tx) error {
		raw, err := tx.Get(colKey(collection))
		if err != nil {
			result = []byte("[]")
			return nil
		}
		var ids []string
		json.Unmarshal([]byte(raw), &ids)

		var items []json.RawMessage
		for _, id := range ids {
			if val, err := tx.Get(id); err == nil {
				items = append(items, json.RawMessage(val))
			}
		}
		result, _ = json.Marshal(items)
		return nil
	})
	return result, err
}

func (s *buntdbStorer) Create(collection string, data []byte) (string, error) {
	id, err := s.nextID()
	if err != nil {
		return "", err
	}
	err = s.db.Update(func(tx *buntdb.Tx) error {
		tx.Set(id, string(data), nil)
		raw, err := tx.Get(colKey(collection))
		var ids []string
		if err == nil {
			json.Unmarshal([]byte(raw), &ids)
		}
		ids = append(ids, id)
		b, _ := json.Marshal(ids)
		tx.Set(colKey(collection), string(b), nil)
		return nil
	})
	return id, err
}

func (s *buntdbStorer) Get(_ string, id string) ([]byte, error) {
	var result []byte
	err := s.db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get(id)
		if err != nil {
			return fmt.Errorf("not found")
		}
		result = []byte(val)
		return nil
	})
	return result, err
}

func (s *buntdbStorer) Update(_ string, id string, data []byte) error {
	return s.db.Update(func(tx *buntdb.Tx) error {
		tx.Set(id, string(data), nil)
		return nil
	})
}

func colKey(collection string) string {
	return "idx:" + collection
}
