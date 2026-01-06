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
		log.Fatalln("Temporal 连接失败:", err)
	}
	defer temporalClient.Close()

	r := gin.Default()
	v1 := r.Group("/api/v1")
	{
		v1.POST("/orders", createOrder)
		v1.GET("/orders/:id", getStatus)
		v1.POST("/orders/:id/pay", payOrder)
		v1.POST("/orders/:id/audit", auditOrder)
	}

	log.Println("API Server 监听 :8000")
	r.Run(":8000")
}

func createOrder(c *gin.Context) {
	var req struct {
		Amount int      `json:"amount"`
		Items  []string `json:"items"`
	}
	c.BindJSON(&req)
	orderID := fmt.Sprintf("ORD-%d", time.Now().Unix())
	order := common.Order{OrderID: orderID, Amount: req.Amount, Items: req.Items}

	_, err := temporalClient.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{
		ID: "WORKFLOW_" + orderID, TaskQueue: common.TaskQueue,
	}, app.OrderFulfillmentWorkflow, order)

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"order_id": orderID})
}

func getStatus(c *gin.Context) {
	val, _ := temporalClient.QueryWorkflow(context.Background(), "WORKFLOW_"+c.Param("id"), "", "get_order_status")
	var status string
	val.Get(&status)
	c.JSON(200, gin.H{"status": status})
}

func payOrder(c *gin.Context) {
	temporalClient.SignalWorkflow(context.Background(), "WORKFLOW_"+c.Param("id"), "", "SIGNAL_PAYMENT_PAID", nil)
	c.JSON(200, gin.H{"msg": "Signal Sent"})
}

func auditOrder(c *gin.Context) {
	var req struct {
		Action string `json:"action"`
	}
	c.BindJSON(&req)
	temporalClient.SignalWorkflow(context.Background(), "WORKFLOW_"+c.Param("id"), "", "SIGNAL_ADMIN_ACTION", req.Action)
	c.JSON(200, gin.H{"msg": "Audit Sent"})
}
