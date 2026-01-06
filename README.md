# OmniFlow v2.5: 生产级分布式电商履约系统

![Go](https://img.shields.io/badge/Go-1%2E21+-00ADD8?style=flat&logo=go)
![Temporal](https://img.shields.io/badge/Temporal-Orchestration-blue?style=flat&logo=temporal)
![MySQL](https://img.shields.io/badge/MySQL-8%2E0-4479A1?style=flat&logo=mysql)
![GORM](https://img.shields.io/badge/GORM-ORM-red?style=flat)
![Gin](https://img.shields.io/badge/Gin-Web_Framework-00ADD8?style=flat&logo=go)

**OmniFlow** 是一个基于 **Temporal** 和 **Go** 构建的、具备高可靠性的分布式订单履约引擎。
本项目不仅演示了 **Saga 分布式事务** 和 **MySQL 悲观锁** 的生产级实践，更包含了一套完整的 **工程化测试套件**，验证了从单元逻辑到长运行流程的正确性。

## 🚀 核心特性 (Key Features)

* **💎 双数据库架构**: **Temporal (PostgreSQL)** 负责流程状态持久化，**业务层 (MySQL)** 负责资产数据存储，彻底解耦。
* **🛡️ 强一致性幂等 (Idempotency Framework)**: 自研基于 MySQL 唯一键 + 事务原子性的 `dedup` 中间件，防止分布式重试导致的“资产重复扣减”。
* **🧪 全面的工程化测试**:
    * **Time Skipping**: 毫秒级验证“30分钟超时”逻辑，无需真实等待。
    * **In-Memory DB**: Activity 测试集成 SQLite 内存模式，验证 SQL 逻辑而不依赖外部环境。
    * **Mocking**: 完美解耦业务逻辑与外部依赖。
* **🔒 高并发防超卖**: 集成 `SELECT ... FOR UPDATE` 悲观锁，确保高并发下的库存准确性。
* **🔄 分布式事务 (Saga Pattern)**: 支付失败或风控拒绝时，自动触发补偿流程（Compensations）。
* **🧩 复杂流程编排**: 支持子流程拆单 (Fan-out)、人工审核 (Signal) 和超时取消 (Durable Timer)。

## 🏗️ 系统架构 (Architecture)

```text
OmniFlow/
├── cmd/
│   ├── api-server/      # [入口] Gin HTTP 网关 (Port 8000)
│   └── worker/          # [入口] Temporal Worker (含 DB 自动迁移)
├── internal/
│   ├── app/             # [业务层]
│   │   ├── activities.go      # 业务动作 (GORM + 锁)
│   │   ├── workflow.go        # 流程编排 (Saga + Signal)
│   │   ├── workflow_test.go   # [测试] 集成测试 (Mock + Time Skip)
│   │   └── activities_test.go # [测试] 单元测试 (SQLite)
│   ├── common/          # [模型层]
│   └── pkg/
│       └── dedup/       # [核心组件] 幂等性中间件
├── docker-compose.yml   # 基础设施 (MySQL 8 + Temporal)
└── go.mod

```

## 🛠️ 快速开始 (Getting Started)

### 1. 启动基础设施

```bash
docker-compose up -d
```

### 2. 运行测试 (强烈推荐) 🔥

在启动服务前，先验证系统逻辑的健壮性。

```bash
go test ./internal/app/... -v
```

你将看到测试套件在 **几百毫秒** 内模拟了包括“订单超时”、“风控拒绝”、“Activity 重试失败”在内的复杂场景。

### 3. 启动服务

```bash
# 终端 1: 启动 Worker (消费者)
go run cmd/worker/main.go

# 终端 2: 启动 API (生产者)
go run cmd/api-server/main.go
```

---

## 🧪 测试策略深度解析 (Testing Strategy)

本项目展示了如何对 Temporal 应用进行**分层测试**：

### 1. Workflow 集成测试 (`workflow_test.go`)

利用 `go.temporal.io/sdk/testsuite` 提供的内存环境。

* **模拟时间**: 当 Workflow 阻塞在 `NewTimer(30*time.Minute)` 时，测试环境会自动“快进”时间，瞬间触发超时逻辑。
* **模拟重试**: 验证 `RetryPolicy` 是否生效，以及 Activity 连续失败后的错误处理逻辑。
* **优雅失败**: 验证 Workflow 在发生业务错误（如风控拒绝）时，能否返回正确的结构化状态（`FAILED`），而不是抛出系统异常。

### 2. Activity 单元测试 (`activities_test.go`)

利用 `gorm` + `sqlite` (内存模式)。

* **隔离性**: 不需要连接真实的 MySQL，每次测试自动建表，跑完自动销毁。
* **幂等性验证**: 编写了 `TestReserveInventory_Idempotency`，连续调用两次扣库函数，断言库存只减少了一次。

---

## 📖 核心技术原理 (Under the Hood)

### 1. 为什么 Worker 挂了流程不断？(Event Sourcing)

OmniFlow 不存储当前状态，只存储**事件历史 (Event History)**。
当 Worker 重启时，它从 Server 拉取历史记录，**重放 (Replay)** 代码执行路径。

* 遇到已执行过的 `Activity`，SDK 直接返回历史结果（Mock），跳过真实执行。
* 遇到未执行的代码，才发起真正的调用。

### 2. 幂等性是如何实现的？

为了防止“网络超时但实际扣款成功”导致的重试重复扣款，我们使用了 **Local Transaction Table** 模式：

```go
tx.Transaction(func(tx *gorm.DB) error {
    // 1. 尝试插入去重键 (Key: "order_123_reserve")
    if err := tx.Create(&log); err != nil {
        return nil // 键已存在 -> 拦截重复请求，假装成功
    }
    // 2. 执行业务 (扣库存)
    return doBusinessLogic(tx)
})
```

利用数据库事务的 ACID 特性，保证“去重键插入”和“业务操作”要么同时成功，要么同时失败。

### 3. 悲观锁 (Pessimistic Locking)

在秒杀以外的常规高并发场景，我们使用 `SELECT ... FOR UPDATE` (GORM: `clause.Locking{Strength: "UPDATE"}`)。
这能从数据库层面将对同一商品的并发扣减串行化，彻底根除超卖风险。

---

## 📝 常用 API

| 动作 | 方法 | URL | 描述 |
| --- | --- | --- | --- |
| **下单** | POST | `/api/v1/orders` | 创建订单，触发工作流 |
| **查询** | GET | `/api/v1/orders/:id` | 查询当前状态 (Query) |
| **支付** | POST | `/api/v1/orders/:id/pay` | 发送支付信号 (Signal) |
| **审核** | POST | `/api/v1/orders/:id/audit` | 发送风控结果 (Signal) |

```
