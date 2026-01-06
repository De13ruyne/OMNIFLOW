package common

const TaskQueue = "OMNIFLOW_TASK_QUEUE"

type Order struct {
	OrderID    string
	Amount     int
	Items      []string
	CustomerID string
}

type Shipment struct {
	ShipmentID string
	OrderID    string
	Warehouse  string
	Items      []string
}

type OrderStatus struct {
	OrderID string
	Status  string
	Message string
}
