<!-- # OmniFlow v2.5: 生产级分布式电商履约系统

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


 -->



# OmniFlow: 分布式电商履约与高并发秒杀引擎 (v2.5)

## 1. 项目概述 (Executive Summary)

**OmniFlow** 是一个基于 **Go** 语言和 **Temporal** 工作流引擎构建的企业级电商履约系统。该项目旨在解决传统单体架构在**高并发秒杀**和**长链路分布式事务**场景下的痛点。

通过引入 **"三层漏斗" (3-Layer Funnel)** 架构，OmniFlow 实现了流量的逐级清洗与削峰，在保证数据强一致性（ACID）的同时，单机环境实测达到了 **13,000+ QPS** 的吞吐能力。

* **核心特性**：分布式 Saga 事务、高并发秒杀、库存强一致性、全链路可观测性。
* **技术栈**：Golang, Temporal (Workflow), Redis (Lua Scripting), MySQL (GORM), Prometheus, Grafana.

---

## 2. 系统架构设计 (System Architecture)

OmniFlow 摒弃了传统的同步调用链，采用了**事件驱动**与**编排式 (Orchestration)** 相结合的架构。核心设计理念是 **"流量漏斗"** 模型。

### 2.1 三层漏斗模型 (The 3-Layer Funnel)

系统将请求处理分为三个层级，每一层都有明确的职责和过滤机制：

| 层级 | 组件 | 职责 | 核心技术 | QPS 承载级 |
| --- | --- | --- | --- | --- |
| **L1 拦截层** | **API Server + Redis** | 流量整形、库存预扣减、快速失败 | **Lua 原子脚本** | 10,000+ |
| **L2 编排层** | **Temporal Cluster** | 状态管理、流程编排、重试与超时 | **Event Sourcing** | 1,000+ |
| **L3 数据层** | **MySQL + Worker** | 资产落地、强一致性校验、持久化 | **悲观锁 (For Update)** | 100+ |

---

## 3. 核心业务流程详解 (Core Business Scenarios)

### 3.1 高并发秒杀场景 (High Concurrency Flash Sale)

**挑战**：在数万用户瞬间抢购少量库存（如 10 台 iPhone）时，防止数据库因连接耗尽而崩溃，并杜绝超卖。

**处理流程**：

1. **原子资格校验 (Redis Lua)**:
* API 接收请求，直接执行 Redis Lua 脚本。
* 脚本原子性地执行 `Check And Decr`。若库存不足，直接返回 `429 Too Many Requests`。
* **效果**：99.9% 的无效流量在这一层被拦截，无需触达数据库。


2. **异步削峰 (Async Hand-off)**:
* 获得资格的请求，API Server 仅负责向 Temporal 提交 Workflow 启动指令。
* 立即向前端返回 `200 OK` (排队中)，实现 HTTP 线程的快速释放。



### 3.2 分布式库存扣减 (Reliable Inventory Reservation)

**挑战**：在分布式环境下，如何保证库存扣减的准确性，且支持幂等（防重复扣减）。

**处理流程**：

1. **Workflow 调度**: Temporal Worker 领取任务，执行 `ReserveInventory` Activity。
2. **数据库悲观锁**:
```sql
START TRANSACTION;
-- 1. 幂等性检查 (利用唯一索引)
INSERT INTO idempotency_logs (key) VALUES ('order_123_reserve');
-- 2. 锁定行
SELECT stock FROM products WHERE id='iPhone15' FOR UPDATE;
-- 3. 扣减
UPDATE products SET stock = stock - 1 WHERE id='iPhone15';
COMMIT;

```


3. **结果**: 即使 Worker 在 Commit 后崩溃，Temporal 的重试机制配合数据库的幂等记录，保证了操作的 **Exactly-Once** 语义。

### 3.3 超时自动取消与 Saga 补偿 (Timeout & Saga Compensation)

**挑战**：用户锁定库存后可能放弃支付，系统需在 30 分钟后自动释放库存，不能依赖轮询数据库（性能差）。

**处理流程**：

1. **零资源挂起**: Workflow 调用 `selector.AddFuture(workflow.NewTimer(30*time.Minute))`。此时 Worker 卸载内存，仅仅在 Temporal DB 中保留一条 Event 记录。
2. **竞态路由 (Race Condition)**:
* **分支 A (支付成功)**: 收到 `Signal`，取消定时器，推进流程。
* **分支 B (超时)**: 定时器触发，执行 **Saga 补偿逻辑**。


3. **补偿执行**:
* 调用逆向 Activity `ReleaseInventory`。
* 将 MySQL 库存回滚，并标记订单为 `CANCELLED`。



---

## 4. 关键技术难点与解决方案 (Engineering Challenges)

### 4.1 解决“超卖”问题的多重防线

* **第一道防线 (Redis)**: 利用 Redis 单线程特性 + Lua 脚本原子性，粗粒度拦截流量。
* **第二道防线 (MySQL)**: 利用 InnoDB 引擎的 `SELECT ... FOR UPDATE` 行锁，确保并发下的最终数据准确性。

### 4.2 为什么选择 Temporal 而不是 Kafka/RabbitMQ？

* **状态可见性**: MQ 是“发后即忘”的，难以追踪订单当前处于“拆单中”还是“等待支付”。Temporal 原生提供状态查询。
* **复杂度治理**: 在 MQ 中实现“30分钟超时 + 补偿”需要引入死信队列和定时任务，逻辑分散。Temporal 通过代码逻辑（`Sleep`）即可实现，逻辑内聚且易于维护。

### 4.3 压测性能报告 (Performance Benchmark)

在单机 Docker 环境（4 Core, 8GB RAM）下，使用 `stress_runner` 进行 2000 并发测试：

* **配置**:
* Redis 连接池: 200
* HTTP Client Keep-Alive: Enabled
* `ulimit -n`: 10240


* **结果**:
* **QPS**: **12,995 req/sec**
* **库存准确性**: 预设 10 库存，成功 10 单，拦截 1990 单。**零超卖**。



---

## 5. 项目代码结构 (Project Structure)

遵循 Golang 标准工程布局 (Standard Go Project Layout)：

```text
OmniFlow/
├── cmd/
│   ├── api-server/      # [入口] HTTP API，集成 Redis 漏斗
│   └── worker/          # [后端] Temporal Worker，处理 MySQL 事务
├── internal/
│   ├── app/
│   │   ├── workflow.go  # [核心] Saga 编排与超时逻辑
│   │   └── activities.go# [原子] 数据库 CRUD 操作
│   ├── pkg/
│   │   ├── store/       # [组件] Redis 客户端与 Lua 脚本封装
│   │   └── dedup/       # [组件] 幂等性 SDK
│   └── common/          # [共享] 类型定义
└── docker-compose.yml   # 基础设施编排 (Redis, MySQL, Temporal, Grafana)

```

---

## 6. API 接口文档 (API Reference)

### 创建订单 (秒杀)

**POST** `/api/v1/orders`

**Request:**

```json
{
  "amount": 100,
  "items": ["iPhone15"]
}

```

**Response (Success):**

```json
{
  "message": "抢购成功，正在排队处理中...",
  "order_id": "ORDER-550e8400-e29b...",
  "run_id": "a3f29b..."
}

```

**Response (Throttled):**

```json
{
  "error": "手慢了，库存不足！"
}

```

*状态码: 429 Too Many Requests*

---

## 7. 未来演进规划 (Roadmap)

1. **通知中心**: 解耦通知渠道，支持邮件、短信、Webhook 插件化。
2. **财务对账**: 利用 Cron Workflow 实现 Redis 与 MySQL 库存的每日自动对账与红冲蓝补。
3. **微服务拆分**: 将 Order 与 Inventory 拆分为独立 Worker，独立扩容。