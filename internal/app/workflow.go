package app

import (
	"omniflow/internal/common"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func OrderFulfillmentWorkflow(ctx workflow.Context, order common.Order) (*common.OrderStatus, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 1,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	logger := workflow.GetLogger(ctx)

	// 状态查询支持
	currentState := "初始化"
	workflow.SetQueryHandler(ctx, "get_order_status", func() (string, error) {
		return currentState, nil
	})

	// var invActs *InventoryActivities
	invActs := &InventoryActivities{}
	var compensations []func(workflow.Context) error

	// === Step 1: 预占库存 ===
	currentState = "正在预占库存"
	if err := workflow.ExecuteActivity(ctx, invActs.ReserveInventory, order).Get(ctx, nil); err != nil {
		currentState = "库存失败"
		return &common.OrderStatus{Status: "FAILED", Message: err.Error()}, nil
	}

	// 注册补偿
	compensations = append(compensations, func(ctx workflow.Context) error {
		return workflow.ExecuteActivity(ctx, invActs.ReleaseInventory, order).Get(ctx, nil)
	})

	// === Step 2: 风控 (大额订单) ===
	if order.Amount > 10000 {
		currentState = "⚠️ 待风控审核"
		var action string
		workflow.GetSignalChannel(ctx, "SIGNAL_ADMIN_ACTION").Receive(ctx, &action)
		if action == "REJECT" {
			rollback(ctx, compensations)
			currentState = "已拒绝"
			return &common.OrderStatus{Status: "REJECTED"}, nil
		}
	}

	// === Step 3: 支付 (含超时) ===
	currentState = "待支付 (30s超时)"
	selector := workflow.NewSelector(ctx)
	hasPaid := false

	selector.AddReceive(workflow.GetSignalChannel(ctx, "SIGNAL_PAYMENT_PAID"), func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, nil)
		hasPaid = true
	})
	selector.AddFuture(workflow.NewTimer(ctx, 30*time.Second), func(f workflow.Future) {
		logger.Info("超时触发")
	})

	selector.Select(ctx)

	if !hasPaid {
		rollback(ctx, compensations)
		currentState = "已取消 (超时)"
		return &common.OrderStatus{Status: "CANCELLED"}, nil
	}

	// === Step 4: 拆单 (子流程) ===
	currentState = "拆单发货中"
	// 模拟拆成两个包裹
	pkgs := []common.Shipment{
		{ShipmentID: order.OrderID + "-A", OrderID: order.OrderID, Warehouse: "Shanghai"},
		{ShipmentID: order.OrderID + "-B", OrderID: order.OrderID, Warehouse: "Guangzhou"},
	}

	var futures []workflow.ChildWorkflowFuture
	for _, p := range pkgs {
		cwo := workflow.ChildWorkflowOptions{WorkflowID: "SHIP_" + p.ShipmentID}
		futures = append(futures, workflow.ExecuteChildWorkflow(workflow.WithChildOptions(ctx, cwo), ShippingChildWorkflow, p))
	}

	for _, f := range futures {
		if err := f.Get(ctx, nil); err != nil {
			return nil, err
		}
	}

	currentState = "已完成"
	return &common.OrderStatus{Status: "COMPLETED"}, nil
}

func rollback(ctx workflow.Context, compensations []func(workflow.Context) error) {
	dCtx, _ := workflow.NewDisconnectedContext(ctx)
	dCtx = workflow.WithActivityOptions(dCtx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	for i := len(compensations) - 1; i >= 0; i-- {
		_ = compensations[i](dCtx)
	}
}
