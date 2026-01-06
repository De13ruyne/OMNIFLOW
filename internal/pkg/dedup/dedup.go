package dedup

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// 私有模型：去重日志表
type idempotencyLog struct {
	IdempotencyKey string `gorm:"primaryKey;type:varchar(128)"`
	CreatedAt      time.Time
}

// AutoMigrate 暴露给 main 函数用于建表
func AutoMigrate(db *gorm.DB) {
	db.AutoMigrate(&idempotencyLog{})
}

// Execute 核心函数：原子性执行 "插入Key" + "业务逻辑"
func Execute(db *gorm.DB, key string, operation func(tx *gorm.DB) error) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// 1. 尝试插入 Key
		logEntry := idempotencyLog{IdempotencyKey: key}
		err := tx.Create(&logEntry).Error

		if err != nil {
			// 2. 如果 Key 已存在 (Duplicate entry)，说明是重试请求 -> 幂等拦截
			if strings.Contains(err.Error(), "Duplicate entry") {
				fmt.Printf("🛡️ [Idempotency] 拦截到重复请求 (Key: %s)，直接返回成功\n", key)
				return nil // 欺骗上层说“成功了”，防止重复执行副作用
			}
			return err // 其他错误正常抛出
		}

		// 3. 执行真正的业务 (使用传入的 tx)
		return operation(tx)
	})
}
