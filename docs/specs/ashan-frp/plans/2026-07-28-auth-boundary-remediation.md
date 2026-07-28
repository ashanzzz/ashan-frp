# 认证边界与凭据安全修复计划

> 状态：**阻断发布**。在本计划完成并通过验收前，任何包含当前认证/凭据改动的分支不得合并。
>
> 范围：修复本地 session、Cloudflare、ChmlFrp 认证边界；保留已完成的控制台、OAuth、隧道及后台能力，不以“重写”为代价。

## 1. 问题事实

本次审查确认以下问题不能靠前端提示或单个单测掩盖：

1. 本地 session 的 `401 UNAUTHORIZED` 与服务商认证失败是两条不同边界。任何前端受保护请求若不统一处理 `401`，都会把“本系统未登录”错报为 Cloudflare/ChmlFrp 问题，或在失败后显示操作成功。
2. ChmlFrp 同时存在两种合法认证模式：历史用户名/密码账号，以及新 API Token/OAuth Token。不能用“第二个字符串参数非空”猜测认证方式；这种猜测会把历史密码当作 Bearer Token。
3. 任何要求验证新 Token 的 Settings PATCH 必须先完成验证，再原子保存全部设置和凭据。验证失败时不允许留下 general、sync、queue、FRPC 或 integration 的半成品写入。
4. 已认证的单管理员设置页允许完整明文显示当前 Cloudflare/ChmlFrp Token；但必须来自加密存储，并且当前 ChmlFrp 账户必须由 `/userinfo` 验证，不能展示 `token`、`oauth2_user` 或 Token 本身等伪身份。

## 2. 非目标

- 不把已认证单管理员设置页的明文回显改回掩码。
- 不删除现有 ChmlFrp OAuth、用户名/密码兼容能力、Cloudflare API Token/Global API Key 支持或控制台功能。
- 不以新增“兼容层”掩盖未定义的认证语义；调用方必须显式选择认证方式。

## 3. 实施顺序与验收

### 阶段 A：将 ChmlFrp 认证方式显式化

**目标：** 历史用户名/密码调用继续先登录；API Token/OAuth Token 调用只使用 Token；两种路径不可互相猜测。

**文件：**
- 修改：`frp-backend/internal/integration/chmlfrp/client.go`
- 修改：`frp-backend/internal/integration/chmlfrp/client_test.go`
- 修改：`frp-backend/internal/http/handlers/settings.go`
- 修改：`frp-backend/internal/worker/runner.go`（按新的显式构造器迁移每个调用点）

**步骤：**
1. 写失败测试：用户名/密码客户端调用 `GetNodes` 时先请求登录端点；Token 客户端调用同一方法时绝不请求登录端点。
2. 将构造 API 分成明确的用户名/密码和 Token 路径，例如 `NewPasswordClient`、`NewTokenClient`，或以受限枚举的认证模式构造；禁止以字符串形状推断。
3. 逐个迁移 handler 与 worker 调用点；历史凭据保留用户名/密码路径，新设置保存的 Token 使用 Token 路径。
4. 对历史 `token`/`oauth2_user` 标识符：回显时标为“未验证”，只能由 Token 验证动作刷新为真实账户，绝不冒充当前账户。
5. 运行 `go test ./internal/integration/chmlfrp ./internal/http/handlers ./internal/worker`。

**完成标准：** 两条认证路径各有独立回归测试，且 worker 使用的路径在测试中可观察、不可被参数猜测改变。

### 阶段 B：将 Settings Token 验证与写入做成原子操作

**目标：** 任一新 ChmlFrp Token 验证失败时，所有 Settings 与凭据保持原样。

**文件：**
- 修改：`frp-backend/internal/http/handlers/settings.go`
- 修改：`frp-backend/internal/repository/` 中已有事务边界或新增最小事务方法
- 修改：`frp-backend/internal/http/handlers/settings_test.go`

**步骤：**
1. 写失败测试：提交新的 general/sync/queue/FRPC 值和无效 ChmlFrp Token；断言 HTTP 错误后设置、integrations、凭据均等于提交前状态。
2. 在任何 repository 写入前验证 Token 与 `/userinfo`；验证得到真实账户后，才进入数据库事务。
3. 在同一事务内写入普通设置、integration metadata 与加密凭据；任一步失败即回滚。
4. 保持 Cloudflare configure 的独立验证/保存边界，不让通用 PATCH 覆盖已验证的 Cloudflare 元数据。
5. 运行 `go test ./internal/http/handlers ./internal/repository`。

**完成标准：** 无效 Token、数据库写入失败和有效 Token 三种情况均有测试；失败不留下任何部分更新。

### 阶段 C：统一前端受保护请求与 401 恢复

**目标：** 本地 session 失效永远进入登录层，永远不伪装为上游错误或成功操作。

**文件：**
- 修改：`frp-backend/internal/web/src/App.vue`
- 修改/新增：`frp-backend/internal/web/src/provider-errors.js` 或统一认证请求模块
- 修改：`frp-backend/internal/web/app.test.mjs`

**步骤：**
1. 列出 `App.vue` 中所有 `/api/v1` 受保护请求。除 session probe 与登录请求外，不允许裸 `fetch`。
2. 写失败测试：Tunnel 创建、删除、ChmlFrp 同步、故障转移、节点动作和 FRPC 动作收到 `401` 时，必须调用统一 `requireLogin()`；不得改变状态或显示成功 notice。
3. 让统一请求函数负责：解析 JSON、识别 `401/UNAUTHORIZED`、打开登录层、抛出可区分的本地认证错误；调用方仅在成功结果后更新界面。
4. Cloudflare 认证错误只在后端明确返回 Cloudflare 代码时显示为 Cloudflare 错误；同样规则适用于 ChmlFrp。
5. 运行 `node --test internal/web/app.test.mjs && npm run build`。

**完成标准：** 搜索结果证明无绕过统一请求器的受保护 API 调用；每类变更操作在 `401` 时均不显示成功。

### 阶段 D：提供可重复的端到端验证

**目标：** 不依赖真实 Token，也能覆盖登录、服务商 mock、持久化、前端登录恢复和浏览器行为。

**文件：**
- 修改/新增：`frp-backend/internal/http/*_test.go`
- 修改/新增：`frp-backend/internal/web/*` 的浏览器测试配置与测试文件
- 修改：`AGENTS.md`、`docs/specs/ashan-frp/design/change-safety-contract.md`

**步骤：**
1. 使用本地 mock Cloudflare/ChmlFrp server：未登录时断言 mock 没有收到请求；登录后断言请求才到达 mock。
2. 对 ChmlFrp mock 覆盖密码登录、Bearer Token、`/userinfo` 成功和失败。
3. 为嵌入式 UI 增加真实浏览器测试：未登录遮罩、重新登录、401 后无成功提示、当前账户和明文 Token 回显。
4. 浏览器服务不可用时，明确标记为发布阻断，不能用静态 bundle 检查替代“已完成浏览器验证”。
5. 运行完整验证矩阵：`go test ./...`、`go vet ./...`、`node --test internal/web/app.test.mjs`、`npm run build`、浏览器测试、Docker smoke、CI。

**完成标准：** 所有关键边界由 mock + 浏览器双重证明；CI 与本地输出均为通过。

## 4. 发布门槛

只有同时满足以下条件才可以合并：

- 阶段 A–D 的回归测试均通过。
- `git diff --check` 无输出，工作树无意外生成文件或真实凭据。
- UI 改动有真实浏览器测试通过；浏览器不可用即不得声称完成 UI 验证。
- 独立代码审查确认没有把用户名/密码认证改为 Token、没有裸受保护请求、没有部分写入。
- PR CI 的测试、构建和镜像发布检查全部成功。
