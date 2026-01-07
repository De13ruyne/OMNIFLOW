package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"

	"omniflow/internal/app"
	"omniflow/internal/common"
	"omniflow/internal/pkg/store" // 🔥 引入新包
)

func main() {
	// 1. 初始化 Redis 连接
	// 注意：go run 本地运行时，连接 localhost:6379
	redisStore := store.NewRedisStore("127.0.0.1:6379")

	// 2. [模拟] 库存预热 (Warm-up)
	// 启动时强制把 iPhone15 库存设为 10，方便你测试
	ctx := context.Background()
	if err := redisStore.PreheatStock(ctx, "iPhone15", 10); err != nil {
		log.Printf("⚠️ 库存预热失败: %v", err)
	} else {
		log.Println("🔥 Redis 库存预热完成: iPhone15 = 10")
	}

	// 3. 初始化 Temporal Client
	c, err := client.Dial(client.Options{
		HostPort: "127.0.0.1:7233",
	})
	if err != nil {
		log.Fatalln("无法连接 Temporal Server", err)
	}
	defer c.Close()

	// 4. 启动 Gin Server
	r := gin.Default()

	// 注入依赖
	r.POST("/api/v1/orders", createOrderHandler(c, redisStore))

	log.Println("🚀 API Server 监听 :8000")
	r.Run(":8000")
}

func createOrderHandler(temporalClient client.Client, redisStore *store.RedisStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Amount int      `json:"amount"`
			Items  []string `json:"items"`
		}

		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
			return
		}

		if len(req.Items) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "商品列表为空"})
			return
		}

		// === 🔥 核心：Redis 流量漏斗 ===
		// 简化逻辑：我们只对第一个商品做秒杀判定
		targetProduct := req.Items[0]

		// 1. 尝试在 Redis 原子扣减
		result, err := redisStore.DeductStock(c.Request.Context(), targetProduct, 1)
		if err != nil {
			log.Printf("Redis 错误: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "系统繁忙"})
			return
		}

		// 2. 判断结果
		if result == 0 {
			// 库存不足 -> 拦截！不请求 Temporal，不查 MySQL
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "手慢了，库存不足！"})
			return
		} else if result == -1 {
			// 没预热 -> 拒绝或者是普通商品
			c.JSON(http.StatusBadRequest, gin.H{"error": "该商品未开放秒杀"})
			return
		}

		// result == 1 -> 抢到了！放行进入后端逻辑

		// === 🌊 放行：进入 Temporal 处理 ===
		workflowID := "ORDER-" + uuid.New().String()
		options := client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: common.TaskQueue,
		}

		order := common.Order{
			OrderID: workflowID,
			Amount:  req.Amount,
			Items:   req.Items,
		}

		// 异步启动 Workflow
		we, err := temporalClient.ExecuteWorkflow(c.Request.Context(), options, app.OrderFulfillmentWorkflow, order)
		if err != nil {
			log.Printf("Workflow 启动失败: %v", err)

			// ⚠️ 补偿机制：Temporal 挂了，把 Redis 库存还回去
			_ = redisStore.RollbackStock(context.Background(), targetProduct, 1)

			c.JSON(http.StatusInternalServerError, gin.H{"error": "订单创建失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":  "抢购成功，正在处理中",
			"order_id": order.OrderID,
			"run_id":   we.GetRunID(),
		})
	}
}
