package app

import (
	"errors"
	"omniflow/internal/common"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

func TestOrderFulfillmentWorkflow_Timeout(t *testing.T) {
	s := testsuite.WorkflowTestSuite{}
	env := s.NewTestWorkflowEnvironment()

	invActs := &InventoryActivities{}

	// 🔥 修正点 1: 传入两个 mock.Anything
	// 第一个匹配 context，第二个匹配 order
	env.OnActivity(invActs.ReserveInventory, mock.Anything, mock.Anything).Return(nil).Once()

	// 同理，ReleaseInventory 也要两个
	env.OnActivity(invActs.ReleaseInventory, mock.Anything, mock.Anything).Return(nil).Once()

	order := common.Order{
		OrderID: "TEST_ORDER_TIMEOUT",
		Amount:  100,
		Items:   []string{"iPhone15"},
	}

	env.ExecuteWorkflow(OrderFulfillmentWorkflow, order)

	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	var result common.OrderStatus
	env.GetWorkflowResult(&result)
	assert.Equal(t, "CANCELLED", result.Status)

	env.AssertExpectations(t)
}

func TestOrderFulfillmentWorkflow_Success(t *testing.T) {
	s := testsuite.WorkflowTestSuite{}
	env := s.NewTestWorkflowEnvironment()

	invActs := &InventoryActivities{}

	// 1. Activity 只会调用一次
	env.OnActivity(invActs.ReserveInventory, mock.Anything, mock.Anything).Return(nil).Once()

	// 🔥 修复点：拆单逻辑会启动 2 个子流程，所以这里要改为 .Times(2)
	env.OnWorkflow(ShippingChildWorkflow, mock.Anything, mock.Anything).Return("SF-123", nil).Times(2)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("SIGNAL_PAYMENT_PAID", "PAID_TEST")
	}, time.Second*1)

	order := common.Order{OrderID: "TEST_ORDER_SUCCESS", Amount: 100}
	env.ExecuteWorkflow(OrderFulfillmentWorkflow, order)

	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	var result common.OrderStatus
	env.GetWorkflowResult(&result)
	assert.Equal(t, "COMPLETED", result.Status)

	env.AssertExpectations(t)
}

func TestOrderFulfillmentWorkflow_AdminReject(t *testing.T) {
	s := testsuite.WorkflowTestSuite{}
	env := s.NewTestWorkflowEnvironment()
	invActs := &InventoryActivities{}

	// Mock: 预占成功，回滚成功
	env.OnActivity(invActs.ReserveInventory, mock.Anything, mock.Anything).Return(nil).Once()
	env.OnActivity(invActs.ReleaseInventory, mock.Anything, mock.Anything).Return(nil).Once()

	// 1. 模拟管理员拒绝信号
	// 假设风控审核需要 1秒
	env.RegisterDelayedCallback(func() {
		// 验证中间状态：此时应该是 "待风控审核"
		val, _ := env.QueryWorkflow("get_order_status")
		var status string
		val.Get(&status)
		assert.Contains(t, status, "待风控审核")

		// 发送拒绝信号
		env.SignalWorkflow("SIGNAL_ADMIN_ACTION", "REJECT")
	}, time.Second*1)

	// 2. 构造大额订单 (> 10000)
	order := common.Order{OrderID: "BIG_ORDER", Amount: 20000}
	env.ExecuteWorkflow(OrderFulfillmentWorkflow, order)

	// 3. 验证结果
	assert.True(t, env.IsWorkflowCompleted())

	var result common.OrderStatus
	env.GetWorkflowResult(&result)

	// 状态应该是 REJECTED，且触发了库存回滚
	assert.Equal(t, "REJECTED", result.Status)
	env.AssertExpectations(t)
}
func TestOrderFulfillmentWorkflow_ActivityFail(t *testing.T) {
	s := testsuite.WorkflowTestSuite{}
	env := s.NewTestWorkflowEnvironment()
	invActs := &InventoryActivities{}

	mockErr := errors.New("数据库连接断开")

	// Mock 失败，允许重试多次
	env.OnActivity(invActs.ReserveInventory, mock.Anything, mock.Anything).Return(mockErr)

	order := common.Order{OrderID: "FAIL_ORDER", Amount: 100}
	env.ExecuteWorkflow(OrderFulfillmentWorkflow, order)

	assert.True(t, env.IsWorkflowCompleted())

	// 🔥 修复点 1: 因为我们把 error 吞掉了，所以 Workflow 应该是“成功完成”的
	assert.NoError(t, env.GetWorkflowError())

	var result common.OrderStatus
	env.GetWorkflowResult(&result)

	// 🔥 修复点 2: 现在可以成功拿到 result 了
	assert.Equal(t, "FAILED", result.Status)
	assert.Contains(t, result.Message, "数据库连接断开")

	// 验证 Mock 是否生效
	env.AssertExpectations(t)
}
