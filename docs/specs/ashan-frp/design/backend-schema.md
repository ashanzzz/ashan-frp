# 后端数据模型 / 表 / 索引设计

> 适用范围：`ashan-frp` 后端持久化层。
> 约束：只描述逻辑 schema、索引与服务边界，不包含实现代码、迁移脚本或 ORM 代码。
> 说明：仓库当前仍处于设计/研究阶段，本文是目标设计，不是现有实现的逐行复述。

## 1. 设计目标

这套 schema 的目标不是“把所有状态都塞进一张大表”，而是把三类信息分开：

1. 账号与权限：谁在操作系统。
2. 业务意图：应该存在什么节点、隧道、网站映射。
3. 运行事实：当前实际观察到什么、最近一次执行结果是什么。

核心原则如下：

- 稳定身份和运行时身份分离。
  - `canonical_key` 表示稳定业务键。
  - `runtime_key` 表示当前实例键，用于重建、漂移和替换检测。
- 期望态和观测态分离。
  - `tunnels`、`website_mappings`、`settings` 是期望态或意图态。
  - `snapshots`、`sync_state`、`job_events` 是观测态或过程态。
- 日志表一律 append-only。
  - `job_events` 和 `audit_log` 不承担业务状态存储职责。
- 密钥和配置分离。
  - secret 放 `upstream_credentials`。
  - 非敏感配置放 `settings`。
- 外部系统不是事实源。
  - 1Panel、Docker、FRP 的远端响应只做输入和对账，不直接作为本地状态真理。

## 2. 领域关系总览

建议的基本关系如下：

- `accounts` 1:N `auth_tokens`
- `accounts` 1:N `upstream_credentials`
- `accounts` 1:N `nodes`
- `accounts` 1:N `tunnels`
- `accounts` 1:N `website_mappings`
- `accounts` 1:N `jobs`
- `accounts` 1:N `settings`
- `nodes` 1:N `tunnels`
- `nodes` 1:N `website_mappings`
- `jobs` 1:N `job_events`
- 任意可同步对象 1:1 `sync_state`
- 任意可观测对象 1:N `snapshots`
- 任意对象 1:N `audit_log`

这里的“任意可同步对象”通常指 `nodes`、`tunnels`、`website_mappings` 和部分 `settings`。

## 3. 稳定键策略

本文后续所有表都默认遵循这三个键：

- `external_id`：上游系统提供的稳定资源 ID。
- `canonical_key`：本地生成的稳定业务键，用于判断“是不是同一件业务”。
- `runtime_key`：本地生成的运行时实例键，用于判断“是不是同一实例”。

推荐约定：

- `external_id` 只在上游真的提供稳定 ID 时使用。
- `canonical_key` 必须可重建，且尽量不包含会漂移的字段。
- `runtime_key` 可以包含实例 ID、宿主端口、当前 host/IP 等会变化的内容。
- `runtime_key` 只做索引和对账，不做长期唯一真理。

## 4. 服务边界

### 4.1 API 层

职责：

- 身份认证、权限校验、输入校验。
- 对 `accounts`、`auth_tokens`、`upstream_credentials`、`nodes`、`tunnels`、`website_mappings`、`settings` 提供 CRUD。
- 把用户动作转成 `jobs`，而不是直接执行业务变更。
- 读取 `sync_state`、`snapshots`、`job_events`、`audit_log` 作为查询视图。

不做的事：

- 不直接调用 1Panel API。
- 不直接做同步决策。
- 不在请求线程里执行长耗时回放、重试或补偿。

### 4.2 同步引擎

职责：

- 根据期望态与观测态计算差异。
- 维护 `canonical_key` / `runtime_key` / `external_id` 的映射关系。
- 判断冲突、漂移、失效、人工接管。
- 生成或更新 `jobs`，并写入 `sync_state`、`snapshots`。

不做的事：

- 不负责 HTTP 接口返回格式。
- 不负责具体的 1Panel 调用细节。
- 不负责排队锁和重试调度的底层实现。

### 4.3 Job Runner

职责：

- 领取可执行任务、加锁、跑重试。
- 更新 `jobs` 的状态机。
- 追加 `job_events`。
- 在任务成功或失败后刷新 `sync_state`。

不做的事：

- 不承载用户交互。
- 不承载业务领域的唯一性判断。
- 不把临时执行结果当成最终事实。

### 4.4 1Panel Adapter

职责：

- 只做 1Panel API 的请求/响应映射。
- 把 1Panel 的 DTO 规范化成同步引擎可消费的结构。
- 记录远端返回的原始快照供 `snapshots` 使用。

不做的事：

- 不保存业务状态。
- 不决定是否覆盖人工配置。
- 不自己判定冲突优先级。

## 5. 表设计

### 5.1 `accounts`

- 主键：`id`（UUID/ULID）。
- 重要列：`login_name`、`email`、`display_name`、`role`、`status`、`password_hash`、`password_algo`、`last_login_at`、`created_at`、`updated_at`、`deleted_at`。
- 唯一约束：`login_name` 唯一；`email` 在规范化后可选唯一。
- 索引：`role, status`；`status, updated_at desc`；`created_at desc`。
- 为什么存在：本地操作员身份、权限判断、所有权归属和审计归因的根表。

### 5.2 `auth_tokens`

- 主键：`id`（UUID/ULID）。
- 重要列：`account_id`、`token_hash`、`token_prefix`、`token_name`、`scopes_json`、`issued_at`、`expires_at`、`revoked_at`、`last_used_at`、`created_ip`、`user_agent`、`created_at`、`updated_at`。
- 唯一约束：`token_hash` 唯一；如果需要人类可读 token 名称，也可补充 `account_id + token_name` 唯一。
- 索引：`account_id`；`expires_at`；`revoked_at`；`last_used_at desc`。
- 为什么存在：承载本地 API/session token，支持轮换、吊销、过期清理和使用追踪。

### 5.3 `upstream_credentials`

- 主键：`id`（UUID/ULID）。
- 重要列：`account_id`、`provider`、`name`、`credential_type`、`secret_ciphertext`、`refresh_token_ciphertext`、`scopes_json`、`token_expires_at`、`status`、`last_validated_at`、`last_error_at`、`last_error_message`、`metadata_json`、`created_at`、`updated_at`、`deleted_at`。
- 唯一约束：`account_id + provider + name` 唯一。
- 索引：`account_id, provider, status`；`provider, token_expires_at`；`updated_at desc`。
- 为什么存在：隔离外部集成密钥与本地登录态，支持 1Panel、FRP、Cloudflare 或其他上游凭据轮换与验证。

### 5.4 `nodes`

- 主键：`id`（UUID/ULID）。
- 重要列：`account_id`、`provider`、`external_id`、`canonical_name`、`display_name`、`node_type`、`endpoint_url`、`region`、`status`、`health_status`、`ban_until`、`last_seen_at`、`last_success_at`、`last_error_code`、`last_error_message`、`metadata_json`、`created_at`、`updated_at`、`archived_at`。
- 唯一约束：优先 `account_id + provider + external_id`；如果某个 provider 没有稳定外部 ID，则退回 `account_id + provider + canonical_name`。
- 索引：`account_id, status`；`provider, health_status`；`ban_until`；`last_seen_at desc`。
- 为什么存在：统一表示可执行目标或上游节点（例如 Docker 主机、1Panel 实例、FRP 节点），并记录健康、封禁和可用性。

### 5.5 `tunnels`

- 主键：`id`（UUID/ULID）。
- 重要列：`account_id`、`node_id`、`external_id`、`name`、`canonical_key`、`runtime_key`、`tunnel_type`、`local_ip`、`local_port`、`remote_port`、`dns_domain_cname`、`dns_proxied`、`desired_state`、`actual_state`、`state_reason`、`desired_hash`、`observed_hash`、`last_applied_snapshot_id`、`last_applied_at`、`last_error_code`、`last_error_message`、`manual_override_json`、`created_at`、`updated_at`、`archived_at`。
- 唯一约束：`account_id + node_id + canonical_key` 唯一；如果上游提供稳定隧道 ID，则 `account_id + node_id + external_id` 也应唯一。
- 索引：`account_id, node_id, desired_state`；`node_id, last_applied_at desc`；`canonical_key`；`runtime_key`。
- 为什么存在：保存固定隧道定义与当前运行位置，支持“同一业务、不同实例”的重建语义，以及与 DNS / 远端端口 / 协议相关的对账。

### 5.6 `website_mappings`

- 主键：`id`（UUID/ULID）。
- 重要列：`account_id`、`node_id`、`tunnel_id`（如果映射来源就是隧道）、`source_kind`、`source_external_id`、`canonical_key`、`runtime_key`、`panel_website_id`、`website_alias`、`primary_domain`、`domains_json`、`proxy_target`、`https_enabled`、`https_port`、`http_config`、`ssl_certificate_ref`、`proxy_enabled`、`proxy_cache_enabled`、`manual_override_json`、`status`、`last_synced_at`、`last_remote_hash`、`last_error_code`、`last_error_message`、`created_at`、`updated_at`、`archived_at`。
- 唯一约束：`account_id + node_id + canonical_key` 唯一；`account_id + node_id + panel_website_id` 唯一；如果 `primary_domain` 是系统主查找键，则对活动记录再加唯一约束。
- 索引：`account_id, node_id, status`；`primary_domain`；`panel_website_id`；`canonical_key`；`runtime_key`；`last_synced_at desc`。
- 为什么存在：把容器端口 / 隧道定义投影成 1Panel 网站对象，保存域名、代理、HTTPS 和人工覆盖痕迹，避免远端网站状态变成唯一事实源。

### 5.7 `jobs`

- 主键：`id`（UUID/ULID）。
- 重要列：`account_id`、`job_type`、`target_type`、`target_id`、`idempotency_key`、`priority`、`status`、`run_after`、`locked_at`、`locked_by`、`attempt_count`、`max_attempts`、`payload_json`、`result_json`、`error_code`、`error_message`、`requested_by_account_id`、`created_at`、`updated_at`、`started_at`、`completed_at`、`archived_at`。
- 唯一约束：`account_id + idempotency_key` 唯一（如果该键被提供）。
- 索引：`status, run_after, priority desc, created_at`；`target_type, target_id`；`requested_by_account_id, created_at desc`；`job_type, status`。
- 为什么存在：作为 API 与执行器之间的持久队列，负责去重、调度、重试、回放和失败恢复。

### 5.8 `job_events`

- 主键：`id`（UUID/ULID）。
- 重要列：`job_id`、`sequence_no`、`event_type`、`level`、`message`、`payload_json`、`trace_id`、`created_by`、`created_at`。
- 唯一约束：`job_id + sequence_no` 唯一，保证同一个 job 内事件顺序稳定。
- 索引：`job_id, sequence_no`；`event_type, created_at desc`；`trace_id`。
- 为什么存在：保存 job 的执行轨迹、步骤、重试原因和失败上下文；它是过程日志，不是业务主状态。

### 5.9 `sync_state`

- 主键：`id`（UUID/ULID）。
- 重要列：`account_id`、`subject_type`、`subject_id`、`desired_hash`、`observed_hash`、`last_snapshot_id`、`last_job_id`、`status`、`conflict_reason`、`dirty`、`retry_count`、`next_retry_at`、`locked_until`、`last_success_at`、`last_attempt_at`、`last_error_code`、`last_error_message`、`manual_override_at`、`manual_override_by`、`metadata_json`、`updated_at`。
- 唯一约束：`account_id + subject_type + subject_id` 唯一。
- 索引：`account_id, status, next_retry_at`；`subject_type, updated_at desc`；`dirty, next_retry_at`。
- 为什么存在：这是同步引擎的短期记忆，避免每次都扫描全量快照和 job 历史来判断是否需要重跑、是否冲突、是否需要人工接管。

### 5.10 `audit_log`

- 主键：`id`（UUID/ULID）。
- 重要列：`account_id`、`actor_type`、`actor_id`、`action`、`subject_type`、`subject_id`、`request_id`、`job_id`、`before_json`、`after_json`、`diff_json`、`source_ip`、`user_agent`、`severity`、`created_at`。
- 唯一约束：无硬业务唯一约束；它是 append-only 审计账本。
- 索引：`subject_type, subject_id, created_at desc`；`actor_id, created_at desc`；`action, created_at desc`；`job_id`。
- 为什么存在：提供面向人和安全审计的不可变追踪，回答“谁在什么时候改了什么、为什么改、改前改后是什么”。

### 5.11 `settings`

- 主键：`id`（UUID/ULID）。
- 重要列：`account_id`、`scope_type`、`scope_id`、`key`、`value_json`、`value_type`、`description`、`updated_by`、`updated_at`、`created_at`、`deleted_at`。
- 唯一约束：`account_id + scope_type + scope_id + key` 唯一；如果是纯全局设置，则 `scope_type = global`、`scope_id = null`。
- 索引：`account_id, scope_type, scope_id`；`key`；`updated_at desc`。
- 为什么存在：保存非敏感系统配置、功能开关、默认值、重试参数、冲突策略和人工覆盖策略；密钥不应放在这里。

### 5.12 `snapshots`

- 主键：`id`（UUID/ULID）。
- 重要列：`account_id`、`source_system`、`source_ref`、`subject_type`、`subject_id`、`snapshot_kind`、`content_json`、`content_hash`、`captured_at`、`captured_by_job_id`、`retention_class`、`expires_at`、`created_at`。
- 唯一约束：无硬唯一约束；同一内容可以重复采集，因为“重复观察”本身也是历史事实。
- 索引：`account_id, subject_type, subject_id, captured_at desc`；`source_system, source_ref, captured_at desc`；`content_hash`。
- 为什么存在：保存 Docker / 1Panel / FRP 的原始或规范化观测结果，供对账、回放、回滚判断和事后排障使用。

## 6. 索引与约束策略

### 6.1 业务唯一性优先级

优先级从高到低如下：

1. 上游稳定 ID（`external_id`）。
2. 稳定业务键（`canonical_key`）。
3. 运行时键（`runtime_key`）。

其中：

- `external_id` 用来对齐远端已有对象。
- `canonical_key` 用来判断是不是同一个业务对象。
- `runtime_key` 用来识别容器重建或实例替换。

### 6.2 索引设计原则

- 队列类表：围绕 `status + 时间 + 优先级` 建组合索引。
- 对账类表：围绕 `subject_type + subject_id`、`canonical_key`、`external_id` 建组合索引。
- 审计类表：围绕 `subject_type + subject_id + created_at` 和 `actor_id + created_at` 建索引。
- JSON 字段默认不建索引；只有当某个键被频繁过滤时，才把它提升为显式列。

### 6.3 软删除与唯一性

对于带 `deleted_at` 或 `archived_at` 的表：

- 唯一约束应只覆盖活动记录。
- 如果数据库支持 partial unique index，优先使用。
- 如果不支持，应用层需要在恢复 / 新建时先做冲突检查。

### 6.4 数据保留

建议的保留策略：

- `job_events`、`audit_log`：长期保留，按容量做归档，不做覆盖更新。
- `snapshots`：按保留窗口清理旧版本，但保留最近成功与最近失败样本。
- `sync_state`：只保留最新行，不保留历史版本。
- `jobs`：完成后可按时间归档，但至少保留可追溯周期内的结果和错误。

## 7. 当前设计里不额外拆表的部分

为了保持第一版 schema 够薄、够稳，以下内容先不单独拆成独立表：

- 1Panel 的域名列表、代理配置和 HTTPS 配置细项。
  - 这些内容可以先作为 `website_mappings.domains_json`、`manual_override_json` 和 `snapshots.content_json` 的一部分。
  - 如果后续需要高频按域名做查询，再拆 `website_domains` 子表。
- 远端 API 的完整原始响应。
  - 只保留对账必需字段和快照哈希，不把每个返回体都做成一个实体表。

## 8. 小结

这套 schema 的核心不是“多建几张表”，而是把三种职责分离清楚：

- `accounts` / `auth_tokens` / `upstream_credentials` 解决身份和凭据。
- `nodes` / `tunnels` / `website_mappings` 解决业务意图和外部资源投影。
- `jobs` / `job_events` / `sync_state` / `audit_log` / `snapshots` 解决执行、对账、追踪和恢复。

只要这几层分开，后续无论是接 1Panel、接 FRP，还是做增量同步和人工接管，数据边界都会稳定很多。
