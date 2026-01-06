package common

const TaskQueue = "OMNIFLOW_TASK_QUEUE"

type Order struct {
	OrderID    string
	Amount     int
	Items      []string
	CustomerID string
}

type OrderStatus struct {
	OrderID string
	Status  string // COMPLETED, FAILED, CANCELLED, REJECTED
	Message string
}
