package app

import (
	"context"
	"fmt"
	"omniflow/internal/common"
	"time"
)

// InventoryActivities 库存相关
type InventoryActivities struct{}

func (a *InventoryActivities) ReserveInventory(ctx context.Context, order common.Order) error {
	fmt.Printf("[Inventory] 正在预占库存: Order %s, Items: %v\n", order.OrderID, order.Items)
	// 模拟耗时操作 (查数据库)
	time.Sleep(time.Second * 2)
	fmt.Println("[Inventory] 库存预占成功 ✅")
	return nil
}

// PaymentActivities 支付相关
type PaymentActivities struct{}

func (a *PaymentActivities) ProcessPayment(ctx context.Context, order common.Order) error {
	fmt.Printf("[Payment] 正在处理扣款: Order %s, Amount: $%d\n", order.OrderID, order.Amount)
	// 模拟耗时操作 (调银行API)
	time.Sleep(time.Second * 3)

	if order.Amount > 10000 {
		return fmt.Errorf("金额过大，风控拦截") // 模拟一个失败场景
	}

	fmt.Println("[Payment] 扣款成功 ✅")
	return nil
}
