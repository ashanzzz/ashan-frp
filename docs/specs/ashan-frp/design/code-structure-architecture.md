# Ashan FRP 代码结构与整体项目架构设计

> 适用范围：`ashan-frp` 后端代码结构、模块边界、前后端协作方式、构建与部署形态。
> 目标：把 `api-payload-mapping.md` 里的合同，推进成可实现、可拆分、可维护的代码架构设计；同时给后续落实提供明确的目录、包边界、依赖方向和阶段切片。
> 当前状态说明：本文明确区分 **当前仓库现状** 与 **目标代码结构**。当前仓库已切到 Go 主线并内嵌 UI，但目标代码边界（更清晰的 application / repository / integration / worker 分层）仍需继续收敛，本文定义的是这套成熟目标。
> 配套文档：`architecture.md`、`frontend-ui.md`、`form-layout-draft.md`、`wireframe-draft.md`、`api-payload-mapping.md`、`backend-schema.md`、`job-event-model.md`、`frpc-runtime.md`。

---

## 1. 当前状态 vs 目标状态

### 1.1 当前仓库可见状态

当前仓库里能看到的实现，已经切到 Go 主线，UI 也已改为 Go 二进制内嵌；当前主要残留是历史设计文档，不再是独立前端运行时工程：

- `frp-backend/cmd/ashan-frp/main.go`：Go 入口，负责启动、配置和优雅退出。
- `frp-backend/internal/server/`：HTTP API、SSE、OpenAPI、静态 UI 入口。
- `frp-backend/internal/store/`：当前仍是 JSON state store，后续可再切 SQLite/GORM。
- `frp-backend/internal/runtime/frpc/`：内存态 runtime manager，作为第一阶段可用切片。
- 旧 flat demo 文件 `main.go` / `handlers.go` / `models.go` / `controller.go` / `chmlfrp.go` / `onepanel.go` / `auth_watchdog.go` 已删除。

### 1.2 目标状态

目标不是“把现有 flat package 修一修”，而是收敛成一个**成熟的模块化单体**：

- **单一 Go 二进制**负责控制面、job runner、frpc runtime manager、SSE、OpenAPI、静态 UI 托管。
- **UI 以内嵌静态资源方式运行**，不再依赖独立前端构建链。
- **所有资源操作都通过明确的 DTO / Command / Query / Repository / Adapter 分层**。
- **OpenAPI / Swagger 作为正式管理合同**，不是附属说明。
- **不保留兼容层**：旧 flat 结构只作为迁移起点，不能长期并存。

---

## 2. 架构决策

### 2.1 架构风格

选择：**模块化单体（modular monolith）**。

原因：

- 代码规模目前适合单体，但必须提前做好边界。
- `job runner`、`frpc runtime manager`、`1Panel adapter`、`SSE broker` 都是天然的独立模块。
- Unraid 单节点场景更适合单进程、单二进制、单部署闭环。
- 后续如果要拆服务，模块边界已经预留好。

### 2.2 运行形态

- 一个 Go 二进制。
- 一个 HTTP API。
- 一个 job runner。
- 一个 frpc runtime manager。
- 一个本地数据库。
- 一个嵌入式前端静态站点。

### 2.3 前端/后端协作方式

- 前端只负责提交意图、显示状态、订阅事件。
- 后端以 API / SSE / OpenAPI 作为正式合同。
- 复杂动作通过 `job_id` + SSE 反馈，不做“点一下就静默完成”的假同步。

### 2.4 技术栈选择

沿用当前项目语义，目标栈保持轻量：

- **Go 1.22+**
- **net/http ServeMux**：HTTP 路由
- **JSON state store**：当前落盘实现，后续可迁移数据库
- **go:embed**：嵌入前端静态资源
- **HTML + CSS + JavaScript**：浏览器端交互
- **OpenAPI + Swagger UI**：API 合同与文档

> 说明：如果未来真的需要更重的前端框架，可以另起独立工程；但当前运行态和发布链路里，前端就是静态 HTML/CSS/JS，不走 React/Vite 构建链。

---

## 3. 目标仓库结构

### 3.1 总体目录树

```text
ashan-frp/
├── frp-backend/
│   ├── go.mod
│   ├── cmd/
│   │   └── ashan-frp/
│   │       └── main.go
│   ├── internal/
│   │   ├── bootstrap/
│   │   │   ├── app.go
│   │   │   └── wiring.go
│   │   ├── config/
│   │   │   ├── config.go
│   │   │   └── defaults.go
│   │   ├── domain/
│   │   │   ├── account/
│   │   │   ├── node/
│   │   │   ├── tunnel/
│   │   │   ├── websitemapping/
│   │   │   ├── job/
│   │   │   ├── event/
│   │   │   ├── setting/
│   │   │   └── runtime/
│   │   ├── application/
│   │   │   ├── node/
│   │   │   ├── tunnel/
│   │   │   ├── websitemapping/
│   │   │   ├── settings/
│   │   │   ├── job/
│   │   │   └── runtime/
│   │   ├── repository/
│   │   │   ├── gorm/
│   │   │   └── query/
│   │   ├── integration/
│   │   │   ├── chmlfrp/
│   │   │   ├── onepanel/
│   │   │   └── cloudflare/
│   │   ├── runtime/
│   │   │   └── frpc/
│   │   ├── worker/
│   │   │   ├── runner.go
│   │   │   ├── lease.go
│   │   │   └── scheduler.go
│   │   ├── http/
│   │   │   ├── router.go
│   │   │   ├── middleware/
│   │   │   ├── handlers/
│   │   │   ├── dto/
│   │   │   ├── response/
│   │   │   ├── sse/
│   │   │   └── openapi/
│   │   ├── events/
│   │   │   ├── broker.go
│   │   │   └── envelope.go
│   │   ├── audit/
│   │   │   └── logger.go
│   │   ├── observability/
│   │   │   ├── logging.go
│   │   │   └── metrics.go
│   │   └── web/
│   │       ├── embed.go
│   │       └── dist/
│   ├── data/
│   │   └── ashan-frp.db
│   └── README.md
├── docs/
│   └── specs/
│       └── ashan-frp/
│           └── design/
│               ├── frontend-ui.md
│               ├── form-layout-draft.md
│               ├── wireframe-draft.md
│               ├── wireframe-draft.excalidraw
│               ├── api-payload-mapping.md
│               └── code-structure-architecture.md
└── .github/
    └── workflows/
        └── build-push.yml
```

### 3.2 结构原则

- `cmd/` 只做入口，不写业务。
- `internal/bootstrap/` 做依赖装配。
- `internal/http/` 只负责协议层。
- `internal/application/` 只负责用例，不直接写 SQL。
- `internal/domain/` 只放领域规则和稳定类型。
- `internal/repository/` 只负责持久化实现。
- `internal/integration/` 只负责外部系统适配。
- `internal/worker/` 只负责 job 生命周期和调度。
- `internal/runtime/frpc/` 只负责进程托管与配置渲染。
- `internal/web/` 只负责嵌入静态文件，不承载页面逻辑。

---

## 4. 分层架构与依赖方向

### 4.1 允许的依赖方向

```text
HTTP handler -> Application service -> Domain -> Repository / Integration
HTTP handler -> Response / DTO
Worker -> Application service -> Domain -> Repository / Integration
Runtime manager -> Domain / Config
OpenAPI -> DTO / Response envelope
Frontend -> API contract
```

### 4.2 禁止的依赖方向

- handler 直接调用外部 API。
- handler 直接操作 GORM model。
- repository 反向依赖 handler。
- integration 依赖 HTTP 层。
- domain 依赖 Gin / GORM / HTTP / filesystem。
- 前端组件直接理解数据库字段名作为业务真理。

### 4.3 为什么要这样分

因为项目的核心风险不是“有没有页面”，而是：

1. 业务定义和运行事实混在一起。
2. 远端系统响应被误当作本地状态真理。
3. Demo 代码和正式代码没有边界。

这些风险只能靠强分层解决。

---

## 5. 包级职责设计

### 5.1 `cmd/ashan-frp`

职责：

- 读取配置。
- 初始化数据库。
- 初始化 repository / service / worker / runtime manager。
- 启动 HTTP server。
- 启动 job scheduler。
- 启动 SSE broker。

只允许存在一个 `main.go` 入口，不能再把初始化逻辑散进 handler 里。

### 5.2 `internal/bootstrap`

职责：

- 组装所有依赖。
- 注入 logger / config / db / repo / service / handler / runner。
- 控制启动顺序与关闭顺序。

建议内部提供：

- `NewApp(cfg)`
- `App.Start(ctx)`
- `App.Shutdown(ctx)`

### 5.3 `internal/config`

职责：

- 解析环境变量。
- 提供默认值。
- 保护敏感配置的读入。
- 把 build-time / runtime 配置分离。

应包括：

- `DATA_DIR`
- `DATABASE_URL` 或 SQLite path
- `FRPC_*`
- `ONEPANEL_*`
- `CHMLFRP_*`
- `CLOUDFLARE_*`
- `LOG_LEVEL`
- `HTTP_ADDR`
- `API_BASE_PATH`

### 5.4 `internal/domain`

职责：

- 定义稳定业务对象。
- 定义状态机、枚举、校验规则。
- 定义领域事件语义。
- 定义轻量值对象。

建议按子包拆分：

- `domain/node`
- `domain/tunnel`
- `domain/websitemapping`
- `domain/job`
- `domain/event`
- `domain/setting`
- `domain/runtime`

### 5.5 `internal/application`

职责：

- 提供用例级服务。
- 组合 domain / repository / integration / worker。
- 封装事务边界。
- 返回 DTO 所需数据。

建议按资源拆分：

- `application/node`
- `application/tunnel`
- `application/websitemapping`
- `application/settings`
- `application/job`
- `application/runtime`

### 5.6 `internal/repository`

职责：

- GORM 实现。
- 查询优化。
- 事务封装。
- 分页、过滤、排序。

建议每个聚合一个 repository 接口加一个 gorm 实现：

- `node_repo.go`
- `tunnel_repo.go`
- `website_mapping_repo.go`
- `job_repo.go`
- `event_repo.go`
- `setting_repo.go`
- `snapshot_repo.go`
- `audit_repo.go`

### 5.7 `internal/integration`

职责：

- `chmlfrp`：登录、节点列表、隧道/账号相关接口。
- `onepanel`：网站对象、代理、HTTPS、域名相关接口。
- `cloudflare`：DNS 记录、代理、区域信息相关接口。

原则：

- 只做请求/响应映射。
- 不做业务决策。
- 不直接写数据库。
- 不直接 emit UI 事件。

### 5.8 `internal/runtime/frpc`

职责：

- 生成 `frpc` 配置。
- 管理 `frpc` 子进程。
- 处理 reload / restart / stop / start / switch-node。
- 采集 stdout / stderr / exit code / pid / heartbeat。
- 为 `jobs` 和 `sync_state` 提供运行态输入。

### 5.9 `internal/worker`

职责：

- job 领取。
- lease 管理。
- 重试调度。
- 心跳与回收。
- 推动状态机。

### 5.10 `internal/http`

职责：

- 路由。
- 中间件。
- 统一响应 envelope。
- 输入校验。
- SSE 输出。
- Swagger / OpenAPI。

建议细分：

- `handlers/`
- `dto/`
- `response/`
- `middleware/`
- `sse/`
- `openapi/`

### 5.11 `internal/events`

职责：

- 事件 envelope。
- cursor 生成和续传。
- 事件广播与订阅管理。
- 事件过滤。

### 5.12 `internal/audit`

职责：

- 审计记录。
- 操作归因。
- before/after/diff 快照拼装。

### 5.13 `internal/web`

职责：

- 嵌入前端 build 产物。
- 为 HTTP server 提供静态资源文件系统。
- 管理 `go:embed` 所需的最终目录布局。

---

## 6. 目标代码与数据模型映射

### 6.1 领域对象建议

| 领域对象 | 建议文件 |
|---|---|
| Account | `internal/domain/account/*.go` |
| Node | `internal/domain/node/*.go` |
| Tunnel | `internal/domain/tunnel/*.go` |
| WebsiteMapping | `internal/domain/websitemapping/*.go` |
| Job | `internal/domain/job/*.go` |
| JobEvent | `internal/domain/event/*.go` |
| Setting | `internal/domain/setting/*.go` |
| RuntimeState / FrpcRuntime | `internal/domain/runtime/*.go` |

### 6.2 DB model 与 domain 的分离

- `domain` 是业务稳定模型。
- `repository/gorm` 是持久化模型。
- 二者不要直接混用。
- GORM tag 只能存在于 repository 层，不进入业务规则层。

### 6.3 DTO 与 domain 的分离

- `http/dto` 对应 API 输入输出。
- `application` 接收 command / query。
- `domain` 只做状态和规则。
- `dto` 不应直接导入 repository。

---

## 7. API handler 与 service 的映射

### 7.1 节点

| 路径 | handler | service | repository / integration |
|---|---|---|---|
| `GET /api/v1/nodes` | `handlers/node/list.go` | `application/node/query.go` | `repository/node_repo.go` |
| `POST /api/v1/nodes` | `handlers/node/create.go` | `application/node/command.go` | `repository/node_repo.go` |
| `PATCH /api/v1/nodes/{id}` | `handlers/node/update.go` | `application/node/command.go` | `repository/node_repo.go` |
| `POST /api/v1/nodes/{id}/actions/check` | `handlers/node/actions.go` | `application/node/actions.go` | `worker/runner.go` |

### 7.2 隧道

| 路径 | handler | service | 下游 |
|---|---|---|---|
| `GET /api/v1/tunnels` | `handlers/tunnel/list.go` | `application/tunnel/query.go` | `repository/tunnel_repo.go` |
| `POST /api/v1/tunnels` | `handlers/tunnel/create.go` | `application/tunnel/command.go` | `repository/tunnel_repo.go` |
| `PATCH /api/v1/tunnels/{id}` | `handlers/tunnel/update.go` | `application/tunnel/command.go` | `repository/tunnel_repo.go` |
| `POST /api/v1/tunnels/{id}/actions/apply` | `handlers/tunnel/actions.go` | `application/tunnel/actions.go` | `worker/runner.go`, `runtime/frpc` |

### 7.3 网站映射

| 路径 | handler | service | 下游 |
|---|---|---|---|
| `GET /api/v1/website-mappings` | `handlers/website/list.go` | `application/websitemapping/query.go` | `repository/website_mapping_repo.go` |
| `POST /api/v1/website-mappings` | `handlers/website/create.go` | `application/websitemapping/command.go` | `repository/website_mapping_repo.go` |
| `PATCH /api/v1/website-mappings/{id}` | `handlers/website/update.go` | `application/websitemapping/command.go` | `repository/website_mapping_repo.go` |
| `POST /api/v1/website-mappings/{id}/actions/sync` | `handlers/website/actions.go` | `application/websitemapping/actions.go` | `worker/runner.go`, `integration/onepanel` |

### 7.4 设置 / 集成 / runtime / jobs

| 路径类别 | handler package | service package | 下游 |
|---|---|---|---|
| `settings` | `handlers/settings/*.go` | `application/settings` | `repository/setting_repo.go` |
| `integrations/*` | `handlers/integration/*.go` | `application/integration` | `integration/*` |
| `frpc/runtime` | `handlers/runtime/*.go` | `application/runtime` | `runtime/frpc`, `worker` |
| `jobs` | `handlers/job/*.go` | `application/job` | `repository/job_repo.go`, `worker` |
| `logs` | `handlers/log/*.go` | `application/log` | `repository/event_repo.go`, `audit` |

---

## 8. 请求处理链路

### 8.1 保存节点的链路

```text
HTTP Request
  -> handler 解析 DTO
  -> validator 校验
  -> application/node service
  -> repository/node 保存
  -> response envelope
```

特点：

- 只落本地意图。
- 不应触发远端副作用。
- 不需要 job。

### 8.2 保存隧道并排队应用的链路

```text
HTTP Request
  -> handler 解析 DTO
  -> application/tunnel service 保存期望态
  -> repository/tunnel 保存
  -> application/job 创建 apply job
  -> worker 后续领取并执行
  -> 通过 SSE 推送状态变化
```

特点：

- 保存和执行分离。
- `job_id` 必须返回。
- 前端显示“已保存，已加入任务队列”。

### 8.3 网站映射同步的链路

```text
HTTP Request
  -> 保存映射意图
  -> 创建 sync job
  -> job runner 调用 onepanel adapter
  -> 写入 snapshots / job_events / sync_state
  -> SSE 推送结果
```

特点：

- 远端状态永远是可变的。
- 本地状态源保存的是意图与对账证据。
- 同步失败可阻塞或重试，但不应悄悄吞掉错误。

### 8.4 设置保存的链路

```text
HTTP Request
  -> handler 合并 patch DTO
  -> application/settings service 进行分组校验
  -> repository/setting 批量写入
  -> 若涉及凭据验证，可追加异步 verify job
```

特点：

- 非敏感设置可同步保存。
- 凭据修改后，验证动作建议异步化。
- 保存成功不等于验证成功。

---

## 9. 嵌入式 UI 结构

### 9.1 当前实现

当前仓库不再维护独立的 `frontend/` 运行时工程。用户界面以 Go 二进制内嵌的静态页为准：

- `frp-backend/internal/web/dist/index.html`
- `frp-backend/internal/web/embed.go`

这意味着：

- UI 由 Go 服务直接托管。
- 事件、列表、按钮动作都只走本地 Go API。
- 不再依赖单独的 `frontend build -> frontend/dist` 产物链。

### 9.2 前端与后端的边界

- UI 只处理展示、输入和提交。
- UI 不直接拼接外部 API。
- UI 不直接理解数据库结构。
- UI 只认识 API envelope 和 DTO。
- 实时更新统一走 `/api/v1/events/stream`。

### 9.3 未来扩展方式

如果后续需要更复杂的交互，再在 `frp-backend/internal/web/dist/` 里补充更完整的内嵌脚本与样式；只有当确实需要独立工程时，才重新引入单独前端目录。当前阶段不保留这条独立构建链。

---

## 10. go:embed 与静态资源结构

### 10.1 目标嵌入方式

当前实现直接把静态 UI 放在 `frp-backend/internal/web/dist/`，并由 `frp-backend/internal/web/embed.go` 通过 `go:embed` 挂载。

负责：

- `//go:embed dist`
- 返回静态文件系统
- 供 HTTP router 挂载 `/ui/`

### 10.2 为什么要这样做

- UI 和后端一起发布，Docker 镜像只保留一个 Go 二进制。
- 不再依赖独立前端构建链。
- router、asset、页面刷新、OpenAPI 各自独立演进。

---

## 11. Build / Docker / CI 设计

### 11.1 建议构建链

```text
backend build -> 单一 Go binary
Docker final image -> alpine + binary + ca-certificates + tzdata
```

### 11.2 Docker 目标形态

- 后端二进制内嵌静态资源。
- 最终镜像尽量只保留 binary + data directory。
- 不引入 Nginx，不引入 Python，不保留独立静态服务器。

### 11.3 GitHub Actions 目标步骤

1. checkout
2. setup-go
3. `go test ./...`
4. `go build ./cmd/ashan-frp`
5. Docker buildx
6. push 镜像

---

## 12. 从当前代码迁移到目标结构的切片顺序

### Phase 1：可编译骨架

目标：先把项目从 demo 态切到“可编译、可分层、可扩展”。

- 引入 `go.mod`
- 建立 `cmd/ashan-frp`
- 建立 `internal/bootstrap`
- 建立 `internal/http/router.go`
- 把当前 demo 代码拆出 HTTP / config / runtime 的最小边界

### Phase 2：合同层

目标：把 API 请求体和响应 envelope 固化。

- DTO
- 统一 response envelope
- 错误码
- SSE envelope
- OpenAPI / Swagger

### Phase 3：业务用例层

目标：把节点 / 隧道 / 网站映射 / 设置 / job 的用例拆清楚。

- application services
- domain rules
- repository 接口
- GORM 实现

### Phase 4：执行与外部适配层

目标：把 1Panel / ChmlFrp / Cloudflare / frpc 都从 demo 中剥离成独立 adapter。

- integration clients
- frpc runtime manager
- job runner
- snapshot / event / audit

### Phase 5：前端与 API 对齐

目标：让 UI 只依赖这份合同，不再依赖 demo 假接口。

- 嵌入式 UI 的静态脚本
- SSE channel 统一

### Phase 6：清理 demo 与历史文件

目标：删掉不再需要的平面 demo 和历史文件。

- 旧 flat handler
- 错误 import / Markdown URL demo
- 无关 raw json / log 文件
- README 中与目标架构冲突的旧说明

---

## 13. 现有文件到目标文件的映射

| 当前文件 | 目标去向 |
|---|---|
| `frp-backend/main.go` | `cmd/ashan-frp/main.go` |
| `frp-backend/handlers.go` | `internal/http/handlers/*` |
| `frp-backend/models.go` | `internal/domain/*` + `internal/repository/gorm/*` |
| `frp-backend/controller.go` | `internal/worker/*` + `internal/runtime/frpc/*` |
| `frp-backend/chmlfrp.go` | `internal/integration/chmlfrp/client.go` |
| `frp-backend/onepanel.go` | `internal/integration/onepanel/client.go` |
| `frp-backend/auth_watchdog.go` | `internal/worker/auth_watchdog.go` 或 `internal/application/settings` |
| `frp-backend/internal/web/dist/index.html` | Go 嵌入式 UI 入口 |
| `frp-backend/internal/web/embed.go` | 静态资源挂载 |

---

## 14. 设计约束与不做事项

### 14.1 不做兼容层

- 不保留旧 shell 工作流为长期并存入口。
- 不保留旧 flat model 作为正式合同。
- 不做 runtime fallback 去适配错误设计。

### 14.2 不做“handler 里一把梭”

- handler 不写 SQL。
- handler 不调外部 API。
- handler 不管理进程。
- handler 不保存业务决策。

### 14.3 不做“前端直连外部系统”

- 前端只连本地 API。
- 1Panel / ChmlFrp / Cloudflare 只由后端适配层访问。

---

## 15. 结论

这份代码结构与整体架构设计的核心结论是：

1. **项目应收敛为模块化单体，不拆成多服务**。
2. **Go 后端要从 flat demo 变成分层模块**。
3. **API、DTO、Domain、Repository、Integration、Worker、Runtime 要硬切边界**。
4. **前端只依赖 API 合同与 SSE 合同，不依赖 demo 接口**。
5. **go:embed、OpenAPI、Swagger、job runner、frpc runtime 都是正式架构的一部分，不是附属品**。

如果后续开始落实，这份文档就是代码重构的总蓝图：

- 先切结构
- 再切合同
- 再切用例
- 再切集成
- 最后切前端与部署
