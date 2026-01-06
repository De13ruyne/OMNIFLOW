package main

import (
	"log"
	"omniflow/internal/app"
	"omniflow/internal/common"
	"omniflow/internal/pkg/dedup"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// 1. 连接 MySQL
	dsn := "root:root@tcp(127.0.0.1:3306)/omniflow?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalln("MySQL 连接失败:", err)
	}

	// 2. 自动建表与初始化数据
	db.AutoMigrate(&app.Product{})
	dedup.AutoMigrate(db) // 建去重表
	initData(db)

	// 3. 连接 Temporal
	c, err := client.Dial(client.Options{HostPort: "127.0.0.1:7233"})
	if err != nil {
		log.Fatalln("Temporal 连接失败:", err)
	}
	defer c.Close()

	// 4. 启动 Worker
	w := worker.New(c, common.TaskQueue, worker.Options{})

	w.RegisterWorkflow(app.OrderFulfillmentWorkflow)
	w.RegisterWorkflow(app.ShippingChildWorkflow)

	// 注入 DB
	w.RegisterActivity(&app.InventoryActivities{DB: db})
	w.RegisterActivity(&app.ShippingActivities{})

	log.Println("Worker 已启动...")
	w.Run(worker.InterruptCh())
}

func initData(db *gorm.DB) {
	var count int64
	db.Model(&app.Product{}).Count(&count)
	if count == 0 {
		db.Create(&app.Product{ID: "iPhone15", Name: "iPhone 15", Stock: 10, Price: 8000})
		db.Create(&app.Product{ID: "MacPro", Name: "MacBook Pro", Stock: 5, Price: 20000})
	}
}
