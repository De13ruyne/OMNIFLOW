package main

import (
	"context"
	"fmt"
	"log"
	"omniflow/internal/app"
	"omniflow/internal/common"
	"time"

	"github.com/gin-gonic/gin"
	"go.temporal.io/sdk/client"
)

var temporalClient client.Client

func main() {
	var err error
	temporalClient, err = client.Dial(client.Options{HostPort: "127.0.0.1:7233"})
	if err != nil {
		log.Fatalln("无法连接 Temporal Server", err)
	}
	defer temporalClient.Close()

	r := gin.Default()

	// 路由定义
	v1 := r.Group("/api/v1")
	{
		v1.POST("/orders", createOrder)          // 下单
		v1.GET("/orders/:id", getOrderStatus)    // 查询状态
		v1.POST("/orders/:id/pay", payOrder)     // 模拟支付
		v1.POST("/orders/:id/audit", auditOrder) // 管理员审核
	}

	log.Println("API Server 监听 :8000")
	r.Run(":8000")
}

// --- Handlers ---

func createOrder(c *gin.Context) {
	var req struct {
		Amount int      `json:"amount"`
		Items  []string `json:"items"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	orderID := fmt.Sprintf("ORD-%d", time.Now().Unix())
	order := common.Order{OrderID: orderID, Amount: req.Amount, Items: req.Items}

	opt := client.StartWorkflowOptions{
		ID:        "WORKFLOW_" + orderID,
		TaskQueue: common.TaskQueue,
	}

	_, err := temporalClient.ExecuteWorkflow(context.Background(), opt, app.OrderFulfillmentWorkflow, order)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"order_id": orderID, "msg": "Order Created"})
}

func getOrderStatus(c *gin.Context) {
	workflowID := "WORKFLOW_" + c.Param("id")
	val, err := temporalClient.QueryWorkflow(context.Background(), workflowID, "", "get_order_status")
	if err != nil {
		c.JSON(500, gin.H{"error": "Query failed", "details": err.Error()})
		return
	}
	var status string
	val.Get(&status)
	c.JSON(200, gin.H{"order_id": c.Param("id"), "status": status})
}

func payOrder(c *gin.Context) {
	workflowID := "WORKFLOW_" + c.Param("id")
	err := temporalClient.SignalWorkflow(context.Background(), workflowID, "", "SIGNAL_PAYMENT_PAID", "PAID")
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"msg": "Payment Signal Sent"})
}

func auditOrder(c *gin.Context) {
	workflowID := "WORKFLOW_" + c.Param("id")
	var req struct {
		Action string `json:"action"` // APPROVE or REJECT
	}
	c.BindJSON(&req)
	err := temporalClient.SignalWorkflow(context.Background(), workflowID, "", "SIGNAL_ADMIN_ACTION", req.Action)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"msg": "Audit Signal Sent"})
}
