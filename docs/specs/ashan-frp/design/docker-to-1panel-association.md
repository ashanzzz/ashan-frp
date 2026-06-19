# Docker ↔ 1Panel 一键关联流程设计

> 适用范围：`ashan-frp` 管理台中，将 Docker 容器自动绑定到 1Panel Website 对象并暴露为 HTTPS 站点的完整流程。
> 约束：只描述流程、决策点、失败恢复和幂等策略，不包含具体实现代码。
> 配套文档：`architecture.md`（上层目录）、`architecture-diagram.md`、`backend-schema.md`、`frontend-ui.md`、`job-event-model.md`。`

---

## 1. 设计目标

这个流程回答一个问题：用户在管理台里“点一下关联”，后台需要完成哪些有依赖顺序的操作，失败时如何恢复，冲突时怎么处理。

核心原则：

- **本地状态为事实源**：远端系统（1Panel、Docker、FRP）的响应只作为输入和对账证据，不直接作为本地状态真理。
- **意图与执行分离**：API 层只保存用户意图、创建调度任务，远端副作用由异步 job runner 执行。
- **全链路可追踪**：每一个操作都有 `job` → `job_event` → `audit_log` 的完整轨迹。
- **固定阈值优先**：所有自动化决策阈值使用固定刻度按钮 / 分段控件，禁用连续滑块。

---

## 2. 阶段定义（Phase Map）

| 阶段 | 名称 | 输入 | 输出 TMP | 责任方 |
|---|---|---|---|---|
| 1 | 容器发现 | `containers` 表（Docker API） | `container_id`、`image`、`port_bindings` | Job Runner → Docker Adapter |
| 2 | 端口解析 | `port_bindings` | 端口 / 协议列表 | Job Runner → Docker Adapter |
| 3 | 用户意图录入 | 前端表单 | `website_mappings` 表新行 | API 服务 |
| 4 | 指向生成 | `website_mappings` + 节点配置 | 域名 / 反代目标 | 同步引擎 |
| 5 | Website 创建 | `website` 定义 | 1Panel `website` 对象 | Job Runner → 1Panel Adapter |
| 6 | 代理配置 | 1Panel `website` 返回 | 代理规则、upstream | Job Runner → 1Panel Adapter |
| 7 | HTTPS 申请 | 域名 / 节点配置 | 证书、TLS 状态 | Job Runner → 1Panel Adapter |
| 8 | 持久化关联 | 所有前置输出 | `website_mappings` + `sync_state` | 同步引擎 |

每个阶段都是一次可能的失败点，因此每个阶段必须在失败后有明确的降级和重试策略。

---

## 3. 步骤序列（Step Sequence）

### 3.1 选择容器 → 绑定目标 → 一键关联

前端交互流程：

```text
[容器列表页]
  │
  ▼
点击某容器行 → 打开详情抽屉
  │
  ▼
[容器详情抽屉：端口映射 / 环境 / 状态]
  │
  ▼
点击“添加为网站映射”
  │
  ▼
[关联表单抽屉]
  - 域名（可选：系统自动分配 subdomain）
  - 目标端口（从容器 port_bindings 解析，默认选中第一个）
  - 是否启用 HTTPS（默认开启）
  - 应用类型（静态 / 反向代理 / PHP 运行环境 —— 如需适配 1Panel 必须指定）
  - 高级：自定义请求头 / 超时 / 重试 / 请求体大小限制
  │
  ▼
提交 → API 创建 website_mapping + 生成 job
  │
  ▼
[列表页自动刷新] → 显示新增映射行，状态为“排队中”
```

### 3.2 后台处理序列

当一个网站映射被创建后，由 job runner 按以下顺序执行：

| 步骤 | 动作 | 失败类型 | 处理 |
|---|---|---|---|
| 1 | 校验容器状态（Docker inspect） | 容器不存在 / 已停止 | 标记异常，可重试 |
| 2 | 解析暴露端口 | 无 ports / 端口冲突 | 标记异常，通知用户补选 |
| 3 | 校验域名可用性 | DNS 已经指向别处 | 冲突处理：或用户确认覆盖，或停止 |
| 4 | 创建 1Panel `website` 对象 | 1Panel 不可用 / 认证失败 | 指数退避重试 |
| 5 | 配置反向代理规则 | 1Panel 拒绝规则格式 | 标记失败，清理已创建对象 |
| 6 | 申请 HTTPS 证书 | 证书申请超时 / 限流 | 重试；超过阈值标记为手动处理 |
| 7 | 写入 `sync_state`，标记为“已同步” | DB 写入失败 | 事务回滚，重试 |

每个步骤都必须生成 `job_events` 记录，并在最终成功或失败后更新 `sync_state`。

---

## 4. 用户动作拆分

### 4.1 写操作（必经过 API → Job Run）

| 动作 | 前端表现 | 后端实际行为 |
|---|---|---|
| 添加网站映射 | 表单提交 → 抽屉关闭 → 列表刷新 | API 写入 `website_mappings`，创建 job `create_website_mapping`，runner 按阶段执行 |
| 编辑网站映射 | 抽屉表单 → 确认 | API 更新 `website_mappings` 列，创建 job `update_website_mapping`，runner 执行差异更新 |
| 重新应用 | 按钮点击 → 状态变为“执行中” | 创建 job `reapply_website_mapping`，runner 重新执行第 5-7 步 |
| 删除网站映射 | 确认弹窗 → 执行 | API 软删除 `website_mappings`，创建 job `remove_website_mapping`，runner 清理远端并释放域名 |
| 手动覆盖 | 详情抽屉 → 设置手动覆盖标记 | 仅更新本地 `website_mappings.manual_override`，同步引擎暂时跳过该对象 |
| 解除手动覆盖 | 详情抽屉 → 清除覆盖标记 | 同步引擎重新计算差异，按需生成 job |

### 4.2 读操作（同步返回，不创建 job）

| 动作 | 前端表现 | 后端实际行为 |
|---|---|---|
| 查看容器列表 | 页面 / 表格 | 查询 `containers` 表，根据 `last_synced_at` 决定是否需要先刷新 |
| 查看容器详情 | 抽屉 | 查询 `containers` + `snapshots` + `website_mappings` 关联记录 |
| 查看网站映射列表 | 页面 / 表格 | 查询 `website_mappings` + `sync_state` + 最近的 `snapshots` |
| 查看网站映射详情 | 抽屉 | 查询完整 `website_mappings` 行 + 关联 `job_events` + `audit_log` |

---

## 5. 幂等与重试

### 5.1 幂等策略

每个 job 必须携带 `idempotency_key`。建议格式：

```text
<action>:<account_id>:<website_mapping_id>:<attempt_count>
```

例如：

```text
create_website_mapping:a_123:wm_456:0
```

Job runner 在领取 job 时，先查询该 `idempotency_key` 是否已有成功的 sibling job。若有，直接跳过，把当前 job 标记为 `succeeded`，`result_json` 内记录“幂等跳过”。

### 5.2 重试策略

| 场景 | 最大重试次数 | 退避策略 | 达到上限后 |
|---|---|---|---|
| 1Panel API 暂时不可用 | 5 | 指数退避 5s → 40s | 标记 `blocked`，通知人工介入 |
| Docker 容器短暂停止 | 3 | 线性退避 10s | 标记 `blocked` |
| DNS 更新未生效 | 5 | 指数退避 30s → 120s | 标记 `blocked`，提示手动检查 DNS |
| 证书申请失败 | 3 | 线性退避 60s | 标记 `blocked`，提示更换证书方式 |

重试次数到达上限后，job 终态为 `blocked`，不再自动重试，必须人工解除阻塞或手动重试。

---

## 6. 冲突与覆盖

### 6.1 冲突检测

冲突判定条件（满足任一即视为冲突）：

| 冲突类型 | 触发条件 | 系统行为 |
|---|---|---|
| 域名已指向别处 | DNS A 记录 / CNAME 与当前节点不匹配 | 阻止创建，提示用户确认 |
| 同一个 1Panel 已存在同名 website | 1Panel 返回 409 / 已存在 | 合并（更新）或报错让用户选择 |
| 目标端口被占用 | 节点该端口已有其他映射 | 提示用户选择其他端口或冲突处理 |
| 手动编辑 1Panel 后状态漂移 | 同步引擎检测到 `sync_state` 与远端不一致 | 标记“手动漂移”，等待用户确认覆盖或回滚 |

### 6.2 手动覆盖

当用户手动在 1Panel 侧修改配置后，同步引擎至少会发现以下差异：

- `snapshots` 中记录的 1Panel `website` 配置与本地 `website_mappings` 定义不一致。
- `sync_state` 中 `observed_hash != desired_hash`。

处理策略：

1. 同步引擎暂停对该对象的自动修复（但不暂停监控）。
2. 前端详情页显示“手动覆盖中”状态标签。
3. 用户可选择：
   - **接受远端为事实** → 把 1Panel 当前状态回写为新的期望态。
   - **恢复本地定义** → 生成 job 重新应用本地配置，覆盖 1Panel 设置。
4. 无论用户选择哪个方向，都会在 `audit_log` 中记录操作人、时间、原因。

---

## 7. 回滚与失败恢复

### 7.1 事务补偿

每个 job 的阶段性变更必须设计补偿操作：

| 阶段 | 成功操作 | 补偿操作 |
|---|---|---|
| Website 创建 | POST /openResty/websites | DELETE /openResty/websites/{id} |
| 代理配置 | PUT /openResty/websites/{id}/config | 恢复原配置或删除网站 |
| HTTPS 申请 | POST /openResty/websites/{id}/https |  disable HTTPS，保留网站 |

### 7.2 失败恢复流程

当一个 job 失败后：

```
1. 回滚所有已成功的变更（如果可安全回滚）。
2. 把 failure_reason 和 rollback_status 写入 job_events。
3. 更新 sync_state 为 dirty（如需重试）或 error（终态）。
4. 发送 SSE 事件 job.failed 或 sync.retry_scheduled。
5. 前端通过 SSE 更新列表 / 详情状态。
```

### 7.3 清理策略

当用户删除一个网站映射时：

1. 软删除 `website_mappings` 记录（设置 `deleted_at`）。
2. 创建 job `remove_website_mapping`。
3. Runner 执行：
   - 删除 1Panel `website`（或至少禁用）。
   - 释放域名解析（如果域名是系统分配的 subdomain）。
   - 释放端口资源。
4. 更新 `sync_state` 为已删除，追加 `audit_log`。

如果删除远端失败（如 1Panel 不可访问），job 进入 `blocked` 状态，用户可手动确认“已清理远端”后解除阻塞。

---

## 8. 持久化关联

### 8.1 关联的存储表示

本地持久化层使用以下信息表示“容器与 1Panel 网站对象的关联”：

```text
website_mappings 表：
  - id                      # 本地业务 ID
  - account_id              # 所属账号
  - node_id                 # 所属上游节点
  - container_id (or local identifier)
                           # 容器标识（可以是 Docker container_id，或本地稳定标识）
  - canonical_key           # 稳定业务键：例如 <account_id>:<container_name>:<exposed_port>
  - runtime_key             # 运行时实例键：当前 Docker container_id + 当前 host + 当前端口
  - external_id             # 1Panel 侧的 website 资源 ID（如 openResty websites 的 ID）
  - domain                  # 分配的域名
  - target_port             # 容器内目标端口
  - target_protocol           # 代理协议（http / https / tcp / udp）
  - https_enabled             # 是否启用 HTTPS
  - status                  # active / disabled / error / manual_override
  - manual_override           # 用户手动接管标记
  - desired_hash              # 本地期望态哈希（用于差异检测）
  - created_at / updated_at / deleted_at
```

### 8.2 观测态存储

- `sync_state`：记录 `website_mappings` 的期望态与实际观测态的差异。
- `snapshots`：保存 1Panel `website` 对象的原始 JSON，以及 Docker 容器状态快照。
- `job_events`：保存每一个操作的轨迹。

### 8.3 关联重建

当容器重新启动、ID 变化、或映射关系丢失时，系统通过以下流程重建关联：

1. 容器发现阶段通过 `canonical_key` 判断是否“是同一个业务对象”。
2. 如果 `canonical_key` 匹配，但 `runtime_key` 不同（如容器 ID 变了），系统标记为“实例漂移”。
3. 同步引擎提示用户确认或自动更新 `runtime_key` 与 `external_id`（根据策略）。
4. 关联重建后，自动执行第 5-7 步（Website 配置 / 代理 / HTTPS 检查）。

---

## 9. 附录：数据表与字段映射

### 9.1 `website_mappings` 字段速查

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | UUID / 业务主键 | 本地唯一标识 |
| `account_id` | UUID | 所属账号 |
| `node_id` | UUID | 所属上游节点 |
| `container_id` | string | 容器标识 |
| `canonical_key` | string | 稳定业务键 |
| `runtime_key` | string | 运行时实例键 |
| `external_id` | string | 1Panel website 资源 ID |
| `domain` | string | 分配的域名 |
| `target_port` | int | 目标端口 |
| `target_protocol` | enum | http / https / tcp / udp |
| `https_enabled` | boolean | 是否启用 HTTPS |
| `status` | enum | active / disabled / error / manual_override |
| `manual_override` | boolean | 用户手动接管标记 |
| `desired_hash` | string | 期望态哈希 |
| `created_at / updated_at / deleted_at` | timestamp | 时间戳 |

### 9.2 流程与已有设计文档的关系

| 本文内容 | 对应文档 | 说明 |
|---|---|---|
| 架构分层与职责边界 | `architecture.md` 第 1-2 节 | 组件分层、模块边界 |
| 数据模型与字段定义 | `backend-schema.md` 第 4-6 节 | 表设计、索引、稳定键策略 |
| job 生命周期与事件 | `job-event-model.md` 第 4-6 节 | 状态机、事件类型、sse 频道 |
| 前端交互流程 | `frontend-ui.md` 第 5.3、5.7 节 | 页面地图、抽屉设计、操作反馈 |
| 架构图与数据流 | `architecture-diagram.md` 第 2-3 节 | 控制面 / 数据面 / 发布链路图 |

---

## 修订日志

| 日期 | 版本 | 说明 |
|---|---|---|
| 2026-06-18 | v1.0 | 重建文档，对齐 architecture / schema / frontend / job-event / diagram 五份设计基线术语 |
