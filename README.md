# OmniFlow v2.5: 生产级分布式电商履约系统

![Go](https://img.shields.io/badge/Go-1%2E21+-00ADD8?style=flat&logo=go)
![Temporal](https://img.shields.io/badge/Temporal-Orchestration-blue?style=flat&logo=temporal)
![MySQL](https://img.shields.io/badge/MySQL-8%2E0-4479A1?style=flat&logo=mysql)
![GORM](https://img.shields.io/badge/GORM-ORM-red?style=flat)
![Gin](https://img.shields.io/badge/Gin-Web_Framework-00ADD8?style=flat&logo=go)

**OmniFlow** 是一个基于 **Temporal** 和 **Go** 构建的、具备高可靠性的分布式订单履约引擎。
从 v2.0 版本开始，项目引入了 **MySQL** 作为业务持久层，实现了基于数据库事务的**强一致性幂等去重**和**悲观锁并发控制**，从一个原型系统演进为具备生产级特性的架构案例。

## 🚀 核心特性 (Key Features)

* **💎 双数据库架构**: 采用架构分离设计。**Temporal (PostgreSQL)** 负责流程状态持久化，**业务层 (MySQL)** 负责资产数据存储，彻底解耦。
* **🛡️ 强一致性幂等 (Idempotency Framework)**: 自研基于 MySQL 唯一键 + 事务原子性的 `dedup` 中间件，完美解决分布式重试导致的“资产重复扣减”问题。
* **🔒 高并发防超卖**: 在 Inventory Activity 中集成 `SELECT ... FOR UPDATE` 悲观锁，确保在高并发秒杀场景下的库存数据准确性。
* **🔄 分布式事务 (Saga Pattern)**: 支付失败或风控拒绝时，自动触发补偿流程（Compensations），回滚已扣减的库存。
* **🧩 复杂流程编排**:
    * **Child Workflows**: 实现拆单逻辑，并行处理多仓库发货（Fan-out/Fan-in）。
    * **Human-in-the-Loop**: 大额订单自动挂起，等待人工通过 API 审核。
    * **Timer & Timeout**: 基于持久化定时器的订单超时自动取消机制。

## 🏗️ 系统架构 (Architecture)

```text
OmniFlow/
├── cmd/
│   ├── api-server/      # [入口] Gin HTTP 网关 (Port 8000)
│   └── worker/          # [入口] Temporal Worker (含 DB 初始化与自动迁移)
├── internal/
│   ├── app/             # [业务层]
│   │   ├── activities.go      # 包含 GORM 事务、锁机制的业务动作
│   │   ├── workflow.go        # 主流程编排 (Saga, Signal, Timer)
│   │   └── workflow_child.go  # 子流程 (并行发货)
│   ├── common/          # [模型层] 通用结构体
│   └── pkg/
│       └── dedup/       # [核心组件] 幂等性执行器中间件
├── docker-compose.yml   # 基础设施 (MySQL 8 + Temporal + PostgreSQL)
└── go.mod
```

## 🛠️ 快速开始 (Getting Started)

### 前置要求

* Go 1.21+
* Docker & Docker Compose

### 1. 启动基础设施

这将启动 MySQL (业务库) 和 Temporal Cluster (含 Postgres 系统库)。

```bash
docker-compose up -d

```

*注意：首次启动 MySQL 可能需要几十秒初始化，请耐心等待。*

### 2. 启动 Worker (消费者)

Worker 会自动连接 MySQL，创建 `products` 和 `idempotency_logs` 表，并初始化测试库存数据。

```bash
go run cmd/worker/main.go

```

### 3. 启动 API Server (生产者)

启动 Web 服务监听 **8000** 端口。

```bash
go run cmd/api-server/main.go

```

---

## 🧪 场景演示 (API Examples)

所有接口均位于 `http://localhost:8000/api/v1`。

### 场景 A: 正常下单与并发扣库

1. **创建订单** (此时 MySQL 库存会立即减少):
```bash
curl -X POST http://localhost:8000/api/v1/orders \
     -d '{"amount": 500, "items": ["iPhone15"]}'

```


*Response: `{"order_id": "ORD-170..."}*`
2. **模拟支付** (触发并行拆单发货):
```bash
curl -X POST http://localhost:8000/api/v1/orders/ORD-170.../pay

```


3. **查看状态**:
```bash
curl http://localhost:8000/api/v1/orders/ORD-170...

```


*Status: "已完成" (包含 "拆单发货" 逻辑)*

### 场景 B: 幂等性测试 (模拟故障重试)

Temporal 的机制决定了 Activity 可能会被重复执行（例如 Worker 崩溃后重启）。

* **机制**: `ReserveInventory` 使用了 `dedup.Execute` 包装器。
* **效果**: 即使代码被重复调用 10 次，`dedup` 会利用 MySQL 的 `Duplicate Entry` 错误拦截后续请求，**库存只会扣减 1 次**，保证资金安全。

### 场景 C: 大额订单风控 (Saga 回滚)

1. **下单 (> 10000元)**:
```bash
curl -X POST http://localhost:8000/api/v1/orders \
     -d '{"amount": 20000, "items": ["MacPro"]}'

```


*Status: "⚠️ 待风控审核"*
2. **管理员拒绝**:
```bash
curl -X POST http://localhost:8000/api/v1/orders/ORD-170.../audit \
     -d '{"action": "REJECT"}'

```


*Result: 触发 Saga 补偿，MySQL 中的库存会自动加回。*

---

## 💡 核心技术深度解析 (Technical Highlights)

### 1. 为什么使用 MySQL 做幂等性，而不是 Redis？

为了保证**金融级的数据一致性**。
在扣减库存场景中，如果使用 Redis 做去重锁，会面临“Redis 写入成功但 MySQL 写入失败”的双写不一致风险。
OmniFlow 采用 **Local Transaction Table** 模式，将“去重键的插入”与“库存扣减”放在同一个 MySQL 事务中提交。根据 ACID 特性，两者要么同时成功，要么同时回滚，彻底根除了数据不一致的可能性。

### 2. 悲观锁 (Pessimistic Locking)

在 `InventoryActivities` 中，我们使用了 GORM 的 Locking 子句：

```go
tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&product, "id = ?", itemID)

```

这对应 SQL 的 `SELECT ... FOR UPDATE`。在高并发场景下，这能防止多个请求同时读取到相同的库存数量（超卖风险），将并行操作串行化。

### 3. Saga 分布式事务

我们放弃了复杂的 2PC/XA 协议，采用了更适合微服务的 Saga 模式。

* **正向操作**: `ReserveInventory` (扣减)
* **补偿操作**: `ReleaseInventory` (回补)
Workflow 会追踪每一步的执行情况，一旦发生非预期错误（如支付超时），Temporal 会自动倒序执行补偿操作，实现数据的最终一致性。

