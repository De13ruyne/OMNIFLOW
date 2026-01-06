package app

import (
	"omniflow/internal/common"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func OrderFulfillmentWorkflow(ctx workflow.Context, order common.Order) (*common.OrderStatus, error) {
	// 1. 配置 Activity 选项
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 1,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	logger := workflow.GetLogger(ctx)

	// 2. 初始化状态与查询 Handler
	currentState := "初始化中..."
	// 设置查询处理函数，允许外部查询 currentState
	if err := workflow.SetQueryHandler(ctx, "get_order_status", func() (string, error) {
		return currentState, nil
	}); err != nil {
		return nil, err
	}

	var invActs *InventoryActivities
	// 定义补偿堆栈 (用于 Saga 回滚)
	var compensations []func(workflow.Context) error

	// ------------------------------------------------------------------
	// Step 1: 预占库存
	// ------------------------------------------------------------------
	currentState = "正在预占库存"
	err := workflow.ExecuteActivity(ctx, invActs.ReserveInventory, order).Get(ctx, nil)
	if err != nil {
		currentState = "库存预占失败"
		return &common.OrderStatus{Status: "FAILED", Message: err.Error()}, err
	}

	// ✅ 成功后立即注册补偿：如果后续失败，需要释放库存
	compensations = append(compensations, func(ctx workflow.Context) error {
		return workflow.ExecuteActivity(ctx, invActs.ReleaseInventory, order).Get(ctx, nil)
	})

	// ------------------------------------------------------------------
	// Step 2: 人工风控审核 (仅针对大额订单 > 10000)
	// ------------------------------------------------------------------
	if order.Amount > 10000 {
		currentState = "⚠️ 待风控审核 (大额订单)"
		logger.Info("触发风控，等待管理员审核...")

		var adminAction string
		signalChan := workflow.GetSignalChannel(ctx, "SIGNAL_ADMIN_ACTION")
		signalChan.Receive(ctx, &adminAction) // 阻塞等待信号

		if adminAction == "REJECT" {
			currentState = "审核拒绝，正在回滚..."
			// 执行补偿
			rollback(ctx, compensations)
			currentState = "已关闭 (风控拒绝)"
			return &common.OrderStatus{Status: "REJECTED", Message: "管理员拒绝"}, nil
		}
		logger.Info("管理员审核通过")
	}

	// ------------------------------------------------------------------
	// Step 3: 等待支付 (超时自动取消)
	// ------------------------------------------------------------------
	currentState = "待支付 (超时倒计时: 30s)"
	logger.Info("等待支付信号...")

	var paymentSignal string
	paymentCh := workflow.GetSignalChannel(ctx, "SIGNAL_PAYMENT_PAID")

	selector := workflow.NewSelector(ctx)
	hasPaid := false

	// 分支 A: 收到支付信号
	selector.AddReceive(paymentCh, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, &paymentSignal)
		hasPaid = true
	})

	// 分支 B: 定时器超时 (测试用30秒)
	selector.AddFuture(workflow.NewTimer(ctx, 30*time.Second), func(f workflow.Future) {
		logger.Info("支付超时定时器触发")
	})

	// 阻塞等待
	selector.Select(ctx)

	if !hasPaid {
		currentState = "超时未支付，正在回滚..."
		rollback(ctx, compensations)
		currentState = "已取消 (超时)"
		return &common.OrderStatus{Status: "CANCELLED", Message: "支付超时"}, nil
	}

	// ------------------------------------------------------------------
	// Step 4: 流程完成
	// ------------------------------------------------------------------
	currentState = "支付成功，准备发货"
	// 这里可以继续添加发货 Activity...
	workflow.Sleep(ctx, time.Second*2) // 模拟发货耗时

	currentState = "已完成"
	return &common.OrderStatus{Status: "COMPLETED", Message: "订单履约完成"}, nil
}

// 辅助函数：执行 Saga 补偿
func rollback(ctx workflow.Context, compensations []func(workflow.Context) error) {
	// 使用 DisconnectedContext 确保即使父 Context 取消也能执行
	dCtx, _ := workflow.NewDisconnectedContext(ctx)
	// 重新附加 Activity 选项
	dCtx = workflow.WithActivityOptions(dCtx, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 1,
	})

	// 倒序执行
	for i := len(compensations) - 1; i >= 0; i-- {
		_ = compensations[i](dCtx)
	}
}
