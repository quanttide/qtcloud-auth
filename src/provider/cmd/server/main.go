package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/quanttide/qtcloud-auth/internal/api"
	"github.com/quanttide/qtcloud-auth/internal/app"
	"github.com/quanttide/qtcloud-auth/internal/store"
)

func main() {
	// ── 存储（DB_DRIVER=postgres 走 DATABASE_URL；默认 SQLite，DB_SQLITE_DSN 控制路径） ──
	db, err := app.OpenDB()
	if err != nil {
		slog.Error("open db", "error", err)
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("get db handle", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	st := store.New(db)

	// 令牌存储（共享，refresh_token 跨实例可用）
	tokenStore, err := store.NewTokenStore(db)
	if err != nil {
		slog.Error("init token store", "error", err)
		os.Exit(1)
	}

	// ── SMS 发送器 ──
	smsDriver := getEnv("SMS_DRIVER", "console")
	var sender api.SMSSender
	switch smsDriver {
	case "console":
		sender = &api.ConsoleSender{}
	case "aliyun":
		as, err := api.NewAliyunSMSSender()
		if err != nil {
			slog.Error("init aliyun sms sender", "error", err)
			os.Exit(1)
		}
		sender = as
	default:
		slog.Error("unknown sms driver", "driver", smsDriver)
		os.Exit(1)
	}

	// ── 认证处理器 ──
	secret := getEnv("JWT_SECRET", "quanttide-auth-secret")
	handler := api.NewAuthHandler(st, secret, sender, tokenStore)
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
	mux.HandleFunc("POST /oauth/register", handler.Register)
	mux.HandleFunc("GET /userinfo", handler.AuthMiddleware(http.HandlerFunc(handler.UserInfo)).ServeHTTP)
	// 运维端点（fail-closed：ADMIN_TOKEN 未配置或请求头不匹配一律 403）
	api.NewAdminHandler(st, getEnv("ADMIN_TOKEN", "")).Register(mux)

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
