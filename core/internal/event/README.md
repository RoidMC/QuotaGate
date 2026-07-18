# event 事件库

`event` 封装了 QuotaGate 内部的事件发布/订阅能力，并提供可选的 **Transactional Outbox（事务外发箱）** 支持，保证业务操作与 Webhook 投递之间的最终一致性。

---

## 目录

1. [设计目标](#设计目标)
2. [架构](#架构)
3. [核心 API](#核心-api)
4. [快速开始](#快速开始)
   - [4.1 普通事件通知](#41-普通事件通知)
   - [4.2 事务化 Outbox 发布](#42-事务化-outbox-发布)
5. [使用建议](#使用建议)
6. [注意事项](#注意事项)

---

## 设计目标

- **调用方无感知底层存储**：无论底层是内存、`Redis Cluster` 还是未来替换为 `RabbitMQ`，发布方都使用同一套 API。
- **可选的事务一致性**：业务代码可以在数据库事务内发布事件，事件会被写入 `webhook_outbox` 表，随事务一起提交或回滚；事务外发布则走普通事件总线。
- **分层解耦**：`event` 包只定义 `OutboxWriter` 接口，不依赖具体的 `repository` 实现；数据层实现该接口，保持基础设施层的纯净。
- **业务解耦**：事件发布者不需要知道 Webhook 的存在；Webhook 消费由后台 `WebhookWorker` 负责。

---

## 架构

```text
┌─────────────────────────────────────────────────────────────────┐
│                         业务 Service                             │
│  db.Transaction(func(tx *gorm.DB) error {                       │
│      ctx = tx.WithTx(ctx, tx)                                │
│      return bus.PublishEvent(ctx, evt)                          │
│  })                                                             │
└───────────────────────────┬─────────────────────────────────────┘
                            │
            ┌───────────────┴───────────────┐
            │                               │
            ▼                               ▼
┌───────────────────────┐       ┌───────────────────────┐
│  ctx 包含 GORM 事务    │       │  ctx 不含事务          │
│  → 写入 webhook_outbox │       │  → 转发给 EventBus     │
│    （与业务同一事务）   │       │    （即时通知订阅者）   │
└───────────┬───────────┘       └───────────┬───────────┘
            │                               │
            ▼                               ▼
┌───────────────────────┐       ┌───────────────────────┐
│   WebhookWorker       │       │   其他内存/Redis 订阅者 │
│  轮询 + FOR UPDATE    │       │  （非 Webhook 用途）    │
│  SKIP LOCKED          │       │                       │
└───────────┬───────────┘       └───────────────────────┘
            │
            ▼
┌───────────────────────┐
│   HTTP Dispatch       │
│   重试 + DeliveryLog  │
└───────────────────────┘
```

### 关键组件

| 组件 | 文件 | 职责 |
|---|---|---|
| `EventBus` | `bus.go` | 基于 `kexswiftbus` 的事件发布/订阅，支持内存和 Redis 两种后端。 |
| `TransactionalBus` | `publisher.go` | 包装 `EventBus`，根据 `context` 中是否携带事务决定写 outbox 还是走总线。 |
| `WithTx` / `TxFromContext` | `tx.go` | 在 `context` 中绑定/提取 GORM 事务。 |
| `WebhookWorker` | `../worker/webhook.go` | 消费 `webhook_outbox`，执行 HTTP 投递。 |
| `OutboxWriter` | `publisher.go` | 事务 outbox 写入抽象接口，由数据层（如 `WebhookRepository`）实现。 |
| `WebhookRepository` | `../repository/webhook.go` | 实现了 `event.OutboxWriter`，在指定 `*gorm.DB`（或事务）上写入 `webhook_outbox`。 |

---

## 核心 API

### `EventBus`

```go
bus := event.NewBus()                       // 内存后端
bus, err := event.NewRedisBus(redisClient)  // Redis 后端

cancel, err := bus.SubscribeEvent("user.register", func(evt event.Event) {
    // 处理事件
})
defer cancel()

bus.PublishEvent(evt)        // 异步
err := bus.PublishEventSync(evt) // 同步
```

### `OutboxWriter`

```go
type OutboxWriter interface {
    CreateOutboxEntries(db *gorm.DB, eventType EventType, eventID, tenantID, payload string) error
}
```

`OutboxWriter` 是事务 outbox 的写入抽象，定义在 `event` 包内。数据层（如 `WebhookRepository`）实现该接口，从而避免 `event` 包反向依赖具体的 repository。

### `TransactionalBus`

```go
bus := event.NewTransactionalBus(eventBus, webhookRepo)

// 事务内发布：写 outbox
err := db.Transaction(func(tx *gorm.DB) error {
    ctx = tx.WithTx(ctx, tx)
    return bus.PublishEvent(ctx, evt)
})

// 事务外发布：走 EventBus
err := bus.PublishEvent(context.Background(), evt)
```

### `event.Event`

```go
type Event struct {
    ID        string      // 事件唯一 ID
    Type      EventType   // 事件类型，如 user.register
    Source    string      // 事件源服务/模块
    Subject   string      // 作用对象，常用 tenant_id
    Data      interface{} // 事件负载
    Timestamp time.Time   // 发生时间
}
```

---

## 快速开始

### 4.1 普通事件通知

适用于：非 Webhook 的即时通知、进程内解耦。

```go
package main

import (
    "context"
    "fmt"

    "github.com/roidmc/quotagate/internal/event"
)

func main() {
    bus := event.NewBus()

    cancel, _ := bus.SubscribeEvent("user.register", func(evt event.Event) {
        fmt.Printf("received: %+v\n", evt)
    })
    defer cancel()

    bus.PublishEvent(event.Event{
        ID:      "evt-1",
        Type:    "user.register",
        Subject: "tenant-1",
        Data:    map[string]string{"email": "a@example.com"},
    })
}
```

### 4.2 事务化 Outbox 发布

适用于：Webhook 投递，要求事件与业务操作原子一致。

```go
package main

import (
    "context"

    "github.com/roidmc/quotagate/internal/event"
    "github.com/roidmc/quotagate/internal/repository"
    "github.com/roidmc/quotagate/internal/util/tx"
    "gorm.io/gorm"
)

func handleRegister(
    ctx context.Context,
    db *gorm.DB,
    bus *event.TransactionalBus,
    userRepo UserRepository,
) error {
    return db.Transaction(func(tx *gorm.DB) error {
        // 1. 业务操作
        if err := userRepo.Create(tx, &User{...}); err != nil {
            return err
        }

        // 2. 在同一事务内发布事件 → 写入 webhook_outbox
        ctx = tx.WithTx(ctx, tx)
        evt := event.Event{
            ID:      "evt-xxx",
            Type:    "user.register",
            Subject: "tenant-1",
            Data:    map[string]string{"user_id": "u-1"},
        }
        return bus.PublishEvent(ctx, evt)
    })
    // 事务提交：outbox 和业务数据一起落盘
    // 事务回滚：outbox 不会写入
}
```

**初始化 `TransactionalBus`（通常在 boot/wire 中完成）：**

```go
webhookRepo := repository.NewWebhookRepository(db) // 实现了 event.OutboxWriter
eventBus := event.NewBus()                         // 或 event.NewRedisBus(redisClient)
txBus := event.NewTransactionalBus(eventBus, webhookRepo)
```

---

## 使用建议

1. **统一事务入口**：建议通过中间件或 `WithTx` 工具把 GORM 事务注入 `context`，业务代码只需要调用 `bus.PublishEvent(ctx, evt)`，无需判断当前是否有事务。
2. **Webhook 走 `TransactionalBus`**：所有需要触发 Webhook 的事件，都通过 `TransactionalBus` 发布。
3. **非 Webhook 事件可继续使用 `EventBus`**：如审计、统计、缓存失效等不需要强一致性的场景。
4. **不要直接调用 `OutboxHandler`**：`worker.NewOutboxHandler` 仅用于把普通 `EventBus` 事件落库到 outbox，适用于无事务需求的遗留路径；新项目优先使用 `TransactionalBus`。
5. **按需实现 `OutboxWriter`**：如果未来除了 Webhook 还有其他需要事务 outbox 的投递渠道（如邮件、短信），各自实现 `event.OutboxWriter` 即可，无需改动 `event` 包。

---

## 注意事项

- `tx.WithTx(ctx, tx)`（`github.com/roidmc/quotagate/internal/util/tx`）中的 `tx` 必须是 GORM 事务对象（`db.Begin()` 或 `db.Transaction` 回调中的 `*gorm.DB`）。传普通 `*gorm.DB` 也可以工作，但不会随外部事务回滚。
- `TransactionalBus.PublishEvent` 在 ctx 含事务时**不会**走 `EventBus`，因此 Redis/内存订阅者不会立刻收到该事件。Webhook 消费由 `WebhookWorker` 异步完成。
- 事务外调用 `TransactionalBus.PublishEvent` 等价于直接调用底层 `EventBus.PublishEvent`。
- `CreateOutboxEntries` 要求所有读写在同一 `*gorm.DB` 上，以避免 SQLite 等单连接场景下的死锁。`TransactionalBus` 已保证这一点。
