package app

import (
	"omniflow/internal/common"
	"time"

	"go.temporal.io/sdk/workflow"
)

// OrderFulfillmentWorkflow 核心履约流程
func OrderFulfillmentWorkflow(ctx workflow.Context, order common.Order) (*common.OrderStatus, error) {
	// 1. 设置 Activity 选项 (超时、重试策略)
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 1, // 单个 Activity 最长运行时间
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var invActs *InventoryActivities
	var payActs *PaymentActivities

	// ==========================================
	// 步骤 1: 预占库存
	// ==========================================
	// 使用 ExecuteActivity 调用，Temporal 会记录这一步的状态
	err := workflow.ExecuteActivity(ctx, invActs.ReserveInventory, order).Get(ctx, nil)
	if err != nil {
		return &common.OrderStatus{Status: "FAILED", Message: "库存预占失败"}, err
	}

	// ==========================================
	// 步骤 2: 支付扣款
	// ==========================================
	err = workflow.ExecuteActivity(ctx, payActs.ProcessPayment, order).Get(ctx, nil)
	if err != nil {
		// TODO: 这里未来要加 Saga 补偿逻辑 (如退还库存)
		return &common.OrderStatus{Status: "FAILED", Message: "支付失败"}, err
	}

	// ==========================================
	// 步骤 3: 等待发货 (模拟人工介入/异步信号)
	// ==========================================
	workflow.GetLogger(ctx).Info("等待物流发货信号...")

	var trackingNumber string
	signalName := "SIGNAL_SHIPPING_INFO"

	// 阻塞！直到收到信号。这意味着这个 Workflow 可能挂起几天。
	signalChan := workflow.GetSignalChannel(ctx, signalName)
	signalChan.Receive(ctx, &trackingNumber)

	workflow.GetLogger(ctx).Info("收到运单号: " + trackingNumber)

	// ==========================================
	// 完成
	// ==========================================
	return &common.OrderStatus{
		OrderID: order.OrderID,
		Status:  "COMPLETED",
		Message: "订单已完成，运单号: " + trackingNumber,
	}, nil
}
