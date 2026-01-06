package main

import (
	"log"
	"omniflow/internal/app"
	"omniflow/internal/common"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	// 1. 连接 Server
	c, err := client.Dial(client.Options{HostPort: "127.0.0.1:7233"})
	if err != nil {
		log.Fatalln("无法连接 Temporal Server", err)
	}
	defer c.Close()

	// 2. 注册 Worker
	w := worker.New(c, common.TaskQueue, worker.Options{})

	w.RegisterWorkflow(app.OrderFulfillmentWorkflow)
	w.RegisterActivity(&app.InventoryActivities{})

	// 3. 启动
	log.Println("Worker 已启动，等待任务...")
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalln("Worker 运行失败", err)
	}
}
