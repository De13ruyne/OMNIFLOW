package main

import (
	"log"
	"omniflow/internal/app"
	"omniflow/internal/common"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	// 1. 创建 Temporal Client
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatalln("无法连接 Temporal Server", err)
	}
	defer c.Close()

	// 2. 创建 Worker
	// 注意：TaskQueue 必须和 Workflow 中指定的一致
	w := worker.New(c, common.TaskQueue, worker.Options{})

	// 3. 注册 Workflow 和 Activities
	// 只有注册了，Temporal Server 才知道这个 Worker 能干什么活
	w.RegisterWorkflow(app.OrderFulfillmentWorkflow)

	invActs := &app.InventoryActivities{}
	payActs := &app.PaymentActivities{}
	w.RegisterActivity(invActs.ReserveInventory)
	w.RegisterActivity(payActs.ProcessPayment)

	// 4. 启动监听
	log.Println("Worker 启动成功，正在监听任务队列...")
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Worker 运行失败", err)
	}
}
