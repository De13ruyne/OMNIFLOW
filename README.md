# OmniFlow: Distributed E-commerce Fulfillment System

![Go](https://img.shields.io/badge/Go-1%2E21%2B-00ADD8?style=flat&logo=go)
![Temporal](https://img.shields.io/badge/Temporal-Orchestration-blue?style=flat&logo=temporal)
![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=flat&logo=docker)

**OmniFlow** is a robust, distributed order fulfillment engine built with **Golang** and **Temporal**. It demonstrates how to handle complex long-running business processes, distributed transactions (Saga), and human-in-the-loop interactions in a microservices architecture.

## 🚀 Features

* **🛡️ Distributed Transactions (Saga Pattern):** Ensures data consistency across services. If payment fails, inventory is automatically rolled back (compensated).
* **⏱️ Timeout & Cancellations:** Orders are automatically cancelled if payment is not received within a specified window (implemented via durable Timers).
* **👮 Human-in-the-Loop:** High-value orders (> $10,000) automatically pause and trigger a fraud check, waiting for manual admin approval via API.
* **🔍 Real-time Visibility:** Query the exact state of any order (e.g., "Waiting for Payment", "Shipping") instantly without database polling.
* **⚡ Asynchronous Signaling:** Uses Temporal Signals to handle external events like Payment Confirmation and Admin Audit.

## 🏗️ Architecture

The system follows a clean architecture with clear separation of concerns:

```text
OmniFlow/
├── cmd/
│   ├── api-server/    # REST API Gateway (Gin) - Triggers workflows
│   └── worker/        # Temporal Worker - Executes business logic
├── internal/
│   ├── app/           # Workflow & Activity Implementations
│   └── common/        # Shared Types & Constants
├── docker-compose.yml # Temporal Server & PostgreSQL Infrastructure
└── go.mod
```

## 🛠️ Getting Started

### Prerequisites
* Go 1.21+
* Docker & Docker Compose

### 1. Start Infrastructure
Start the Temporal Server and PostgreSQL database:
```bash
docker-compose up -d
```
*Access Temporal Web UI at: http://localhost:8080*

### 2. Start the Worker
The worker executes the workflows and activities.
```bash
go run cmd/worker/main.go
```

### 3. Start the API Server
The API server handles HTTP requests and communicates with the Temporal cluster.
```bash
go run cmd/api-server/main.go
```

---

## 🧪 Usage Scenarios (API Examples)

### Scenario A: Happy Path (Standard Order)
1.  **Create Order**:
    ```bash
    curl -X POST http://localhost:8000/api/v1/orders \
         -d '{"amount": 500, "items": ["iPhone 15"]}'
    ```
    *Response: `{"order_id": "ORD-170..."}`*

2.  **Check Status**:
    ```bash
    curl http://localhost:8000/api/v1/orders/ORD-170...
    ```
    *Status: "待支付 (超时倒计时: 30s)"*

3.  **Simulate Payment**:
    ```bash
    curl -X POST http://localhost:8000/api/v1/orders/ORD-170.../pay
    ```
    *Status changes to: "已完成"*

### Scenario B: Timeout & Compensation
1.  Create an order but **do not pay**.
2.  Wait for 30 seconds.
3.  Check status: *Status: "已取消 (超时)"* (Inventory is released automatically).

### Scenario C: High-Value Order (Human Review)
1.  **Create High-Value Order (> $10,000)**:
    ```bash
    curl -X POST http://localhost:8000/api/v1/orders \
         -d '{"amount": 20000, "items": ["Mac Pro"]}'
    ```

2.  **Check Status**:
    *Status: "⚠️ 待风控审核 (大额订单)"* (Workflow is paused).

3.  **Admin Approve/Reject**:
    ```bash
    curl -X POST http://localhost:8000/api/v1/orders/ORD-170.../audit \
         -d '{"action": "APPROVE"}' 
         # Or use "REJECT" to trigger rollback
    ```

## 📚 Tech Stack
* **Language**: Golang
* **Orchestration**: Temporal.io
* **Web Framework**: Gin
* **Database**: PostgreSQL (via Temporal)
