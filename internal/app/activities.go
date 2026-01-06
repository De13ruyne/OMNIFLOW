package app

import (
	"context"
	"fmt"
	"omniflow/internal/common"
	"time"
)

type InventoryActivities struct{}

func (a *InventoryActivities) ReserveInventory(ctx context.Context, order common.Order) error {
	fmt.Printf("📦 [Inventory] 锁定库存: Order=%s, Items=%v\n", order.OrderID, order.Items)
	time.Sleep(time.Second * 1) // 模拟DB操作
	return nil
}

func (a *InventoryActivities) ReleaseInventory(ctx context.Context, order common.Order) error {
	fmt.Printf("🔄 [Inventory] 释放库存 (补偿): Order=%s\n", order.OrderID)
	time.Sleep(time.Second * 1)
	return nil
}

// 这里可以继续扩展 PaymentActivities, ShippingActivities 等
