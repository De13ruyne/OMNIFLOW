package main

import (
	"log"
	"net/http"
	"omniflow/internal/app"
	"omniflow/internal/common"
	"omniflow/internal/pkg/dedup"
	"time"

	// Prometheus 官方库
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	// Uber Tally 库
	"github.com/uber-go/tally/v4"
	tallyprom "github.com/uber-go/tally/v4/prometheus"

	// Temporal 的 Tally 适配层
	sdktally "go.temporal.io/sdk/contrib/tally"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// 1. 启动 Prometheus HTTP Handler
	// -----------------------------------------------------
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("📊 Metrics 监听端口 :9091")
		if err := http.ListenAndServe(":9091", nil); err != nil {
			log.Fatalln("Metrics 服务启动失败:", err)
		}
	}()

	// 2. 初始化 Uber Tally + Prometheus Reporter (🔥 修正部分)
	// -----------------------------------------------------
	// 直接使用 NewReporter 和 Options，而不是 Configuration
	reporter := tallyprom.NewReporter(tallyprom.Options{
		// 这里指定 Prometheus 的 DefaultRegisterer，
		// 这样 Tally 产生的数据就会注册到我们在第 1 步里 promhttp.Handler() 使用的同一个注册表中
		Registerer: prometheus.DefaultRegisterer,
		OnRegisterError: func(err error) {
			log.Println("Tally Prometheus 注册错误:", err)
		},
	})

	// C. 创建 Tally Root Scope
	// SanitizeOptions 会把 metrics 名字里的非法字符（如点号）变成下划线，符合 Prometheus 规范
	scope, _ := tally.NewRootScope(tally.ScopeOptions{
		Tags:            map[string]string{"service": "omniflow-worker"},
		CachedReporter:  reporter,
		Separator:       tallyprom.DefaultSeparator,
		SanitizeOptions: &tallyprom.DefaultSanitizerOpts,
	}, 1*time.Second)

	// -----------------------------------------------------

	// 3. 连接 MySQL
	dsn := "root:root@tcp(127.0.0.1:3306)/omniflow?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalln("MySQL 连接失败:", err)
	}

	db.AutoMigrate(&app.Product{})
	dedup.AutoMigrate(db)
	initData(db)

	// 4. 连接 Temporal (注入适配后的 MetricsHandler)
	// -----------------------------------------------------
	c, err := client.Dial(client.Options{
		HostPort: "127.0.0.1:7233",
		// 使用 contrib/tally 包将 Tally Scope 转换为 Temporal Handler
		MetricsHandler: sdktally.NewMetricsHandler(scope),
	})
	if err != nil {
		log.Fatalln("Temporal 连接失败:", err)
	}
	defer c.Close()

	// 5. 启动 Worker
	w := worker.New(c, common.TaskQueue, worker.Options{})
	w.RegisterWorkflow(app.OrderFulfillmentWorkflow)
	w.RegisterWorkflow(app.ShippingChildWorkflow)
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
