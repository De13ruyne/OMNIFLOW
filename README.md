# OmniFlow: 分布式电商履约系统 (Distributed E-commerce Fulfillment System)

![Go](https://img.shields.io/badge/Go-1%2E21%2B-00ADD8?style=flat&logo=go)
![Temporal](https://img.shields.io/badge/Temporal-Orchestration-blue?style=flat&logo=temporal)
![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=flat&logo=docker)

**OmniFlow** 是一个基于 **Golang** 和 **Temporal** 构建的强健壮性分布式订单履约引擎。它演示了如何在微服务架构中处理复杂的长运行业务流程、分布式事务（Saga 模式）以及人工介入（Human-in-the-Loop）场景。

## 🚀 核心功能 (Features)

* **🛡️ 分布式事务 (Saga 模式):** 确保跨服务的数据一致性。如果支付失败，已预占的库存会自动回滚（执行补偿操作）。
* **⏱️ 超时与自动取消:** 如果在指定时间窗口内未收到支付，订单将自动取消并释放资源（通过持久化计时器 Timer 实现）。
* **👮 人工介入 (Human-in-the-Loop):** 大额订单（> $10,000）会自动暂停流程并触发风控检查，无限期等待管理员通过 API 进行人工审批。
* **🔍 实时状态可视化:** 无需轮询数据库，即可通过 Query 接口即时查询任意订单的精确内部状态（例如“待支付”、“发货中”、“待审核”）。
* **⚡ 异步信号驱动:** 使用 Temporal Signals 处理外部异步事件，如“支付成功确认”和“管理员审核指令”。

## 🏗️ 系统架构 (Architecture)

本系统遵循整洁架构（Clean Architecture），职责分离清晰：

```text
OmniFlow/
├── cmd/
│   ├── api-server/    # REST API 网关 (Gin) - 负责接收请求并触发 Workflow
│   └── worker/        # Temporal Worker - 负责执行核心业务逻辑 (Workflows & Activities)
├── internal/
│   ├── app/           # Workflow 和 Activity 的具体实现逻辑
│   └── common/        # 共享类型定义与常量
├── docker-compose.yml # 基础设施：Temporal Server 和 PostgreSQL
└── go.mod

```

## 🛠️ 快速开始 (Getting Started)

### 前置要求

* Go 1.21+
* Docker & Docker Compose

### 1. 启动基础设施

启动 Temporal Server 和 PostgreSQL 数据库：

```bash
docker-compose up -d

```

*启动后，访问 Temporal Web UI 控制台：http://localhost:8080*

### 2. 启动 Worker (消费者)

Worker 负责轮询任务队列并执行具体的业务逻辑。

```bash
go run cmd/worker/main.go

```

### 3. 启动 API Server (生产者)

API Server 处理 HTTP 请求并与 Temporal 集群通信。

```bash
go run cmd/api-server/main.go

```

---

## 🧪 使用场景演练 (API Examples)

### 场景 A: 标准流程 (Happy Path)

1. **创建订单**:
```bash
curl -X POST http://localhost:8000/api/v1/orders \
     -d '{"amount": 500, "items": ["iPhone 15"]}'

```


*响应: `{"order_id": "ORD-170..."}*`
2. **查询状态**:
```bash
curl http://localhost:8000/api/v1/orders/ORD-170...

```


*当前状态: "待支付 (超时倒计时: 30s)"*
3. **模拟支付**:
```bash
curl -X POST http://localhost:8000/api/v1/orders/ORD-170.../pay

```


*状态变更为: "已完成"*

### 场景 B: 超时与补偿 (Timeout & Compensation)

1. 创建订单，但 **不进行支付**。
2. 等待 30 秒（模拟超时）。
3. 再次查询状态: *状态: "已取消 (超时)"* (此时后台已自动执行库存释放操作)。

### 场景 C: 大额订单人工审核 (Human Review)

1. **创建大额订单 (> $10,000)**:
```bash
curl -X POST http://localhost:8000/api/v1/orders \
     -d '{"amount": 20000, "items": ["Mac Pro"]}'

```


2. **查询状态**:
*当前状态: "⚠️ 待风控审核 (大额订单)"* (Workflow 已自动暂停)。
3. **管理员审核 (通过或拒绝)**:
```bash
curl -X POST http://localhost:8000/api/v1/orders/ORD-170.../audit \
     -d '{"action": "APPROVE"}' 
     # 或者使用 "REJECT" 触发回滚

```



## 📚 技术栈 (Tech Stack)

* **开发语言**: Golang
* **流程编排**: Temporal.io
* **Web 框架**: Gin
* **数据库**: PostgreSQL (via Temporal)
