// Package app 提供数据库打开与迁移（对齐 qtcloud-pay 的 internal/app 模式）。
package app

import (
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-auth/model"
)

// Open 打开数据库并迁移全部模型（driver: sqlite/postgres，dsn 对应驱动格式）。
func Open(driver, dsn string) (*gorm.DB, error) {
	var (
		db  *gorm.DB
		err error
	)
	switch driver {
	case "postgres":
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	default: // sqlite（开发默认）
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{TranslateError: true})
	}
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Role{}, &model.VerificationCode{},
	); err != nil {
		return nil, err
	}
	if driver != "postgres" {
		// SQLite 单写者：限制单连接，避免并发写事务触发 database is locked
		sqlDB, err := db.DB()
		if err != nil {
			return nil, err
		}
		sqlDB.SetMaxOpenConns(1)
	}
	return db, nil
}

// OpenDB 按环境变量（DB_DRIVER / DATABASE_URL / DB_SQLITE_DSN）打开数据库。
func OpenDB() (*gorm.DB, error) {
	if os.Getenv("DB_DRIVER") == "postgres" {
		return Open("postgres", os.Getenv("DATABASE_URL"))
	}
	dsn := os.Getenv("DB_SQLITE_DSN")
	if dsn == "" {
		dsn = "qtcloud-auth.db"
	}
	return Open("sqlite", dsn)
}
