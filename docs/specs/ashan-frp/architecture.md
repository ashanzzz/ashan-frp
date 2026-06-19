# ashan-frp 系统架构总览

> 适用范围：`ashan-frp` 管理台整体架构设计。
> 本文件整合所有前置研究/设计卡片结论，只纳入已验证内容，不引入未经验证的 API 或实现细节。
> 配套文档：`design/backend-schema.md`、`design/architecture-diagram.md`、`design/frontend-ui.md`、`design/job-event-model.md`、`design/docker-to-1panel-association.md`。

---

## 1. 架构总览（Architecture Overview）

`ashan-frp` 是一套面向 Unraid NAS 环境的 FRP 隧道自动化运维管理台，将原有的 shell 脚本工作流升级为具备持久化状态、异步调度、可视化管理、多上游适配的系统化平台。

### 1.1 设计原则

| 原则 | 说明 |
|------|------|
| 本地状态为事实源 | 远端系统（1Panel、Docker、FRP、DNS）的响应只作为输入和对账证据，不做为本地状态真理。 |
| 意图与执行分离 | API 层只保存用户意图、创建调度任务，远端副作用由异步 job runner 执行。 |
| 观测与期望分离 | `tunnels`/`website_mappings` 存期望态，`snapshots`/`sync_state` 存观测态。 |
| 全链路可追踪 | 每一个远端操作都有 job → event → audit_log 的完整轨迹。 |
| 固定阈值优先 | 所有自动化决策阈值使用固定刻度按钮/分段控件，禁用连续滑块。 |

### 1.2 组件分层

系统按职责分为五层：

```
┌─────────────────────────────────────────┐
│  客户端层                                │
│  ├─ 运维浏览器（管理操作、SSE 订阅）      │
│  └─ 访问者浏览器（访问公开 HTTPS 站点）    │
├─────────────────────────────────────────┤
│  管理控制平面                            │
│  ├─ 前端 UI（仪表盘、资源管理、设置）      │
│  └─ API 服务（鉴权、校验、落库、查询）     │
├─────────────────────────────────────────┤
│  状态与调度平面                          │
│  └─ 数据库（accounts / nodes / tunnels / │
│      website_mappings / jobs / sync_state /│
│      snapshots / audit_log / settings）   │
├─────────────────────────────────────────┤
│  执行平面                                │
│  ├─ Job Runner（领取、执行、重试、状态机） │
│  └─ 1Panel Adapter（协议转换、DTO 归一化不守）│
├─────────────────────────────────────────┤
│  外部运行时                              │
│  ├─ 1Panel（网站对象、代理规则、HTTPS）    │
│  ├─ 反向代理 / OpenResty                 │
│  ├─ Docker / 应用容器                     │
│  ├─ FRP 远端节点                          │
│  └─ Cloudflare DNS                       │
└─────────────────────────────────────────┘
```

---

## 2. 模块边界（Module Boundaries）

### 2.1 前端 UI

| 职责 | 不做的 |
|------|--------|
| 展示状态、发起操作、显示 job 进度 | 直接调用 1Panel API |
| 订阅 SSE 接收实时更新 | 在请求线程中执行远端副作用 |
| 抽屉/弹窗操作交互 | 保存业务状态 |

**页面地图**：概览(仪表盘) → 资源(节点/隧道/网站映射) → 运营(任务队列/日志) → 系统(设置)

### 2.2 API 服务

| 职责 | 不做的 |
|------|--------|
| 身份认证、权限校验、输入校验 | 直接调用 1Panel API |
| 对核心表提供 CRUD | 做同步决策 |
| 把用户动作转成 `jobs` 入队 | 在请求线程里执行长耗时回放 |
| 读取 `sync_state`/`snapshots`/`job_events` 作为查询视图 | 把临时执行结果当最终事实 |

### 2.3 同步引擎（内置于 Job Runner 或独立模块）

| 职责 | 不做的 |
|------|--------|
| 计算期望态与观测态差异 | 承载 HTTP 接口返回格式 |
| 维护 `canonical_key`/`runtime_key`/`external_id` 映射 | 具体 1Panel 调用细节 |
| 判断冲突、漂移、失效、人工接管 | 排队锁和重试调度的底层实现 |
| 生成或更新 `jobs`，写入 `sync_state`/`snapshots` | — |

### 2.4 Job Runner

| 职责 | 不做的 |
|------|--------|
| 领取可执行任务、加锁、跑重试 | 承载用户交互 |
| 更新 `jobs` 状态机 | 业务领域唯一性判断 |
| 追加 `job_events` | 把临时执行结果当最终事实 |
| 任务成功后刷新 `sync_state` | — |

**状态机**：`queued` → `running` → `succeeded` / `retry_wait` → `blocked` / `failed` / `canceled`

### 2.5 1Panel Adapter

| 职责 | 不做的 |
|------|--------|
| 1Panel API 的请求/响应映射 | 保存业务状态 |
| DTO 规范化（1Panel 格式 → 同步引擎可消费结构） | 决定是否覆盖人工配置 |
| 记录远端原始快照到 `snapshots` | 自己判定冲突优先级 |

### 2.6 数据持久层（数据库）

| 表分组 | 说明 |
|--------|------|
| 身份与凭据 | `accounts`、`auth_tokens`、`upstream_credentials` |
| 业务意图 | `nodes`、`tunnels`、`website_mappings`、`settings` |
| 执行与观测 | `jobs`、`job_events`、`sync_state`、`snapshots` |
| 审计 | `audit_log` |

---

## 3. 数据流 / 序列流（Data Flow / Sequence Flow）

### 3.1 控制面数据流

```
操作者 ──► 前端 UI
              │
              ▼ HTTP + SSE
            API 服务
              │
    ┌─────────┼─────────┐
    ▼         ▼         ▼
  数据库    jobs 队列    SSE 推送
    │         │
    │         ▼
    │     Job Runner
    │         │
    │         ▼
    │     1Panel Adapter
    │         │
    │         ▼
    │     1Panel API
    │         │
    └── 状态回写 ◄── 归一化 DTO ── 原始响应
```

**关键规则**：
- 同步请求只负责"把意图写进去"和"给出即时响应"。
- 需要触发远端副作用的动作，必须经过 job。
- `job_events` 记录过程，`jobs.status` 记录当前快照，`sync_state` 记录同步引擎的短期记忆。

### 3.2 发布链路：container → website → proxy → HTTPS

```
 ┌──────────┐    ┌─────────────┐    ┌──────────┐    ┌────────────┐
 │ 应用容器  │───►│ 1Panel       │───►│ 反向代理 │───►│ HTTPS 入口 │
 │ (IP:Port)│    │ Website 对象 │    │ OpenResty│    │  (443)     │
 └──────────┘    └─────────────┘    └──────────┘    └─────┬──────┘
                                                          │
                                                    ┌─────┴─────┐
                                                    │ 访问者浏览器│
                                                    └───────────┘
```

这条链路的核心含义：
- 容器只提供服务端点，不直接决定公网暴露方式。
- website 对象承载域名、证书、代理规则和站点级配置。
- proxy 承担连接接入、TLS 终止和反向转发。
- HTTPS 入口是最终可见面；配置成功 ≠ 公网可访问。

### 3.3 映射到现有 shell 工作流

| 现有命令 | 对应 job 家族 | 异步化策略 |
|----------|--------------|-----------|
| `health` | `health.check` | 周期 job，带抖动 |
| `failover` | `sync.reconcile` / `tunnel.failover` | 多步远端操作，有轨迹 |
| `fastest` | `sync.optimize` / `tunnel.optimize` | 带冷却时间和抖动 |
| `userinfo` | `account.refresh` | 同步用户信息 |
| `nodes` | `node.refresh` | 刷新候选节点列表 |

---

## 4. 部署假设（Deployment Assumptions）

### 4.1 目标环境

| 项 | 假设 |
|----|------|
| 主运行平台 | Unraid NAS (192.168.8.11) |
| 容器运行时 | Docker（现有 frpc 容器管理模式） |
| 上游控制面 | 1Panel（网站对象、代理规则、HTTPS 管理） |
| 隧道服务 | ChmlFrp / FRP |
| DNS 服务 | Cloudflare DNS |
| 认证系统 | QZhua OAuth2 (access_token / refresh_token) |

### 4.2 部署弹性

本文档定义的是逻辑边界而非物理部署方式。以下模块可同进程、同容器或完全拆分：

- **前端 UI**：可独立部署为静态站点，也可与 API 服务同进程。
- **API 服务 + Job Runner**：可同进程（简化部署）或拆分（水平扩展 runner）。
- **1Panel Adapter**：可内嵌于 runner，也可作为 sidecar 独立。
- **数据库**：优先 SQLite（单节点 / 试验阶段），后续可迁移至 PostgreSQL。

### 4.3 关键外部依赖

| 依赖 | 假设 |
|------|------|
| 1Panel | 已安装且 OpenResty 实例存在；非 443 HTTPS 端口可通过 `AppInstall.HttpsPort` 链路获取 |
| Docker | 容器端口映射可通过 `ContainerHelper.exposedPorts` 结构化获取 |
| ChmlFrp API | 支持 OAuth2 认证；节点列表、隧道管理接口稳定 |
| Cloudflare DNS | 提供 API Token 和 Zone ID |
| DNS 解析 | `primary_domain` 指向正确的公网 IP（需人工确认） |

---

## 5. 风险 / 权衡 / 开放问题（Risks / Trade-offs / Open Questions）

### 5.1 高风险项

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| **1Panel HTTPS 非 443 端口** | HTTP/3 Alt-Svc 硬编码 `:443` 指向错误端口 | 文档标记为高风险；需人工确认或等待上游修复 |
| **OAuth2 refresh_token 失效** | 自动化完全中断，需人工重新授权 | 提前告警 + 降级提示；refresh 自动保存新 token |
| **1Panel API 版本漂移** | adapter DTO 映射失效 | adapter 层独立，变更只影响映射层；快照保留原始响应对账 |
| **容器重建后 runtime_key 变化** | `canonica_key` ↔ `runtime_key` 映射断裂 | 稳定业务键使用 containerName+containerPort+protocol；运行时键含 containerID+hostIP+hostPort |
| **单节点数据库 QPS 瓶颈** | 高并发同步时排队延迟 | 早期 SQLite 足够；可迁移 PostgreSQL；job runner 是主要负载而非 API |

### 5.2 设计权衡

| 权衡点 | 选择 A | 选择 B | 当前选择 | 理由 |
|--------|--------|--------|----------|------|
| 同步 vs 异步 | 所有操作同步返回 | 远端副作用全部异步化 | **异步化** | 避免请求线程阻塞；支持重试和轨迹 |
| 状态存储 | 远端系统作为真理源 | 本地状态为真理源 | **本地为源** | 远端不可靠（节点可能离线）；本地可审计可回滚 |
| 前端实时 | 纯轮询 | SSE 为主，polling 降级 | **SSE 为主** | 更接近实时；Job 状态变化即时可见 |
| 阈值输入 | 连续滑块 | 固定刻度按钮/分段控件 | **固定刻度** | 避免边界值模糊；利于审计和协作复述 |
| 部署复杂度 | 微服务拆分 | 单体集中 | **逻辑分层、物理可集中** | Unraid 单节点环境；先跑通再拆分 |

### 5.3 开放问题（待后续卡片解决）

| # | 问题 | 状态 |
|---|------|------|
| 1 | 是否支持多 1Panel 实例管理？ | 设计预留 `nodes.provider` + `upstream_credentials.provider` 字段，但交互细节未展开 |
| 2 | HTTPS 证书自动化策略（Let's Encrypt / 自定义证书）？ | `website_mappings.ssl_certificate_ref` 预留，具体集成待设计 |
| 3 | FRP 节点健康探测的精确判定逻辑？ | 依赖 ChmlFrp API 稳定性；需收集更多运行时数据 |
| 4 | 前端技术栈选型？ | 设计文档已完成交互规范，技术实现未限定 |
| 5 | 数据库迁移策略（SQLite → PostgreSQL）？ | 早期用 SQLite；结构化 query 避免 SQLite 特有语法 |

---

## 6. 修订日志（Revision Log）

| 版本 | 日期 | 作者 | 变更说明 |
|------|------|------|---------|
| 0.1 | 2026-06-18 | writer | 初始版本：整合 9 张前置卡片结论，形成统一架构总览 |  |

---

## A. 附录：与前置文档的对应关系

| 前置卡片 | 文档 | 本文件引用章节 |
|----------|------|--------------|
| t_021a7d2b | `research/1panel-https-port.md` | 3.2 发布链路（HTTPS 端口风险） |
| t_17b03637 | `design/frontend-ui.md` | 1.2 组件分层 / 2.1 前端 UI |
| t_57854c0f | `design/docker-to-1panel-association.md` | 3.2 发布链路（container→website） |
| t_9f78b197 | `design/backend-schema.md` | 2.6 数据持久层 / 所有表设计 |
| t_ae707529 | `research/chmlfrp-domain-model.md` | 3.3 现有 shell 工作流映射 |
| t_bf83a3bd | `design/architecture-diagram.md` | 1.1 架构总览 / 3.1 数据流 |
| t_cd1b759b | `design/job-event-model.md` | 2.4 Job Runner / 3.1 控制面数据流 |
| t_eaceff72 | `research/1panel-website-api.md` | 3.2 发布链路（website API） |
| t_ebb6b35c | `research/docker-port-mapping.md` | 3.2 发布链路（container 端口） |
