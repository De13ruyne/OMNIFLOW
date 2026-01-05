package common

// TaskQueue 定义任务队列名称
const TaskQueue = "OMNIFLOW_TASK_QUEUE"

// Order details
type Order struct {
	OrderID    string
	Amount     int
	Items      []string
	CustomerID string
}

// OrderStatus 用于 Workflow 返回结果
type OrderStatus struct {
	OrderID string
	Status  string
	Message string
}
