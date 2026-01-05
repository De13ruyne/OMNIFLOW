package main

import (
	"context"
	"fmt"
	"log"
	"omniflow/internal/app"
	"omniflow/internal/common"
	"time"

	"go.temporal.io/sdk/client"
)

func main() {
	// 1. 连接 Client
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatalln("无法连接 Temporal Server", err)
	}
	defer c.Close()

	// 2. 准备订单数据
	order := common.Order{
		OrderID:    fmt.Sprintf("ORD-%d", time.Now().Unix()),
		Amount:     888,
		Items:      []string{"iPhone 15", "Case"},
		CustomerID: "USER_001",
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "WORKFLOW_" + order.OrderID, // 业务ID，防止重复下单
		TaskQueue: common.TaskQueue,
	}

	// 3. 启动 Workflow
	we, err := c.ExecuteWorkflow(context.Background(), workflowOptions, app.OrderFulfillmentWorkflow, order)
	if err != nil {
		log.Fatalln("无法启动 Workflow", err)
	}

	log.Printf("Workflow 已启动! RunID: %s, OrderID: %s\n", we.GetRunID(), order.OrderID)
	log.Println("请去 Temporal Web UI (http://localhost:8080) 查看流程状态。")

	// 注意：因为 Workflow 中有等待信号的步骤，所以这里不能立刻 get result
	// 我们模拟过了5秒后，仓库发货了（发送信号）

	time.Sleep(time.Second * 5)
	log.Println(">>> 模拟：仓库已打包，正在发送发货信号...")

	err = c.SignalWorkflow(context.Background(), workflowOptions.ID, we.GetRunID(), "SIGNAL_SHIPPING_INFO", "SF123456789")
	if err != nil {
		log.Fatalln("发送信号失败", err)
	}

	log.Println(">>> 信号已发送，Workflow 应该会继续执行并完成。")
}
