package app

import (
	"context"
	"omniflow/internal/common"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 定义一个临时的去重表结构，用于测试中自动建表
// 这样不需要引用 internal/pkg/dedup，避免循环依赖
type testIdempotencyLog struct {
	IdempotencyKey string `gorm:"primaryKey;type:varchar(128)"`
	CreatedAt      time.Time
}

// 初始化内存数据库
func setupTestDB() *gorm.DB {
	// 使用内存数据库
	db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})

	// 建表：商品表 + 幂等性日志表
	db.AutoMigrate(&Product{})
	db.AutoMigrate(&testIdempotencyLog{})
	// 注意：上面的 testIdempotencyLog 表名默认是 test_idempotency_logs
	// 但我们的 dedup 包里用的是 idempotency_logs
	// 为了测试通过，我们手动指定表名，或者简单点：
	// 直接让 Activity 里的 dedup 包去建表。
	// 这里为了简化，我们假设 dedup 包里的 Execute 会自动处理，或者我们在 setup 里不做 dedup 表的各种复杂配置。
	// *修正*：为了让 dedup 包能工作，我们需要让它把数据写进这个 DB。
	// dedup.Execute 内部使用 db.Create(&logEntry)，只要表存在就行。
	// 我们手动创建一下 dedup 需要的那张表：
	db.Exec("CREATE TABLE idempotency_logs (idempotency_key varchar(128) PRIMARY KEY, created_at datetime)")

	return db
}

func TestReserveInventory_Success(t *testing.T) {
	db := setupTestDB()
	// 准备数据
	db.Create(&Product{ID: "TEST_ITEM", Name: "Test", Stock: 10})

	acts := &InventoryActivities{DB: db}
	order := common.Order{
		OrderID: "ORDER_001",
		Items:   []string{"TEST_ITEM"},
	}

	// 执行
	err := acts.ReserveInventory(context.Background(), order)

	// 验证
	assert.NoError(t, err)
	var p Product
	db.First(&p, "id = ?", "TEST_ITEM")
	assert.Equal(t, 9, p.Stock)
}

func TestReserveInventory_InsufficientStock(t *testing.T) {
	db := setupTestDB()
	db.Create(&Product{ID: "NO_STOCK_ITEM", Stock: 0})

	acts := &InventoryActivities{DB: db}
	order := common.Order{
		OrderID: "ORDER_002",
		Items:   []string{"NO_STOCK_ITEM"},
	}

	err := acts.ReserveInventory(context.Background(), order)

	assert.Error(t, err)
	// 这里的错误信息要和你代码里 fmt.Errorf 的内容一致
	assert.Contains(t, err.Error(), "库存不足")
}

func TestReserveInventory_Idempotency(t *testing.T) {
	db := setupTestDB()
	db.Create(&Product{ID: "ITEM_X", Stock: 10})

	acts := &InventoryActivities{DB: db}
	order := common.Order{OrderID: "ORDER_RETRY", Items: []string{"ITEM_X"}}

	// 第一次执行
	err := acts.ReserveInventory(context.Background(), order)
	assert.NoError(t, err)

	// 第二次执行 (模拟重试)
	err = acts.ReserveInventory(context.Background(), order)
	assert.NoError(t, err)

	// 验证：库存只能扣一次 (10 - 1 = 9)，不能是 8
	var p Product
	db.First(&p, "id = ?", "ITEM_X")
	assert.Equal(t, 9, p.Stock, "幂等性失效：库存被重复扣减")
}
