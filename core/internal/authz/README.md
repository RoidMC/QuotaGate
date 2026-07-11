# Authz 授权模块

`internal/authz` 是 QuotaGate 的授权决策中心，采用**域 RBAC 为主、ABAC 为辅、ReBAC 下沉到业务层**的分层模型。

本模块当前使用**单个全局 Casbin SyncedEnforcer**，所有用户-角色关系与角色继承关系在启动时加载到内存，运行时直接调用 `Enforce`，无需每请求重建 Enforcer。

---

## 目录

1. [架构概览](#架构概览)
2. [核心设计决策](#核心设计决策)
3. [关键组件](#关键组件)
4. [请求处理流程](#请求处理流程)
5. [多实例同步](#多实例同步)
6. [性能基准](#性能基准)
7. [安全边界](#安全边界)
8. [配置](#配置)
9. [相关文件](#相关文件)

---

## 架构概览

```text
请求 → BearerAuth → Authz Middleware
            |
            v
    [sanitizePath 路径遍历检查]
            |
            v
    [可选] Token 角色与 DB 严格校验
            |
            v
    [主 RBAC Enforcer] --拒绝--> 403
            |
          通过
            |
    [租户隔离二次校验：objOwner == "*" 或 objOwner == tenantID]
            |
          通过
            |
    [RouteMeta 规则类型]
            |
    rbac  → 放行
    abac  → [ABAC Enforcer] --拒绝--> 403
    rebac → 由 handler/service 层做实例级所有权校验
```

### 与旧架构的区别

旧方案（已废弃）每请求创建一个临时 Enforcer，从 base Enforcer 拷贝 `p` 策略、从 `RoleRegistry` 拷贝继承规则、注入当前用户 `g` 关系后执行一次即丢弃。该方案在 benchmark 中表现约为 **430 µs / 请求**，内存与分配开销都很高。

当前方案使用单个全局 `SyncedEnforcer`：

- `p` 策略从 `casbin_rule` 表加载。
- 角色继承 `g` 规则从 `RoleDefinition.InheritedRoles` 派生，仅存内存。
- 用户-角色 `g` 规则从 `UserRoleAssignment` 加载，仅存内存。
- 运行时直接 `Enforce`，无需重建。

---

## 核心设计决策

### 1. 主 RBAC 采用五元组 + 域

模型定义：

```text
[request_definition]
r = subOwner, subName, method, urlPath, objOwner

[policy_definition]
p = subOwner, subName, method, urlPath, objOwner

[role_definition]
g = _, _, _

[matchers]
m = g(r.subName, p.subName, r.subOwner) && keyMatch(r.subOwner, p.subOwner) && r.method == p.method && keyMatch3(r.urlPath, p.urlPath) && keyMatch(r.objOwner, p.objOwner)
```

- `subOwner`：请求发起者所属租户 ID。
- `subName`：用户 ID。未认证请求不进入 Authz 流程（由 router 层挂载到 public 路由组），因此 `subName` 不会为空。
- `objOwner`：被访问资源所属租户 ID；系统级资源用 `"*"`。
- `keyMatch` 让系统策略（`subOwner = "*"` 或 `objOwner = "*"`）命中任意租户请求。

### 2. 全局 SyncedEnforcer

- 使用 `casbin.SyncedEnforcer` 提供并发读保护。
- 启动时调用 `LoadPolicy()` 加载 `p` 策略。
- 调用 `InitGroupingRelations()` 批量注入继承 `g` 规则与用户-角色 `g` 规则。
- `g` 规则写入前关闭 `EnableAutoSave(false)`，确保不会持久化到 `casbin_rule` 表。

### 3. g 规则内存化

- `baseEnforcer` / 主 Enforcer 的 adapter 只存储 `p` 策略。
- `RoleRegistry` 在内存中维护从 `RoleDefinition.InheritedRoles` 派生的角色继承 `g` 规则。
- `AuthzManager.lastAssignments` 缓存用户-角色分配，支持 `ReloadPolicy()` 后快速重建 `g` 规则。
- 用户-角色分配变更通过 `AssignUserRole` / `RevokeUserRole` 直接操作内存中的全局 Enforcer，不写入数据库之外的持久化。

### 4. ABAC 作为二级校验

- 独立模型：`r = sub, dom, method, path, obj`。
- 独立表：`casbin_rule_abac`。
- `sub` / `obj` 为结构体，支持 `ID`、`Owner`、`Roles []interface{}`、`Attrs map[string]any`。
- `subRule` / `objRule` 通过白名单 parser 校验：
  - 仅允许 `==`、`!=`、`in` 运算符。
  - 仅允许预注册属性：`r.sub.ID`、`r.sub.Owner`、`r.sub.Roles`、`r.sub.Attrs.*`、`r.obj.Owner`、`r.obj.Name`、`r.obj.Attrs.*`。
  - 规则必须引用至少一个请求属性，纯常量表达式一律拒绝。
  - 允许交叉引用：`subRule` 可引用 `r.obj.*`，`objRule` 可引用 `r.sub.*`。

### 5. RouteMeta 驱动规则类型

- `RouteMeta{TenantID, Method, Path, RuleType}` 决定某条路由走 RBAC / ABAC / ReBAC。
- 启动时加载到内存索引，管理端变更后 reload。
- `ruleType` 与 Casbin 策略定义分离，不污染策略语义。

### 6. 租户隔离双防线

- 第一道：Casbin `keyMatch` 让系统策略命中所有租户请求。
- 第二道：middleware 强制校验 `objOwner == "*" || objOwner == tenantID`，防止跨租户策略匹配绕过。
- 系统级继承规则使用域 `"*"`，并通过 `AddDomainMatchingFunc("keyMatch", util.KeyMatch)` 匹配任意具体租户域。

---

## 关键组件

| 文件 | 职责 |
|------|------|
| [authz.go](authz.go) | `AuthzManager`：全局 RBAC/ABAC Enforcer 管理、策略 CRUD、跨实例事件发布/订阅。 |
| [rolegistry.go](rolegistry.go) | `RoleRegistry`：从 `RoleDefinition` 递归展开继承链，按域维护内存 `g` 规则。 |
| [model.go](model.go) | Casbin model 字符串：`RBACWithDomainsModel`、`ABACWithDomainsModel`；`Subject`/`Object` 结构体。 |
| [policy.go](policy.go) | 默认 RBAC 五元组策略、默认系统角色、默认 `RouteMeta`。 |
| [abac_validator.go](abac_validator.go) | ABAC `subRule` / `objRule` 白名单 parser，防止恒真绕过。 |
| [authz_bench_test.go](authz_bench_test.go) | 性能基准：全局 Enforcer、临时 Enforcer、10k 用户、启动加载。 |
| [test/authz_test.go](test/authz_test.go) | 单元测试：域 RBAC、角色继承、ABAC 校验与执行。 |
| [test/authz_event_test.go](test/authz_event_test.go) | 跨实例事件同步测试。 |
| [test/middleware_test.go](test/middleware_test.go) | HTTP 中间件测试：允许/拒绝/路径遍历/跨租户。 |
| [test/traversal_extreme_test.go](test/traversal_extreme_test.go) | 路径遍历边界测试。 |

---

## 请求处理流程

1. Middleware 从 Token context 读取 `userID`、`tenantID`、`roles`。
2. 路径 `sanitizePath` 阻止 `..` 遍历。
3. 若启用 `StrictValidation`，比较 Token roles 与 DB effective roles。
4. 调用 `AuthzManager.EnforceRBAC(ctx, tenantID, userID, roles, method, path, objOwner)`。
   - `roles` 参数保留兼容性，实际权限由全局 Enforcer 中的 `g` 规则解析。
5. RBAC 通过后检查 `objOwner` 租户隔离。
6. 查询 `RouteMeta.RuleType(method, path)`：
   - `rbac`：放行。
   - `abac`：构造 `Subject`/`Object` 调用 `EnforceABAC`。
   - `rebac`：由业务层处理实例级权限。

---

## 多实例同步

当部署多个 QuotaGate 实例时，每个实例都持有独立的内存 Enforcer。角色/分配变更需要通过事件总线同步到其他实例。

### 事件类型

| 事件 | 触发时机 | 处理行为 |
|------|----------|----------|
| `role.assign` | `RoleService.AssignRoleToUser` 成功 | 远端实例调用 `AssignUserRole` 添加本地 `g` 规则。 |
| `role.revoke` | `RoleService.RevokeRoleFromUser` 成功 | 远端实例调用 `RevokeUserRole` 移除本地 `g` 规则。 |
| `role.changed` | 角色定义增删改 | 远端实例通过 `groupingLoader` 重新加载全部角色定义与分配，重建 `g` 规则。 |

### 实现要点

- `AuthzManager.WithEventBus(bus, instanceID, loader)` 将 Manager 与事件总线绑定。
- `instanceID` 用于过滤本机发出的事件，避免重复应用。
- `SubscribeToEvents()` 在启动完成后订阅三类事件。
- `Close()` 取消订阅，保证优雅退出。
- `boot.InitEventBus()` 根据配置选择 Redis（多实例）或内存（单实例）后端。

### 事件丢失兜底

当前实现依赖事件总线的可靠性。若出现事件丢失，可通过以下方式恢复：

- 重启实例：启动时会全量重新加载 `g` 规则。
- 运行时心跳校验：可基于 `kexswiftdb` 维护全局版本号，检测到版本漂移时主动 `ReloadGroupingRelations`。

---

## 性能基准

以下结果在 Windows / AMD Ryzen 5 7500F 上测得，使用 `go test ./internal/authz/ -bench=BenchmarkAuthz -benchtime=1s -benchmem`：

| Benchmark | 延迟 | 内存 / op | 分配 / op | 说明 |
|-----------|------|-----------|-----------|------|
| `GlobalEnforcer` | ~74–108 µs | ~62 KB | ~884 | 2 个 `g` 关系基线。 |
| `TempEnforcerPerRequest` | ~430–462 µs | ~241 KB | ~4761 | 旧方案，每请求新建 Enforcer。 |
| `10kUsers` | ~63–97 µs | ~61 KB | ~888 | 100 租户、10k 用户、30k `g` 关系。 |
| `InitGroupingRelations_10k` | ~96–118 ms | ~25 MB | ~393k | 30k `g` 关系冷启动加载。 |

### 结论

- **运行时**：全局 Enforcer 在 30k `g` 关系下仍保持 <100 µs，性能基本不受 `g` 关系数量线性影响。Casbin 的 per-domain RoleManager 使用哈希 + 树查找，扩展性良好。
- **启动时**：30k `g` 关系冷加载约 100 ms 级，远低于 500 ms 经验阈值，可接受。
- **旧方案**：每请求临时 Enforcer 比全局方案慢 5–6 倍，内存多 4 倍，分配多 5 倍，已废弃。

### 后续优化方向（按需）

- `EnforceRBAC` 单次调用仍有 ~884 次堆分配，主要来自 Casbin 内部临时对象。若未来 QPS 达到万级且 GC 成为瓶颈，可考虑在调用层引入 `sync.Pool` 或评估 Casbin 的 `EnforceEx` / 预编译路径。
- 当 `g` 关系达到 10 万级以上或启动加载超过 500 ms 时，可考虑 Filtered Policy 按租户懒加载。

---

## 安全边界

1. **public 路由隔离**
   - `/api/auth/*` 等公开路由不挂载 Authz middleware，避免策略配置错误导致系统锁定。
   - 未认证请求永远不会进入授权决策流程。

2. **subName 非空**
   - 由于未认证请求走 public 路由，进入 Authz 流程的请求 `subName`（userID）始终非空，不再处理 `anonymous` 角色。

3. **g 规则不落库**
   - 角色继承与用户-角色分配只存内存，避免 Casbin adapter 双写一致性问题。
   - 写 `g` 规则前 `EnableAutoSave(false)`，写完后恢复。

4. **ABAC 白名单 parser**
   - 防止 `subRule` / `objRule` 被构造为恒真表达式。
   - 仅允许预注册属性和有限运算符。

5. **跨租户二次校验**
   - middleware 在 RBAC 通过后校验 `objOwner`，确保资源访问不跨租户。

6. **系统域隔离**
   - 系统级继承规则使用域 `"*"` 并通过 `keyMatch` 匹配具体租户域。
   - 租户无法创建名为 `"*"` 的角色或覆盖系统继承关系。

---

## 配置

```yaml
authz:
  enable_abac:       false   # 是否启用二级 ABAC enforcer
  strict_validation: false   # Token 角色与 DB 不一致时是否拒绝
```

- `enable_abac=false` 时，ABAC 相关 API 返回错误。
- `strict_validation=true` 适合高安全场景；常规场景建议依赖角色变更后 Token 撤销。

---

## 相关文件

- [../middleware/authz.go](../middleware/authz.go) - HTTP 授权中间件
- [../boot/authz.go](../boot/authz.go) - 启动时初始化系统角色、默认策略、RouteMeta、事件订阅
- [../boot/event.go](../boot/event.go) - 事件总线初始化（Redis / 内存）
- [../config/config.go](../config/config.go) - `AuthzConfig`
- [../model/role.go](../model/role.go) - `RoleDefinition`、`UserRoleAssignment`、`RoleMutualExclusion`、`RouteMeta`
- [../repository/role.go](../repository/role.go) - 角色与分配数据访问
- [../repository/route_meta.go](../repository/route_meta.go) - RouteMeta 数据访问
- [../service/role.go](../service/role.go) - 角色业务逻辑与事件发布
- [../service/route_meta.go](../service/route_meta.go) - 路由元数据业务逻辑
- [../handler/role.go](../handler/role.go) - 角色管理 REST API
- [../handler/policy.go](../handler/policy.go) - 策略管理 REST API
- [../handler/route_meta.go](../handler/route_meta.go) - RouteMeta 管理 REST API
