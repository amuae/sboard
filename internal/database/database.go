package database

import (
	"log"
	"os"
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
		Logger: newDatabaseLogger(log.New(os.Stdout, "\r\n", log.LstdFlags)),
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

func newDatabaseLogger(writer logger.Writer) logger.Interface {
	return logger.New(writer, logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	})
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

// migrateUserExtraUUIDs reconciles the legacy UUID1-UUID10 columns with the
// normalized UserExtraUUID table. The normalized table is authoritative when
// both sides contain a value, and missing old records are copied into it.
func migrateUserExtraUUIDs() error {
	var users []ProxyUser
	if err := DB.Find(&users).Error; err != nil {
		return err
	}

	generatedUsers := 0
	updatedUsers := 0
	for _, user := range users {
		var records []UserExtraUUID
		if err := DB.Where("user_id = ?", user.ID).Find(&records).Error; err != nil {
			return err
		}

		tableValues := make(map[int]string, len(records))
		for _, record := range records {
			if record.Slot >= 1 && record.Slot <= 10 && record.UUID != "" {
				tableValues[record.Slot] = record.UUID
			}
		}
		legacyValues := []string{
			user.UUID1, user.UUID2, user.UUID3, user.UUID4, user.UUID5,
			user.UUID6, user.UUID7, user.UUID8, user.UUID9, user.UUID10,
		}

		values := make(map[int]string, 10)
		for slot := 1; slot <= 10; slot++ {
			if value := tableValues[slot]; value != "" {
				values[slot] = value
			} else if value := legacyValues[slot-1]; value != "" {
				values[slot] = value
			}
		}

		// A completely empty user is an old installation. Initialize all ten
		// slots once; persisted values make subsequent starts idempotent.
		if len(values) == 0 {
			for slot := 1; slot <= 10; slot++ {
				values[slot] = uuid.New().String()
			}
			generatedUsers++
		}

		if err := DB.Transaction(func(tx *gorm.DB) error {
			updates := make(map[string]interface{})
			for slot, value := range values {
				var existing UserExtraUUID
				result := tx.Where("user_id = ? AND slot = ?", user.ID, slot).First(&existing)
				switch {
				case result.Error == nil:
					// Keep a previously stored normalized value if one exists.
					if existing.UUID != "" {
						value = existing.UUID
					} else if err := tx.Model(&existing).Update("uuid", value).Error; err != nil {
						return err
					}
				case result.Error == gorm.ErrRecordNotFound:
					if err := tx.Create(&UserExtraUUID{UserID: user.ID, Slot: slot, UUID: value}).Error; err != nil {
						return err
					}
				default:
					return result.Error
				}

				if column, ok := legacyExtraUUIDColumn(slot); ok && legacyValues[slot-1] != value {
					updates[column] = value
				}
			}
			if len(updates) > 0 {
				if err := tx.Model(&ProxyUser{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
					return err
				}
				updatedUsers++
			}
			return nil
		}); err != nil {
			return err
		}
	}

	if generatedUsers > 0 {
		log.Printf("已为 %d 个用户初始化额外 UUID", generatedUsers)
	}
	if updatedUsers > 0 {
		log.Printf("已同步 %d 个用户的额外 UUID", updatedUsers)
	}
	return nil
}
