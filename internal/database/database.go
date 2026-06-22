package database

import (
	"log"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB 数据库实例（向后兼容，推荐使用 GetDB()）
var DB *gorm.DB

// SetDB 设置数据库实例（用于依赖注入/测试）
func SetDB(d *gorm.DB) { DB = d }

// GetDB 获取数据库实例，若未初始化则 panic
func GetDB() *gorm.DB {
	if DB == nil {
		panic("database not initialized, call Init() first")
	}
	return DB
}

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

	// SQLite 性能优化
	DB.Exec("PRAGMA journal_mode = WAL")   // WAL 模式提升并发性能
	DB.Exec("PRAGMA synchronous = NORMAL") // 平衡安全性和性能
	DB.Exec("PRAGMA cache_size = -64000")  // 64MB 缓存
	DB.Exec("PRAGMA busy_timeout = 5000")  // 5秒锁等待超时
	DB.Exec("PRAGMA temp_store = MEMORY")  // 临时表存储在内存

	// 自动迁移数据库表 (迁移时临时禁用外键以支持表重建)
	DB.Exec("PRAGMA foreign_keys = OFF")
	if err := autoMigrate(); err != nil {
		return err
	}
	// 迁移完成后启用外键
	DB.Exec("PRAGMA foreign_keys = ON")

	// 初始化默认的 OAuth 提供商
	if err := InitDefaultOAuthProviders(); err != nil {
		log.Printf("初始化默认 OAuth 提供商失败: %v", err)
	}

	// 初始化默认的订阅配置
	if err := InitDefaultSubscriptionConfigs(); err != nil {
		log.Printf("初始化默认订阅配置失败: %v", err)
	}

	// 预热数据库连接 - 执行简单查询确保连接池已建立
	var count int64
	DB.Model(&Admin{}).Count(&count)

	log.Printf("数据库初始化完成: %s", dbPath)
	return nil
}

// autoMigrate 自动迁移数据库表结构
func autoMigrate() error {
	err := DB.AutoMigrate(
		&Admin{},
		&ProxyUser{},
		&UserExtraUUID{},
		&InboundNode{},
		&NodeUserRelation{},
		&Server{},
		&ServerNodeConfig{},
		&ServerOutbound{},
		&ExternalNode{},
		&DeployLog{},
		&SystemConfig{},
		&OAuthProvider{},
	)
	if err != nil {
		return err
	}

	// 为没有额外 UUID 的老用户补充生成
	return migrateUserExtraUUIDs()
}

// migrateUserExtraUUIDs 为老用户补充生成额外 UUID 并填充 UserExtraUUID 表
func migrateUserExtraUUIDs() error {
	var users []ProxyUser

	// 1. 查找 UUID1 为空的用户（老用户没有额外 UUID），为其生成
	if err := DB.Where("uuid1 IS NULL OR uuid1 = ''").Find(&users).Error; err != nil {
		return err
	}

	for _, user := range users {
		user.UUID1 = uuid.New().String()
		user.UUID2 = uuid.New().String()
		user.UUID3 = uuid.New().String()
		user.UUID4 = uuid.New().String()
		user.UUID5 = uuid.New().String()
		user.UUID6 = uuid.New().String()
		user.UUID7 = uuid.New().String()
		user.UUID8 = uuid.New().String()
		user.UUID9 = uuid.New().String()
		user.UUID10 = uuid.New().String()

		if err := DB.Save(&user).Error; err != nil {
			log.Printf("为用户 %s 生成额外 UUID 失败: %v", user.Name, err)
			continue
		}
		log.Printf("已为用户 %s 生成 10 个额外 UUID", user.Name)

		// 同步到 UserExtraUUID 表
		uuids := []string{user.UUID1, user.UUID2, user.UUID3, user.UUID4, user.UUID5, user.UUID6, user.UUID7, user.UUID8, user.UUID9, user.UUID10}
		for slot, uid := range uuids {
			if uid != "" {
				if err := DB.Create(&UserExtraUUID{UserID: user.ID, Slot: slot + 1, UUID: uid}).Error; err != nil {
					log.Printf("为用户 %s 同步额外 UUID slot %d 到关联表失败: %v", user.Name, slot+1, err)
				}
			}
		}
	}

	// 2. 迁移已有 UUID1-UUID10 但尚未写入 UserExtraUUID 表的用户
	return migrateExistingUserExtraUUIDs()
}

// migrateExistingUserExtraUUIDs 将已有 UUID1-UUID10 的用户数据同步到 UserExtraUUID 表
func migrateExistingUserExtraUUIDs() error {
	var users []ProxyUser
	if err := DB.Where("uuid1 IS NOT NULL AND uuid1 != ''").Find(&users).Error; err != nil {
		return err
	}

	for _, user := range users {
		// 检查是否已有 UserExtraUUID 记录
		var count int64
		DB.Model(&UserExtraUUID{}).Where("user_id = ?", user.ID).Count(&count)
		if count > 0 {
			continue
		}

		uuids := []string{user.UUID1, user.UUID2, user.UUID3, user.UUID4, user.UUID5, user.UUID6, user.UUID7, user.UUID8, user.UUID9, user.UUID10}
		for slot, uid := range uuids {
			if uid != "" {
				if err := DB.Create(&UserExtraUUID{UserID: user.ID, Slot: slot + 1, UUID: uid}).Error; err != nil {
					log.Printf("为用户 %s (ID=%d) 迁移额外 UUID slot %d 失败: %v", user.Name, user.ID, slot+1, err)
				}
			}
		}
		log.Printf("已为用户 %s (ID=%d) 迁移额外 UUID 到关联表", user.Name, user.ID)
	}

	return nil
}
