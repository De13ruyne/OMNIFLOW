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