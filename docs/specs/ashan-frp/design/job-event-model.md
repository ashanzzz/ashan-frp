# 任务事件模型 / SSE / 错误码设计

> 适用范围：`ashan-frp` 后端的 `jobs` / `job_events` / `sync_state` / SSE 推送 / 任务错误语义。
> 约束：只描述逻辑语义、状态机、事件协议与重试原则，不包含实现代码、迁移脚本或 ORM 代码。
> 说明：本文与 `backend-schema.md` 配套，字段层面的表结构以 `backend-schema.md` 为准。

## 1. 设计目标

这套设计不是为了“多加几层日志”，而是为了把异步控制平面讲清楚：

1. `jobs` 描述“当前这件事做到哪一步了”。
2. `job_events` 描述“这件事一路发生过什么”。
3. SSE 描述“前端/CLI 如何实时看到这件事的变化”。
4. `sync_state` 描述“这个对象是否还需要同步、是否冲突、何时重试”。
5. 错误码描述“为什么失败、能不能自动重试、需不需要人介入”。

核心原则如下：

- 状态和历史分离。
  - `jobs.status` 是当前快照。
  - `job_events` 是 append-only 轨迹。
- 事件和状态分离。
  - SSE 只是投影层，不是事实源。
- 业务意图和外部执行分离。
  - API 先落本地意图，再由异步 job 处理远端副作用。
- 错误语义要可机读。
  - 每个失败都必须能回答：是否可重试、何时重试、是否需要人工。
- 心跳与健康要可观测。
  - 长任务必须有心跳，系统健康必须能从最近一次成功/失败推导。

## 2. 概念定义

### 2.1 Job

Job 是后端异步执行单元，表示一次可追踪、可重试、可取消的工作。

典型 job 包括：

- 同步远端配置。
- 重建隧道或网站映射。
- 定时健康检查。
- 定时周期同步。
- 故障切换或优化选优。
- 回滚、清理、补偿。

Job 不是业务对象本身；业务对象仍然是 `nodes`、`tunnels`、`website_mappings`、`settings` 等表里的记录。

### 2.2 Event

Event 是 Job 生命周期或同步过程中的一次不可变记录。

Event 只负责记录“发生了什么”，不负责决定“最终状态是什么”。

### 2.3 Channel

Channel 是 SSE 订阅的逻辑范围。

一个 channel 可以是：

- 单个 job 的事件流。
- 某个 account 的作业总览流。
- 某个 subject 的同步流。
- 某个 account 的健康流。

### 2.4 Cursor

Cursor 是 SSE 的续传位置。

Cursor 必须是：

- 由服务端生成。
- 客户端透明保存。
- 重新连接时可直接带回。
- 不依赖客户端自己理解内部排序细节。

### 2.5 Heartbeat

Heartbeat 是长任务执行中的存活信号。

它的目的不是“刷屏”，而是：

- 避免 lease 被误判失效。
- 让 UI 知道任务仍在推进。
- 让调度器能回收真正卡死的任务。

### 2.6 Health Check

Health check 是对系统或某个组件的周期性探测。

健康检查可以由短任务 job 承担，也可以由只读探针直接生成健康事件，但最终都应该能投影到 SSE 和健康状态视图里。

## 3. Job 类型与同步边界

### 3.1 总原则：先落本地，再做远端

凡是需要操作 1Panel、Docker、FRP、DNS 或其他远端系统的动作，都应该遵循两段式：

1. 先同步地保存本地意图或请求。
2. 再异步地执行远端副作用。

这样做的好处是：

- 请求线程不会被长耗时阻塞。
- 失败可以重试。
- 前端可以立即看到 job 进度。
- 同一份意图可以被审计和回放。

### 3.2 同步动作

以下动作应优先保持同步完成，也就是请求返回时已经拿到最终结果，且不依赖长耗时的远端回放：

| 动作类别 | 是否同步 | 说明 |
|---|---|---|
| 读取列表 / 详情 / 统计 | 是 | 纯查询，不创建 job。 |
| 账号登录 / token 刷新 | 是 | 如果一次远端调用即可完成，保持同步。 |
| 本地 settings 修改 | 是 | 只影响本地配置，不涉及远端执行。 |
| 本地意图草稿保存 | 是 | 例如把待同步配置先写入本地表。 |
| job 取消请求 | 是 | 先把控制平面状态改为“取消中/已取消”，远端收尾可异步跟进。 |

### 3.3 异步动作

以下动作应通过 job 执行：

| 动作类别 | 是否异步 | 说明 |
|---|---|---|
| 创建 / 更新 / 删除 `nodes`、`tunnels`、`website_mappings` 的远端投影 | 是 | 只要涉及远端系统，就不要阻塞请求线程。 |
| 周期同步 | 是 | 由调度器触发，必须支持重试与抖动。 |
| 故障切换 / 最优节点切换 | 是 | 往往需要多步远端操作。 |
| 健康探测 job | 是 | 如果涉及远端探测、阈值判断、持久化健康结果，就应作为 job。 |
| 回滚 / 补偿 / 清理 | 是 | 需要事件轨迹和失败原因。 |
| 批量导入 / 批量删除 | 是 | 可能涉及大量对象和分段重试。 |

### 3.4 API 返回语义

当一个请求触发异步 job 时：

- 立即返回本地已保存的对象状态。
- 同时返回 `job_id`、`job_status`、`trace_id`（如果有）。
- SSE 负责把后续变化推给客户端。

如果请求只做本地同步变更，则不应强制创建 job。

## 4. Job 生命周期状态机

### 4.1 状态集合

`jobs.status` 建议使用以下状态：

| 状态 | 含义 | 是否终态 |
|---|---|---|
| `queued` | 已入队，等待 runner 领取或等待 `run_after` 到期 | 否 |
| `running` | 已领取并开始执行 | 否 |
| `retry_wait` | 失败后等待下次重试时间 | 否 |
| `blocked` | 需要人工介入或外部条件解除后才能继续 | 否 |
| `succeeded` | 成功完成 | 是 |
| `failed` | 终态失败，不再自动重试 | 是 |
| `canceled` | 被取消 | 是 |

说明：

- `queued` 包含“可立即执行”和“尚未到达 run_after”两种等待语义。
- `retry_wait` 表示这个 job 仍然活着，只是被延后了。
- `blocked` 表示自动重试不再有意义，必须有人或外部系统介入。

### 4.2 状态转换

推荐的转换如下：

```text
queued -> running -> succeeded
queued -> running -> retry_wait -> queued -> running
queued -> running -> blocked -> queued
queued -> canceled
running -> canceled
running -> failed
retry_wait -> canceled
blocked -> canceled
```

补充规则：

- `queued -> running` 只有在 runner 成功 lease 后才发生。
- `running -> retry_wait` 只有在错误可重试且 `attempt_count < max_attempts` 时发生。
- `running -> blocked` 适用于缺凭据、冲突待决、人工确认等场景。
- 终态之后不应再修改 `jobs.status`，只可追加审计或后继 job。

### 4.3 关键字段的职责

`jobs` 表中的关键列在状态机里的作用如下：

- `attempt_count`：已实际执行的次数。
- `max_attempts`：最多允许自动重试的次数。
- `run_after`：下一次允许开始执行的时间。
- `locked_at` / `locked_by`：当前 lease 的持有信息。
- `result_json`：终态成功时的结构化结果摘要。
- `error_code` / `error_message`：当前终态错误摘要。
- `idempotency_key`：用于去重，避免重复入队。

### 4.4 lease 与回收

如果 job 已经 `running`，但满足以下条件之一：

- `locked_at` 超过 lease timeout 未刷新。
- runner 心跳丢失。
- runner 宕机或重启。

则调度器可以把 job 视为“租约失效”。

处理原则：

- 如果 job 是幂等的，且没有确认产生不可逆副作用，可以回收后重新入队。
- 如果 job 可能已经对远端产生不可逆副作用，则应先标记为 `blocked` 或 `failed`，等待人工确认。

## 5. 事件模型

### 5.1 通用事件封装

SSE 和 `job_events.payload_json` 应使用同一套逻辑封装。推荐字段如下：

| 字段 | 类型 | 含义 |
|---|---|---|
| `schema_version` | int | 事件 payload 版本号，初始建议为 `1`。 |
| `channel` | string | 事件所属 channel。 |
| `kind` | string | 事件类型，建议使用点号分层命名。 |
| `cursor` | string | 续传游标。SSE 的 `id` 线应使用它。 |
| `level` | string | `debug` / `info` / `warn` / `error`。 |
| `message` | string | 面向操作者的一句话摘要。 |
| `job` | object | 事件关联的 job 摘要。 |
| `subject` | object | 事件关联的业务对象摘要。 |
| `actor` | object | 触发事件的主体，可选。 |
| `payload` | object | kind 专属数据。 |
| `error` | object | 失败时的结构化错误，可选。 |
| `trace_id` | string | 追踪 ID，可与 audit_log 对齐。 |
| `created_at` | string | ISO8601 时间戳。 |

### 5.2 事件命名规则

事件类型建议使用点号分层：

- `job.created`
- `job.started`
- `job.progress`
- `job.heartbeat`
- `job.retry_scheduled`
- `job.blocked`
- `job.succeeded`
- `job.failed`
- `job.canceled`
- `job.reclaimed`
- `sync.dirty`
- `sync.diff_ready`
- `sync.conflict`
- `sync.reconciled`
- `health.ok`
- `health.degraded`
- `health.down`
- `health.recovered`

约束：

- `job_events.event_type` 与 SSE `kind` 保持同值。
- 事件名是机器键，不是展示文案。
- 不要把 `message` 当作机器判断条件。

### 5.3 Job 生命周期事件

| kind | 触发时机 | payload 建议字段 |
|---|---|---|
| `job.created` | job 首次入库 | `job`, `target`, `idempotency_key`, `run_after` |
| `job.started` | runner 成功领取并开始执行 | `runner_id`, `attempt_count`, `lease_timeout_seconds` |
| `job.progress` | 执行到阶段节点 | `stage`, `percent`, `detail`, `step_index` |
| `job.heartbeat` | 长任务存活刷新 | `runner_id`, `elapsed_seconds`, `lease_remaining_seconds` |
| `job.retry_scheduled` | 决定重试 | `error`, `attempt_count`, `retry_after`, `backoff_seconds` |
| `job.blocked` | 需要人工介入 | `block_reason`, `required_action`, `hint` |
| `job.reclaimed` | 租约失效后被重新接管 | `previous_runner_id`, `reason`, `attempt_count` |
| `job.succeeded` | 成功完成 | `result_ref`, `summary`, `applied_snapshot_ids` |
| `job.failed` | 终态失败 | `error`, `terminal`, `retryable`, `next_action` |
| `job.canceled` | 被取消 | `canceled_by`, `reason` |

### 5.4 Sync 相关事件

Sync 事件通常由同步引擎发出，或由 job runner 在执行同步 job 时发出：

| kind | 含义 | payload 建议字段 |
|---|---|---|
| `sync.dirty` | 目标对象的期望态已变化 | `subject`, `desired_hash`, `observed_hash` |
| `sync.diff_ready` | 差异已计算完成 | `diff_summary`, `snapshot_ref`, `job_id` |
| `sync.conflict` | 发现冲突、漂移或人工覆盖 | `conflict_reason`, `manual_action`, `observed_hash` |
| `sync.reconciled` | 同步完成且状态收敛 | `applied_snapshot_id`, `result_ref`, `next_retry_at` |
| `sync.retry_scheduled` | 同步被延后重试 | `retry_after`, `backoff_seconds`, `reason` |
| `sync.manual_override` | 人工接管或覆盖 | `override_by`, `override_reason`, `effective_after` |

### 5.5 Health 相关事件

Health 事件用于表达“系统是否还活着、还健康”：

| kind | 含义 | payload 建议字段 |
|---|---|---|
| `health.ok` | 健康正常 | `component`, `last_success_at`, `latency_ms`, `fail_count` |
| `health.degraded` | 有部分异常，但仍可服务 | `component`, `reason`, `threshold_seconds`, `fail_count` |
| `health.down` | 健康不可用 | `component`, `reason`, `last_success_at`, `outage_seconds` |
| `health.recovered` | 从异常恢复 | `component`, `recovered_from`, `recovered_at` |

### 5.6 payload 的组织规则

`payload` 里的数据应遵守以下约束：

- 只放机器需要的最小字段。
- 大对象不要直接塞满事件；应该引用 `snapshot_id`、`result_ref` 或其他可检索引用。
- 不要放密钥、token、完整 secret、或不必要的远端原始响应。
- 结构必须可向后兼容，新增字段可以，删除或改义要提升 `schema_version`。

## 6. SSE channel 名称与续传游标

### 6.1 订阅原则

推荐一个 SSE 连接对应一个逻辑 channel。

理由：

- 续传语义简单。
- 授权边界清晰。
- 前端/CLI 更容易在断线后恢复。
- 不需要在第一版做复杂多路复用。

如果后续确实需要合并多个 channel，可以在上层做聚合视图，但底层 channel 语义不变。

### 6.2 建议的 channel 命名

| channel | 作用范围 | 典型消费者 | 续传特征 |
|---|---|---|---|
| `job.<job_id>` | 单个 job 的完整轨迹 | job 详情页、任务面板 | 以 `job_id + sequence_no` 为主 |
| `account.<account_id>.jobs` | 某个账号下所有 job 的总览 | 总览页、通知中心 | 以投影顺序游标为主 |
| `sync.<subject_type>.<subject_id>` | 某个 subject 的同步状态 | 对象详情页 | 以 `sync_state` 投影游标为主 |
| `health.<account_id>` | 某个账号的健康面板 | 运维看板 | 以健康样本游标为主 |

命名规则：

- 全小写 ASCII。
- 使用点号分段。
- 不包含空格。
- 不包含用户可任意输入的自由文本。

### 6.3 游标格式

cursor 必须是服务端可解析的 opaque string。客户端只需要保存和回传，不需要理解内部结构。

建议约定如下：

- `job.<job_id>` channel：cursor 直接编码 `job_id + sequence_no`。
  - 例如：`v1|job|<job_id>|7`
- `account.<account_id>.jobs` channel：cursor 编码投影排序位置。
  - 例如：`v1|account_jobs|<created_at>|<job_id>|<sequence_no>`
- `sync.<subject_type>.<subject_id>` channel：cursor 编码 subject 的最后一次投影位置。
  - 例如：`v1|sync|<subject_type>|<subject_id>|<updated_at>|<last_job_id>`
- `health.<account_id>` channel：cursor 编码健康样本位置。
  - 例如：`v1|health|<account_id>|<captured_at>|<sample_id>`

### 6.4 续传规则

客户端断线重连时，必须发送：

- 标准 SSE 的 `Last-Event-ID`。
- 或者等价的 `cursor` 查询参数。

服务端在恢复连接后应：

1. 从 cursor 之后的事件开始补发。
2. 保证同一 channel 内顺序稳定。
3. 若 cursor 已超出保留窗口，则返回“需要重新拉取快照”的重置事件。

建议的重置事件：

- `stream.reset_required`

其 payload 至少应说明：

- 需要重新拉取哪个 channel。
- 建议先读哪张快照或哪类列表接口。

### 6.5 顺序规则

- `job.<job_id>` 内部顺序以 `job_events.sequence_no` 为准。
- `account.<account_id>.jobs` 的排序以 `created_at` 为主，`job_id` 和 `sequence_no` 为稳定的次级 tiebreaker。
- `sync` channel 的排序以 `updated_at` 为主。
- `health` channel 的排序以采样时间为主。

## 7. 错误类、错误码与重试语义

### 7.1 错误码命名规则

建议使用稳定的全大写蛇形命名：

- `VALIDATION_*`
- `AUTH_*`
- `CONFLICT_*`
- `REMOTE_*`
- `JOB_*`
- `HEALTH_*`
- `SYSTEM_*`

原则：

- 错误码是协议，不是文案。
- 同一个错误码的语义不要轻易漂移。
- `error_message` 可以更友好，但不应该被客户端拿来做分支判断。

### 7.2 错误分类

| 类别 | 典型错误码 | HTTP 语义 | 是否自动重试 | 处理结果 |
|---|---|---|---|---|
| 校验错误 | `VALIDATION_INVALID_INPUT`、`VALIDATION_MISSING_FIELD` | 400 / 422 | 否 | 直接失败 `failed` |
| 认证 / 授权 | `AUTH_UNAUTHORIZED`、`AUTH_FORBIDDEN`、`AUTH_TOKEN_EXPIRED` | 401 / 403 | 一般否 | 需要刷新凭据或重新登录 |
| 冲突 / 前置条件 | `CONFLICT_STATE_STALE`、`CONFLICT_DUPLICATE_JOB`、`CONFLICT_MANUAL_OVERRIDE` | 409 | 默认否 | 进入 `blocked` 或 `failed` |
| 远端瞬时故障 | `REMOTE_TIMEOUT`、`REMOTE_UNAVAILABLE`、`REMOTE_RATE_LIMITED`、`REMOTE_BAD_GATEWAY` | 502 / 503 / 504 / 429 | 是 | 进入 `retry_wait` |
| lease / 调度异常 | `JOB_LEASE_LOST`、`JOB_RECLAIMED`、`JOB_ALREADY_RUNNING` | 423 / 409 | 视幂等性而定 | 可回收或阻塞 |
| 终态系统故障 | `SYSTEM_SERIALIZATION_ERROR`、`SYSTEM_INTERNAL_ERROR` | 500 | 视安全性而定 | 通常失败，少量可重试 |
| 健康异常 | `HEALTH_DOWN`、`HEALTH_DEGRADED` | 200 也可，但带降级状态 | 否 | 记录健康事件，不一定算 job 失败 |

### 7.3 retryable 的判定

默认规则：

- 只有“幂等 + 远端瞬时异常 + 未超过最大次数”才自动重试。
- 校验错误、冲突、权限不足、手工覆盖等默认不自动重试。
- 如果 job 可能已经对远端造成不可逆副作用，不能盲目重试。

### 7.4 重试策略

建议采用“指数退避 + 全抖动（full jitter）”：

- `base_delay`：基础等待时间。
- `backoff = min(max_delay, base_delay * 2^(attempt_count - 1))`
- `retry_after = now + random(0, backoff)`

推荐默认值：

- `job.retry_base_seconds = 5`
- `job.retry_max_seconds = 300`
- `job.retry_jitter_seconds = 按 backoff 全抖动实现`

补充规则：

- `attempt_count` 每次真正开始执行时加 1。
- `retry_wait` 状态必须写入 `run_after`。
- 如果 `attempt_count >= max_attempts`，则转为 `failed`，错误码建议改为 `JOB_MAX_ATTEMPTS`。
- 对 `REMOTE_RATE_LIMITED` 可以使用更长的基础延迟。
- 对 `AUTH_TOKEN_EXPIRED` 不应无限重试；更合理的是进入 `blocked` 并等待凭据刷新。

### 7.5 错误对象建议形状

事件里的 `error` 对象建议包含：

- `class`：错误类。
- `code`：稳定错误码。
- `http_status`：如果映射到 HTTP，给出状态码。
- `retryable`：是否可自动重试。
- `manual_action_required`：是否需要人工。
- `message`：简短说明。
- `details`：可选的结构化补充。

## 8. 周期同步与随机抖动

### 8.1 为什么需要 jitter

如果所有账号、节点、隧道都在同一秒触发同步，就会产生：

- 远端 API 瞬时峰值。
- 本地队列拥塞。
- SSE 突发刷屏。
- 重试雪崩。

因此周期同步和健康检查都必须带随机抖动。

### 8.2 触发方式

建议把周期任务视为“按窗口入队的 job”，而不是“固定时间点硬跑”。

每次调度时：

1. 先确定基础周期窗口。
2. 再在窗口内随机偏移一个 jitter。
3. 生成 job 时带上 `idempotency_key`，避免重启后重复入队。

### 8.3 推荐参数

可通过 `settings` 表保存如下逻辑配置键：

| setting key | 建议默认值 | 含义 |
|---|---|---|
| `job.sync_interval_seconds` | `300` | 周期同步基础间隔 |
| `job.sync_jitter_seconds` | `60` | 周期同步随机抖动窗口 |
| `job.health_interval_seconds` | `60` | 健康检查基础间隔 |
| `job.health_jitter_seconds` | `15` | 健康检查随机抖动窗口 |
| `job.heartbeat_interval_seconds` | `15` | 长任务心跳间隔 |
| `job.lease_timeout_seconds` | `45` | 任务租约超时 |
| `job.retry_base_seconds` | `5` | 自动重试基础退避 |
| `job.retry_max_seconds` | `300` | 自动重试最大退避 |
| `job.fastest_cooldown_seconds` | `900` | 主动选优冷却时间 |

说明：

- 上述键只是推荐逻辑名，实际实现可在不破坏语义的前提下调整命名。
- 对同一 subject，随机抖动应避免所有对象在同一瞬间同步。
- 如果后续要进一步稳定分布，可以给 jitter 加 subject key 的随机种子，但第一版只要“随机且有限窗口”即可。

### 8.4 幂等性要求

周期任务必须配合 `idempotency_key`：

- 同一窗口内重复调度，应命中同一个 job，或明确去重。
- 调度器重启后，不能因为重新扫描而把同一对象入队多次。
- 对 `health`、`sync`、`fastest` 这类周期性动作尤其重要。

## 9. 心跳 / 健康检查模型

### 9.1 长任务心跳

如果一个 job 预计执行时间超过心跳间隔，就必须发心跳事件。

建议规则：

- 第一次 `job.heartbeat` 最晚不迟于 `running` 后 15 秒。
- 之后每隔 `job.heartbeat_interval_seconds` 刷新一次。
- 每次心跳都应刷新 lease freshness（通常是更新 `locked_at`）。

心跳的意义：

- 告诉 UI“任务还活着”。
- 告诉调度器“这个 lease 还有效”。
- 告诉运维“卡死还是只是慢”。

### 9.2 心跳丢失的处理

如果超过 `job.lease_timeout_seconds` 未收到心跳：

- 先视为 lease 失效。
- 再根据 job 是否幂等、是否存在不可逆副作用来决定回收、重试或阻塞。
- 必须发出 `job.reclaimed`、`job.blocked` 或 `job.failed` 之一，让客户端可见。

### 9.3 健康检查 job

健康检查可以视为一种特殊 job：`health.check`。

它的职责是：

- 探测本地执行器是否正常。
- 探测远端 API 是否可达。
- 统计最近失败率、延迟、告警阈值。
- 产出健康事件供 SSE 和看板展示。

### 9.4 健康状态分级

建议使用以下健康状态：

| 状态 | 含义 | 典型判定 |
|---|---|---|
| `unknown` | 尚未采样或数据不足 | 首次启动、首次探测前 |
| `ok` | 最近一次探测成功，且无持续异常 | 最近成功在阈值内 |
| `degraded` | 仍可工作，但失败或延迟上升 | 超过一部分阈值、局部组件异常 |
| `down` | 明确不可用 | 连续失败、超时、心跳丢失 |

### 9.5 健康视图的来源

健康视图不应该依赖一张独立“事实真理表”。

它可以由以下信息综合得到：

- 最近的 `health.check` job 结果。
- 最近的 `job.heartbeat`。
- `jobs` 的失败趋势。
- `sync_state` 的冲突和重试积压。

也就是说，健康是“投影结果”，不是“单点写死”。

## 10. 与现有脚本 / 工作流的关系

README 中的现有 shell 工作流可以被映射为以下 job 家族：

| 现有命令语义 | 建议 job 家族 | 说明 |
|---|---|---|
| `health` | `health.check` | 周期健康检查与看板刷新。 |
| `failover` | `sync.reconcile` / `tunnel.failover` | 发现离线后进行恢复与切换。 |
| `fastest` | `sync.optimize` / `tunnel.optimize` | 基于冷却时间和抖动做主动选优。 |
| `userinfo` | `account.refresh` | 同步用户信息或授权状态。 |
| `nodes` | `node.refresh` | 刷新候选节点列表。 |

映射原则：

- 一条 shell 命令可以触发一个 job，也可以触发一组子 job。
- 如果动作会产生远端副作用，就必须有 job 和事件轨迹。
- 如果只是本地读写或展示，不必强行异步化。

## 11. 与 `backend-schema.md` 的对应关系

为了避免文档之间语义漂移，这里再明确一次对应关系：

- `jobs.status`：本文件定义的生命周期状态。
- `jobs.run_after`：`queued` / `retry_wait` 的调度时间。
- `jobs.locked_at` / `locked_by`：lease 与回收依据。
- `jobs.result_json`：终态成功的结构化摘要。
- `jobs.error_code` / `error_message`：终态失败摘要。
- `job_events.sequence_no`：同一 job 内的严格顺序。
- `job_events.event_type`：事件 kind 的持久化值。
- `job_events.payload_json`：事件的结构化详情。
- `sync_state.next_retry_at`：同步引擎的下一次计划重试时间。
- `sync_state.status`：对象同步状态投影。
- `snapshots`：需要保存的远端原始观测或规范化观测。

## 12. 小结

这份设计的目标是：

- 让异步 job 有清晰的状态机。
- 让事件有稳定的 kind、payload 和 cursor。
- 让 SSE 能断线续传。
- 让错误可分类、可重试、可阻塞。
- 让周期同步和健康检查不会在同一时间点挤爆系统。
- 让 UI / CLI 能实时看到“发生了什么”。

只要 `jobs`、`job_events`、`sync_state`、`snapshots` 这几层边界保持清晰，后续无论是接 1Panel、接 FRP，还是做更复杂的对账和人工接管，都会更稳。
