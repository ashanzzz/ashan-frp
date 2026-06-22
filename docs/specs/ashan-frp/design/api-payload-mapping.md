# Ashan FRP 字段到 API 请求体映射稿

> 适用范围：`ashan-frp` 管理台的资源表单、列表筛选、动作按钮、SSE 订阅与 OpenAPI 合同。
> 目标：把 `form-layout-draft.md` 里的字段顺序、`wireframe-draft.md` 的页面骨架，以及 `job-event-model.md` 的异步语义，统一落成一套可直接给前后端共同实现的 API 请求体 / 响应体映射稿。
> 当前状态说明：本文显式区分 **当前仓库现状** 与 **目标 API 合同**。当前仓库中的 Go/TS 代码仍是骨架，不应被误认为本文档里的目标接口已经存在。
> 配套文档：`frontend-ui.md`、`form-layout-draft.md`、`wireframe-draft.md`、`backend-schema.md`、`job-event-model.md`、`docker-to-1panel-association.md`、`code-structure-architecture.md`。

---

## 1. 当前状态 vs 目标状态

### 1.1 当前仓库里实际能看到的 API 现状

基于当前仓库代码，已观察到：

- `frp-backend/internal/server/server.go` 已提供 `/api/v1/*`、`/api/docs`、`/api/openapi.json`、`/api/v1/events/stream` 与 `/ui/`。
- 旧的 flat demo 接口前缀（`/api/version`、`/api/tunnels`、`/api/chmlfrp/nodes`、`/api/frpc/runtime/info`）已经不再是当前主线。
- 早期 UI 原型曾假设存在旧式 `/api/sse/tunnels` 与 `/api/tunnels/:name/start|stop`，现在这些语义已经收敛为 `/api/v1` 合同。
- 现有接口使用统一 envelope，并带有 `request_id` / `trace_id` / `job` 等异步控制面字段。

### 1.2 本文要定义的目标状态

本文目标不是继续补零散接口，而是把管理面 API 直接收敛成：

1. **统一版本前缀**：全部收口到 `/api/v1`
2. **统一响应 envelope**：同步 / 异步都可机读
3. **表单字段到 DTO 一一映射**
4. **按钮动作与 job 创建语义明确化**
5. **SSE / OpenAPI / Swagger 对齐**

---

## 2. API 合同总原则

### 2.1 路径原则

| 类型 | 路径约定 |
|---|---|
| 资源列表 / 详情 | `/api/v1/<resources>` |
| 资源动作 | `/api/v1/<resources>/{id}/actions/<action>` |
| 实时事件 | `/api/v1/events/stream` |
| OpenAPI | `/api/openapi.json` |
| Swagger UI | `/api/docs` |

### 2.2 响应 envelope

#### 同步成功

```json
{
  "data": {},
  "meta": {
    "request_id": "req_01...",
    "trace_id": "trc_01..."
  }
}
```

#### 异步成功（创建了 job）

```json
{
  "data": {},
  "meta": {
    "request_id": "req_01...",
    "trace_id": "trc_01...",
    "job": {
      "id": "job_01...",
      "status": "queued",
      "channel": "subject:tunnel:tun_01..."
    }
  }
}
```

#### 错误响应

```json
{
  "error": {
    "code": "TUNNEL_PORT_CONFLICT",
    "message": "目标远端端口已被占用",
    "retryable": false,
    "details": {
      "remote_port": 6001
    }
  },
  "meta": {
    "request_id": "req_01...",
    "trace_id": "trc_01..."
  }
}
```

### 2.3 ID 与命名原则

- 目标设计中资源 ID 使用 **UUID / ULID 字符串**，而不是当前 demo 模型的 `uint`。
- 表单字段名面向业务，但请求体字段名尽量和 schema 列名对齐。
- 旧 flat `FRPTunnel` / `OnePanelConfig` demo model 不再作为外部 API 合同。

### 2.4 同步 / 异步边界

| 动作 | 是否异步 |
|---|---|
| 列表 / 详情查询 | 否 |
| 纯本地 settings 保存 | 否或批量同步保存 |
| 会触发 1Panel / FRP / DNS / frpc 副作用的操作 | 是 |
| 重新应用 / 启停 / 重载 / 同步 / 重建 | 是 |

---

## 3. 列表筛选条到 Query 参数映射

### 3.1 节点列表

| UI 字段 | Query 参数 | 类型 | 说明 |
|---|---|---|---|
| 关键词搜索 | `q` | string | 匹配 `display_name` / `canonical_name` / `external_id` |
| provider | `provider` | string | 如 `chmlfrp` / `1panel` / `cloudflare` |
| 状态 | `status` | string | `active` / `disabled` / `archived` |
| 健康状态 | `health_status` | string | `online` / `degraded` / `offline` / `banned` |
| 归档开关 | `include_archived` | boolean | 默认 `false` |

### 3.2 隧道列表

| UI 字段 | Query 参数 | 类型 |
|---|---|---|
| 关键词搜索 | `q` | string |
| 节点 | `node_id` | string |
| 期望状态 | `desired_state` | string |
| 差异状态 | `diff_status` | string |
| 手动覆盖开关 | `manual_override` | boolean |

### 3.3 网站映射列表

| UI 字段 | Query 参数 | 类型 |
|---|---|---|
| 域名搜索 | `q` | string |
| 节点 | `node_id` | string |
| HTTPS 状态 | `https_enabled` | boolean |
| 同步状态 | `status` | string |
| 归档开关 | `include_archived` | boolean |

### 3.4 任务队列

| UI 字段 | Query 参数 | 类型 |
|---|---|---|
| 状态 | `status` | string |
| job type | `job_type` | string |
| target type | `target_type` | string |
| target id | `target_id` | string |
| 锁定状态 | `locked` | boolean |
| 时间范围 | `from` / `to` | ISO8601 string |

### 3.5 日志页

| UI 字段 | Query 参数 | 类型 |
|---|---|---|
| 来源 | `source` | string |
| 等级 | `level` | string |
| target type | `target_type` | string |
| target id | `target_id` | string |
| 时间范围 | `from` / `to` | ISO8601 string |
| 关键词 | `q` | string |

---

## 4. 节点表单 → API 请求体映射

### 4.1 DTO：`NodeUpsertRequest`

```json
{
  "display_name": "香港 BGP 极速",
  "provider": "chmlfrp",
  "node_type": "frp_node",
  "endpoint_url": "https://node.example.com",
  "region": "hongkong",
  "status": "active",
  "canonical_name": "hk-bgp-fast",
  "metadata": {
    "location": "HK",
    "tags": ["intl", "bgp"]
  }
}
```

### 4.2 字段映射表

| 表单字段 | 请求体 key | 类型 | 必填 | 备注 |
|---|---|---:|---|---|
| `display_name` | `display_name` | string | 是 | 用户主输入名称 |
| `provider` | `provider` | string | 是 | 上游提供方 |
| `node_type` | `node_type` | string | 是 | 如 `frp_node` / `panel_instance` |
| `endpoint_url` | `endpoint_url` | string | 否 | 某些节点类型必填 |
| `region` | `region` | string | 否 | 用于 UI 展示与策略选择 |
| `status` | `status` | string | 是 | `active` / `disabled` |
| `canonical_name` | `canonical_name` | 否 | 否 | 高级区字段 |
| `metadata_json` | `metadata` | object | 否 | 前端不传 raw json 名称，统一叫 `metadata` |

### 4.3 端点

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/api/v1/nodes` | 列表 |
| `POST` | `/api/v1/nodes` | 新建节点 |
| `GET` | `/api/v1/nodes/{id}` | 详情 |
| `PATCH` | `/api/v1/nodes/{id}` | 更新节点 |
| `POST` | `/api/v1/nodes/{id}/actions/check` | 立即检查，异步 job |
| `POST` | `/api/v1/nodes/{id}/actions/archive` | 归档，通常同步更新本地状态 |

### 4.4 返回说明

- `POST /nodes`、`PATCH /nodes/{id}` 默认返回当前节点快照。
- `actions/check` 返回 `job_id`，前端通过 SSE 或轮询 job 查看结果。

---

## 5. 隧道表单 → API 请求体映射

### 5.1 DTO：`TunnelUpsertRequest`

```json
{
  "node_id": "node_01...",
  "name": "npm-registry",
  "tunnel_type": "http",
  "desired_state": "enabled",
  "local_ip": "127.0.0.1",
  "local_port": 3000,
  "remote_port": 0,
  "dns_domain_cname": "npm.example.com",
  "dns_proxied": true
}
```

### 5.2 字段映射表

| 表单字段 | 请求体 key | 类型 | 必填 | 备注 |
|---|---|---:|---|---|
| `node_id` | `node_id` | string | 是 | 绑定节点 |
| `name` | `name` | string | 是 | 业务主键的一部分 |
| `tunnel_type` | `tunnel_type` | string | 是 | `http` / `https` / `tcp` / `udp` |
| `desired_state` | `desired_state` | string | 是 | `enabled` / `disabled` |
| `local_ip` | `local_ip` | string | 是 | 本地服务地址 |
| `local_port` | `local_port` | integer | 是 | 本地端口 |
| `remote_port` | `remote_port` | integer | 否 | TCP/UDP 常用 |
| `dns_domain_cname` | `dns_domain_cname` | string | 否 | HTTP/HTTPS 常用 |
| `dns_proxied` | `dns_proxied` | boolean | 否 | 仅域名模式适用 |

### 5.3 明确不进入主请求体的字段

这些字段只出现在详情只读区，不应作为前端主表单提交项：

- `actual_state`
- `state_reason`
- `last_applied_at`
- `last_error_code`
- `last_error_message`
- `runtime_key`
- `desired_hash`
- `observed_hash`

### 5.4 端点

| 方法 | 路径 | 说明 | 是否异步 |
|---|---|---|---|
| `GET` | `/api/v1/tunnels` | 列表 | 否 |
| `POST` | `/api/v1/tunnels` | 新建隧道 | 否（只落本地） |
| `GET` | `/api/v1/tunnels/{id}` | 详情 | 否 |
| `PATCH` | `/api/v1/tunnels/{id}` | 更新隧道定义 | 否（只落本地） |
| `POST` | `/api/v1/tunnels/{id}/actions/apply` | 立即应用到远端 / frpc | 是 |
| `POST` | `/api/v1/tunnels/{id}/actions/start` | 启用运行 | 是 |
| `POST` | `/api/v1/tunnels/{id}/actions/stop` | 停用运行 | 是 |
| `POST` | `/api/v1/tunnels/{id}/actions/recreate` | 重建 / 对账修复 | 是 |
| `POST` | `/api/v1/tunnels/{id}/actions/archive` | 归档 | 视实现而定 |

### 5.5 `保存并排队应用` 的响应

```json
{
  "data": {
    "id": "tun_01...",
    "name": "npm-registry",
    "desired_state": "enabled"
  },
  "meta": {
    "request_id": "req_01...",
    "trace_id": "trc_01...",
    "job": {
      "id": "job_01...",
      "status": "queued",
      "channel": "subject:tunnel:tun_01..."
    }
  }
}
```

---

## 6. 网站映射表单 → API 请求体映射

### 6.1 DTO：`WebsiteMappingUpsertRequest`

```json
{
  "source_kind": "tunnel",
  "node_id": "node_01...",
  "tunnel_id": "tun_01...",
  "source_external_id": "",
  "website_alias": "npm-site",
  "primary_domain": "npm.example.com",
  "domains": [
    "npm.example.com",
    "registry.example.com"
  ],
  "https_enabled": true,
  "certificate": {
    "mode": "auto",
    "ssl_certificate_ref": ""
  },
  "proxy": {
    "enabled": true,
    "cache_enabled": false,
    "target_mode": "derived",
    "target": "http://127.0.0.1:3000"
  },
  "http_config": {
    "headers": [],
    "body_size_limit": "20m",
    "read_timeout": "30s"
  },
  "conflict_strategy": "pause_on_conflict"
}
```

### 6.2 为什么 `domains_json` 不直接映射成同名字段

前端设计里明确不建议让用户编辑 raw JSON，因此目标 API 合同中应把：

- UI 输入：`primary_domain` + 备用域名数组
- API 请求：`domains: string[]`
- 持久化层：后端再序列化为 `domains_json`

### 6.3 字段映射表

| 表单字段 | 请求体 key | 类型 | 必填 | 备注 |
|---|---|---:|---|---|
| `source_kind` | `source_kind` | string | 是 | `tunnel` / `container` / `custom` |
| `node_id` | `node_id` | string | 是 | 所属节点 |
| `tunnel_id` | `tunnel_id` | string | 条件必填 | `source_kind=tunnel` 时必填 |
| `source_external_id` | `source_external_id` | string | 条件必填 | `source_kind=container/custom` 时使用 |
| `website_alias` | `website_alias` | string | 否 | 本地别名 |
| `primary_domain` | `primary_domain` | string | 是 | 主域名 |
| `domains_json` | `domains` | array[string] | 是 | 结构化数组，不传 raw JSON |
| `https_enabled` | `https_enabled` | boolean | 是 | 是否启用 HTTPS |
| `ssl_certificate_ref` | `certificate.ssl_certificate_ref` | string | 否 | 已有证书引用 |
| 证书来源 | `certificate.mode` | string | 是 | `auto` / `existing` / `manual` |
| `proxy_enabled` | `proxy.enabled` | boolean | 是 | 是否启用代理 |
| `proxy_cache_enabled` | `proxy.cache_enabled` | boolean | 否 | 是否启用缓存 |
| `proxy_target` | `proxy.target` | string | 条件必填 | `source_kind=custom` 时允许显式传入 |
| `http_config` | `http_config` | object | 否 | 高级区参数 |
| 冲突处理策略 | `conflict_strategy` | string | 否 | UI 逻辑字段 |

### 6.4 明确不进入主请求体的只读字段

- `status`
- `panel_website_id`
- `last_synced_at`
- `last_error_code`
- `last_error_message`
- `runtime_key`
- `last_remote_hash`

### 6.5 端点

| 方法 | 路径 | 说明 | 是否异步 |
|---|---|---|---|
| `GET` | `/api/v1/website-mappings` | 列表 | 否 |
| `POST` | `/api/v1/website-mappings` | 新建映射意图 | 否 |
| `GET` | `/api/v1/website-mappings/{id}` | 详情 | 否 |
| `PATCH` | `/api/v1/website-mappings/{id}` | 更新映射意图 | 否 |
| `POST` | `/api/v1/website-mappings/{id}/actions/sync` | 创建 / 更新远端网站对象 | 是 |
| `POST` | `/api/v1/website-mappings/{id}/actions/reapply` | 重新应用 | 是 |
| `POST` | `/api/v1/website-mappings/{id}/actions/enable-https` | 启用 HTTPS | 是 |
| `POST` | `/api/v1/website-mappings/{id}/actions/disable-https` | 关闭 HTTPS | 是 |
| `POST` | `/api/v1/website-mappings/{id}/actions/accept-remote` | 接受远端为事实 | 是 |
| `POST` | `/api/v1/website-mappings/{id}/actions/restore-local` | 恢复本地定义 | 是 |
| `POST` | `/api/v1/website-mappings/{id}/actions/archive` | 归档 | 视实现而定 |

---

## 7. 设置页 → API 请求体映射

### 7.1 设计取舍

设置页 UI 是“多卡片 + 固定保存条”，因此最稳的合同不是一堆零散 key-value POST，而是：

- 页面保存时，使用 **批量 patch DTO**
- 凭据验证 / 重新授权 / 吊销，使用 **动作接口**

### 7.2 DTO：`SettingsBatchPatchRequest`

```json
{
  "general": {
    "default_log_lines": 100,
    "data_retention_days": 30,
    "default_refresh_mode": "polling"
  },
  "sync": {
    "healthcheck_interval": "1m",
    "sync_poll_interval": "10s",
    "diff_strategy": "pause_on_conflict",
    "manual_override_priority": "manual_wins"
  },
  "queue": {
    "max_attempts": 5,
    "retry_backoff": "30s",
    "stalled_job_policy": "mark_blocked",
    "archive_retention_days": 30
  },
  "frpc_runtime": {
    "frpc_enabled": true,
    "frpc_binary_source": "embedded",
    "frpc_binary_version": "0.54.0",
    "frpc_log_level": "info",
    "frpc_healthcheck_interval": "30s",
    "frpc_restart_backoff": "30s",
    "auto_recover_strategy": "reload_then_restart",
    "switch_node_strategy": "prefer_healthy_low_load"
  }
}
```

### 7.3 卡片字段映射

#### A. 通用控制
| UI 字段 | 请求体 key |
|---|---|
| 默认数据刷新显示方式 | `general.default_refresh_mode` |
| 日志默认显示条数 | `general.default_log_lines` |
| 数据保留期 | `general.data_retention_days` |

#### B. 同步策略
| UI 字段 | 请求体 key |
|---|---|
| 健康检查间隔 | `sync.healthcheck_interval` |
| 同步轮询间隔 | `sync.sync_poll_interval` |
| 差异处理策略 | `sync.diff_strategy` |
| 手动覆盖优先级 | `sync.manual_override_priority` |

#### C. 队列 / 重试
| UI 字段 | 请求体 key |
|---|---|
| 最大重试次数 | `queue.max_attempts` |
| 重试退避 | `queue.retry_backoff` |
| stalled job 自动处理策略 | `queue.stalled_job_policy` |
| 归档保留策略 | `queue.archive_retention_days` |

#### D. FRPC Runtime
| UI 字段 | 请求体 key |
|---|---|
| `frpc_enabled` | `frpc_runtime.frpc_enabled` |
| `frpc_binary_source` | `frpc_runtime.frpc_binary_source` |
| `frpc_binary_version` | `frpc_runtime.frpc_binary_version` |
| `frpc_log_level` | `frpc_runtime.frpc_log_level` |
| `frpc_healthcheck_interval` | `frpc_runtime.frpc_healthcheck_interval` |
| `frpc_restart_backoff` | `frpc_runtime.frpc_restart_backoff` |
| 自动恢复策略 | `frpc_runtime.auto_recover_strategy` |
| 切换节点策略 | `frpc_runtime.switch_node_strategy` |

### 7.4 凭据与授权动作接口

| 动作 | 方法 | 路径 | 请求体 |
|---|---|---|---|
| 保存 / 更新 ChmlFrp 凭据 | `PUT` | `/api/v1/integrations/chmlfrp/credentials` | `{ "name": "default", "username": "...", "password": "..." }` |
| 验证 ChmlFrp 凭据 | `POST` | `/api/v1/integrations/chmlfrp/credentials/verify` | `{ "name": "default" }` |
| 触发 ChmlFrp 登录 / 刷新 token | `POST` | `/api/v1/integrations/chmlfrp/auth/login` | `{ "name": "default" }` |
| 保存 / 更新 1Panel 凭据 | `PUT` | `/api/v1/integrations/onepanel/credentials` | `{ "name": "default", "base_url": "...", "entrance": "...", "api_token": "..." }` |
| 验证 1Panel 凭据 | `POST` | `/api/v1/integrations/onepanel/credentials/verify` | `{ "name": "default" }` |
| 保存 / 更新 Cloudflare 凭据 | `PUT` | `/api/v1/integrations/cloudflare/credentials` | `{ "name": "default", "api_token": "...", "zone_id": "..." }` |
| 验证 Cloudflare 凭据 | `POST` | `/api/v1/integrations/cloudflare/credentials/verify` | `{ "name": "default" }` |

> 凭据读取接口只返回掩码摘要、状态、最近验证时间、最近错误；不返回明文 secret。

---

## 8. frpc Runtime / 任务 / 日志 API

### 8.1 frpc Runtime

结合 `frpc-runtime.md`，目标合同：

| 方法 | 路径 | 说明 | 是否异步 |
|---|---|---|---|
| `GET` | `/api/v1/frpc/runtime` | 当前运行态摘要 | 否 |
| `POST` | `/api/v1/frpc/runtime/actions/start` | 启动 frpc | 是 |
| `POST` | `/api/v1/frpc/runtime/actions/stop` | 停止 frpc | 是 |
| `POST` | `/api/v1/frpc/runtime/actions/restart` | 重启 frpc | 是 |
| `POST` | `/api/v1/frpc/runtime/actions/reload` | 重载配置 | 是 |
| `POST` | `/api/v1/frpc/runtime/actions/switch-node` | 切换节点 | 是 |
| `GET` | `/api/v1/frpc/runtime/logs` | 读取 frpc 日志 | 否 |
| `GET` | `/api/v1/frpc/runtime/config` | 读取当前渲染配置 | 否 |

### 8.2 任务队列

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/api/v1/jobs` | 列表 |
| `GET` | `/api/v1/jobs/{id}` | 详情 |
| `POST` | `/api/v1/jobs/{id}/actions/retry` | 立即重试 |
| `POST` | `/api/v1/jobs/{id}/actions/cancel` | 取消 |
| `POST` | `/api/v1/jobs/{id}/actions/run-now` | 提前执行 |
| `POST` | `/api/v1/jobs/{id}/actions/unlock` | 解除卡住任务 |

### 8.3 日志与原始快照

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/api/v1/logs/job-events` | 作业日志 |
| `GET` | `/api/v1/logs/audit` | 审计日志 |
| `GET` | `/api/v1/logs/sync-trace` | 同步轨迹 |
| `GET` | `/api/v1/logs/snapshots` | 原始快照列表 |
| `GET` | `/api/v1/logs/snapshots/{id}` | 单个原始快照 |

---

## 9. SSE / 实时订阅映射

### 9.1 当前问题

当前前端只有 `useSSE('/api/sse/tunnels')` 这种单点式假设，和 `job-event-model.md` 的 channel / cursor 模型不一致。

### 9.2 目标接口

统一为：

`GET /api/v1/events/stream?channel=<channel>&cursor=<cursor>`

### 9.3 推荐 channel

| 页面 / 场景 | channel |
|---|---|
| 全局管理台状态 | `account:current` |
| 单个节点详情 | `subject:node:{id}` |
| 单个隧道详情 | `subject:tunnel:{id}` |
| 单个网站映射详情 | `subject:website_mapping:{id}` |
| 任务队列 | `jobs:account:current` |
| frpc runtime | `runtime:frpc` |

### 9.4 事件 payload 最低要求

- `schema_version`
- `channel`
- `kind`
- `cursor`
- `level`
- `message`
- `job`
- `subject`
- `payload`
- `error`
- `trace_id`
- `created_at`

这与 `job-event-model.md` 对齐，不再保留当前前端那种“直接推数组”的简化协议。

---

## 10. OpenAPI / Swagger 落地要求

因为用户明确要求 API-first 管理面，不额外做独立控制页，因此：

- OpenAPI JSON 必须稳定暴露在 `/api/openapi.json`
- Swagger UI 必须稳定暴露在 `/api/docs`
- 所有资源 DTO、错误码、动作接口、SSE 订阅入口都必须进 OpenAPI 文档

### 10.1 OpenAPI 中必须明确的 schema

- `NodeUpsertRequest`
- `TunnelUpsertRequest`
- `WebsiteMappingUpsertRequest`
- `SettingsBatchPatchRequest`
- `CredentialUpsertRequest`（各 provider 分支）
- `JobSummary`
- `ApiEnvelope<T>`
- `ApiError`

---

## 11. 与代码结构设计的衔接

这份映射稿要求后端代码结构必须支持三层分离：

1. **HTTP Request DTO**：面向 API
2. **Application Command / Query**：面向业务用例
3. **Persistence Model / Snapshot Model**：面向数据库和外部状态

也就是说：

- 不允许直接把 GORM model 当 API request body
- 不允许直接把 raw JSON 字段暴露给表单
- 不允许继续用 flat `package main` 的 demo 结构承载成熟合同

因此本文必须与接下来的代码架构设计稿配套使用。

---

## 12. 结论

这份文档完成后，Ashan FRP 的设计链路已经从：

- 页面地图
- 字段顺序
- 线框图

继续推进到了：

- **字段到请求体的明确映射**
- **按钮动作到异步 job 的明确映射**
- **筛选条到 query 参数的明确映射**
- **SSE / Swagger / OpenAPI 的统一入口设计**

下一步如果进入实现，就应该按这份合同反推：

1. Go 请求 DTO
2. 前端表单 state
3. OpenAPI schema
4. handler / service / job command 的代码边界
