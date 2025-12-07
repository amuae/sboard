package database

import (
	"log"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Init 初始化数据库连接
func Init(dbPath string) error {
	var err error

	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return err
	}

	// 获取底层数据库连接并配置
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	// SQLite 优化配置
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 启用外键
	DB.Exec("PRAGMA foreign_keys = ON")

	// 自动迁移数据库表
	if err := autoMigrate(); err != nil {
		return err
	}

	log.Printf("数据库初始化完成: %s", dbPath)
	return nil
}

// autoMigrate 自动迁移数据库表结构
func autoMigrate() error {
	return DB.AutoMigrate(
		&Admin{},
		&ProxyUser{},
		&InboundNode{},
		&NodeUserRelation{},
		&Server{},
		&ServerNodeConfig{},
		&DeployLog{},
		&SystemConfig{},
	)
}

// GetDB 获取数据库实例
func GetDB() *gorm.DB {
	return DB
}
