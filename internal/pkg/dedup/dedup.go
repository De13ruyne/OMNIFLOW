package dedup

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type idempotencyLog struct {
	IdempotencyKey string `gorm:"primaryKey;type:varchar(128)"`
	CreatedAt      time.Time
}

func AutoMigrate(db *gorm.DB) {
	db.AutoMigrate(&idempotencyLog{})
}

func Execute(db *gorm.DB, key string, operation func(tx *gorm.DB) error) error {
	return db.Transaction(func(tx *gorm.DB) error {
		logEntry := idempotencyLog{IdempotencyKey: key}
		err := tx.Create(&logEntry).Error

		if err != nil {
			// 🔥 修复点：同时兼容 MySQL ("Duplicate entry") 和 SQLite ("UNIQUE constraint failed")
			errMsg := err.Error()
			if strings.Contains(errMsg, "Duplicate entry") ||
				strings.Contains(errMsg, "UNIQUE constraint failed") {
				fmt.Printf("🛡️ [Idempotency] 拦截到重复请求 (Key: %s)\n", key)
				return nil
			}
			return err
		}

		return operation(tx)
	})
}
