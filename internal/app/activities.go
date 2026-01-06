package app

import (
	"context"
	"fmt"
	"omniflow/internal/common"
	"omniflow/internal/pkg/dedup"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Product 商品表模型
type Product struct {
	ID    string `gorm:"primaryKey"` // e.g. "iPhone15"
	Name  string
	Stock int
	Price int
}

type InventoryActivities struct {
	DB *gorm.DB
}

// 1. 预占库存 (幂等 + 悲观锁)
func (a *InventoryActivities) ReserveInventory(ctx context.Context, order common.Order) error {
	// 生成去重键：订单号 + 动作
	idemKey := fmt.Sprintf("order_%s_reserve", order.OrderID)
	fmt.Printf("📦 [Inventory] 请求预占: %s\n", order.OrderID)

	// 使用 dedup 中间件
	return dedup.Execute(a.DB, idemKey, func(tx *gorm.DB) error {
		for _, itemID := range order.Items {
			var product Product

			// 🔥 核心技术点：FOR UPDATE 悲观锁，防止超卖
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&product, "id = ?", itemID).Error; err != nil {
				return fmt.Errorf("商品 %s 不存在", itemID)
			}

			if product.Stock < 1 {
				return fmt.Errorf("商品 %s 库存不足", itemID)
			}

			product.Stock -= 1
			if err := tx.Save(&product).Error; err != nil {
				return err
			}
		}
		fmt.Printf("✅ [Inventory] 数据库扣减成功 (剩余: %d)\n", -1) // 简化log
		return nil
	})
}

// 2. 释放库存 (幂等)
func (a *InventoryActivities) ReleaseInventory(ctx context.Context, order common.Order) error {
	idemKey := fmt.Sprintf("order_%s_release", order.OrderID)
	fmt.Printf("🔄 [Inventory] 请求回滚: %s\n", order.OrderID)

	return dedup.Execute(a.DB, idemKey, func(tx *gorm.DB) error {
		for _, itemID := range order.Items {
			if err := tx.Model(&Product{}).Where("id = ?", itemID).
				Update("stock", gorm.Expr("stock + ?", 1)).Error; err != nil {
				return err
			}
		}
		fmt.Println("✅ [Inventory] 库存已回滚")
		return nil
	})
}

// --- 简单的发货 Activity ---
type ShippingActivities struct{}

func (a *ShippingActivities) GenerateShippingLabel(ctx context.Context, shipment common.Shipment) (string, error) {
	time.Sleep(time.Second * 1) // 模拟打单
	label := fmt.Sprintf("SF-%s-%d", shipment.Warehouse, time.Now().UnixMilli())
	return label, nil
}
