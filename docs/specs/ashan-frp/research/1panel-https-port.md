# 1Panel HTTPS / 非 443 端口行为研究

## 结论先行

- `httpsPort` 不是 `Website` 表里的独立持久化字段。
- 单站 HTTPS 页面里看到的 `httpsPort` 是只读展示值，来源于 `GET /websites/:id/https`。
- 真正决定 HTTPS 监听端口的，是 OpenResty 安装实例的 `AppInstall.HttpsPort`，再叠加网站域名端口集合。
- 因此，`30001` 这类非 443 端口是“可以成立”的，但它不是站点自己单独保存的端口；它必须进入 OpenResty 安装端口 / 域名端口这条链路。
- 目前可见的单站 HTTPS 更新逻辑没有把 `HttpsPorts[]` 当成有效写入目标；保存页面不会把 `httpsPort` 写回 `Website` 表。

## 先把“展示值”和“持久化值”分开

### 展示值

- `response.WebsiteHTTPS.HttpsPort`：后端拼出来的字符串，前端直接展示。
- 前端页面 `frontend/src/views/website/website/config/basic/https/index.vue` 里，`<el-text>{{ form.httpsPort }}</el-text>` 只是展示，没有输入框。
- `frontend/src/views/website/website/components/https/index.vue` 里虽然 props 里有 `httpsPort`，但组件本身也不是编辑端口，只是参与表单状态。

### 持久化值

- `model.AppInstall.HttpsPort`：OpenResty 安装实例持久化端口。
- `model.WebsiteDomain.Port`：站点域名绑定端口。
- `model.Website.HttpConfig`：`HTTPSOnly` / `HTTPAlso` / `HTTPToHTTPS`。
- `model.Website.WebsiteSSLID`：站点绑定的证书。
- `model.WebsiteSSL`：只存证书、域名、签发信息，不存端口。

### 不存在的东西

- `model.Website` 里没有单独的 `httpsPort` 列。
- `model.WebsiteSSL` 里也没有站点 HTTPS 端口字段。
- 所以不能把页面上看到的 `httpsPort` 当成“网站模型里的真实存储值”。

## 数据流：`httpsPort` 是怎么来的

### 读取路径

1. 前端调用 `GET /websites/:id/https`。
2. 后端 `WebsiteService.GetWebsiteHTTPS()` 进入 `getHttpsPort(websiteId)`。
3. `getHttpsPort` 会读取该网站的域名记录：
   - 如果 `domain.SSL == true`，就把 `domain.Port` 收集进返回集合。
   - 如果某个域名端口等于 OpenResty 的 `HttpPort`，则额外把 OpenResty 的 `HttpsPort` 放进结果。
   - 如果最终没有收集到任何 https 端口，则回退为未启用 SSL 的域名端口集合。
4. 结果被 `strings.Join(..., ",")` 拼成 `response.WebsiteHTTPS.HttpsPort`。

这意味着：

- 这个字段本质上是“从域名和 OpenResty 安装端口派生出来的展示值”。
- 它不是单点写入的配置项。
- 同一个站点如果有多个 SSL 端口，展示值可能是逗号分隔字符串，而不是单个数值。

### 保存路径

`POST /websites/:id/https` 对应 `WebsiteService.OpWebsiteHTTPS()`，核心行为是：

- 变更 `website.Protocol`、`website.WebsiteSSLID`、`website.HttpConfig`。
- 根据证书和域名重新写 nginx 站点配置。
- 重载 nginx。

在本次阅读到的代码里，没有看到它把 `Website` 表写出一个独立 `httpsPort`。

## 端口写回链路的核心逻辑

### `applySSL` 是真正改 nginx 配置的地方

`applySSL(website, websiteSSL, req)` 做了这些事：

- 读取站点域名列表。
- 把已启用 SSL 的域名端口收集到 `httpsPorts`。
- 如果站点绑定了 OpenResty 的默认 HTTP 端口，就把 OpenResty 的 `HttpsPort` 加进去。
- 如果没有任何 SSL 端口，则回退使用非 SSL 域名端口。
- 对每个 HTTPS 端口调用 `setListen(..., ssl=true)`。
- 按 `req.HttpConfig` 选择：
  - `HTTPSOnly`
  - `HTTPToHTTPS`
  - `HTTPAlso`
- 最后写 nginx 配置并生成 PEM 文件。

### `setListen` 和 `AddHTTP2HTTPS`

- `setListen(server, port, ipv6, http3, defaultServer, ssl)` 会把 listen 写成对应的 `ssl` / `quic` 组合。
- `AddHTTP2HTTPS(httpsPort)` 会在 `httpsPort != 443` 时写成：
  - `return 301 https://$host:<port>$request_uri`
- 当 `httpsPort == 443` 时，才会省略端口号。

这说明：

- 非 443 端口在重定向层面是被明确支持的。
- 代码并不是把 443 写死在跳转逻辑里。

## OpenResty 安装端口和网站 HTTPS 的耦合

### OpenResty 端口来自安装实例

- `getAppInstallPort(constant.AppOpenresty)` 直接返回 `install.HttpPort` / `install.HttpsPort`。
- `AppInstall` 模型本身就持久化了这两个字段。
- `WebsiteDomain` 创建时会把 OpenResty 的 HTTP / HTTPS 端口传进 `getWebsiteDomains(...)`。

### 默认 server 也跟着 OpenResty 安装端口走

- `handleSSLConfig(...)` / `updateDefaultServer(...)` 会把默认 server 改写为：
  - `listen <HttpPort>`
  - `listen <HttpsPort> ssl`
  - `listen <HttpsPort> quic reuseport`
- `nginx.go` 的 disable 分支也会按 `appInstall.HttpsPort` 删除 listen / include / http2 / ssl_reject_handshake。

### 端口校验有“默认 HTTPS 端口豁免”

- `checkWebsitePort(defaultHTTPsPort, port, websiteType)` 对 `port == defaultHTTPsPort` 不做 `common.ScanPort` 冲突扫描。
- 也就是说：OpenResty 的默认 HTTPS 端口在站点域名端口校验里是特殊值。
- 但它仍会检查是否被其他 app / runtime 占用。

## 能不能用 30001？

### 可以，但要分清是谁的 30001

1. 如果你说的是“OpenResty 的 HTTPS 安装端口能不能是 30001”：
   - 可以。
   - `AppInstall.HttpsPort` 是持久化字段，`app_install.go` 里会对 `PANEL_APP_PORT_HTTPS` 做校验后写回。
   - `updateDefaultServer(...)`、`handleSSLConfig(...)`、`getHttpsPort(...)`、`AddHTTP2HTTPS(...)` 都会跟着它走。

2. 如果你说的是“站点自己在 HTTPS 页面里单独指定一个 30001”：
   - 目前看不到真正的持久化路径。
   - 页面上的 `httpsPort` 是展示值，不是输入框。
   - `WebsiteHTTPSOp` 里虽然有 `HttpsPorts []int`，但在本次阅读到的 `OpWebsiteHTTPS` / `applySSL` 路径里没有看到它被消费成真正的写回端口。

### 非 443 的具体行为

- `HTTPToHTTPS` 会生成带端口的跳转。
- `setListen` 会把 30001 写成 `listen 30001 ssl` / `listen [::]:30001 ssl`。
- 站点域名端口如果就是 30001，也会被纳入 HTTPS 端口集合。

## 失败案例 / 注意事项

### 1) `httpsPort` 是展示值，不是站点持久化列

这是最容易混淆的点。UI 上看到的值，不代表 `Website` 表里有一列叫 `httpsPort`。

### 2) `HttpsPorts[]` 在当前单站保存链路里不一定生效

- `WebsiteHTTPSOp` 和批量接口里都有 `HttpsPorts`。
- 但在本次阅读到的 `OpWebsiteHTTPS` / `applySSL` 里，没有看到把这个字段作为独立端口配置写入 nginx。
- 因此不要把它理解成“点了就能改站点 HTTPS 端口”的稳定能力。

### 3) HTTP/3 的 `Alt-Svc` 头是硬编码 `:443`

`applySSL` 和 `ChangeHSTSConfig` 里都写了固定的：

- `Alt-Svc: 'h3=":443"; ma=2592000'`

这对非 443 HTTPS 很危险：

- 浏览器会被告知去 443 走 HTTP/3。
- 如果实际 HTTPS 在 30001，上报的 Alt-Svc 端口就不匹配。

所以：

- 非 443 + HTTP/3 是当前实现里最明显的兼容风险。
- 如果要做设计文档，必须把这个当作限制条件写进去。

### 4) 端口冲突校验仍然存在

- `checkPort` 会检查 app install / host scan / runtime 占用。
- `checkWebsitePort` 也会检查 domain / app / runtime / host scan。
- 所以“30001 可用”不是绝对的，仍要看是否被别的服务占用。

### 5) `applySSL` 里有一些错误处理比较宽松

本次阅读到的代码里，`applySSL` 某些内部错误会直接返回 `nil`，排障时不要只看表层成功/失败消息，最好再检查生成的 nginx 配置是否真的落盘。

## 适合放进后续设计/文档里的表述

建议把术语统一成下面这样：

- “站点 HTTPS 展示端口” = `response.WebsiteHTTPS.HttpsPort`
- “OpenResty 安装 HTTPS 端口” = `model.AppInstall.HttpsPort`
- “站点实际监听端口” = `WebsiteDomain.Port` + nginx `listen` 指令
- “站点 HTTPS 开关和证书绑定” = `website.Protocol` / `website.WebsiteSSLID` / nginx 配置

如果未来要做“站点级独立 HTTPS 端口”，那不是改一个 UI 字段就够了，至少要补齐：

- 持久化字段
- request / response DTO
- 校验逻辑
- nginx 配置写回
- HTTP→HTTPS 跳转
- HTTP/3 的 `Alt-Svc`

## 本次研究覆盖的关键文件

- `/opt/data/cache/1Panel/agent/app/model/app_install.go`
- `/opt/data/cache/1Panel/agent/app/model/website.go`
- `/opt/data/cache/1Panel/agent/app/model/website_ssl.go`
- `/opt/data/cache/1Panel/agent/app/dto/request/website.go`
- `/opt/data/cache/1Panel/agent/app/dto/response/website.go`
- `/opt/data/cache/1Panel/agent/app/service/website.go`
- `/opt/data/cache/1Panel/agent/app/service/website_op.go`
- `/opt/data/cache/1Panel/agent/app/service/website_domain.go`
- `/opt/data/cache/1Panel/agent/app/service/website_utils.go`
- `/opt/data/cache/1Panel/agent/app/service/app_utils.go`
- `/opt/data/cache/1Panel/agent/app/service/app_install.go`
- `/opt/data/cache/1Panel/agent/app/service/nginx_utils.go`
- `/opt/data/cache/1Panel/agent/app/service/nginx.go`
- `/opt/data/cache/1Panel/frontend/src/views/website/website/config/basic/https/index.vue`
- `/opt/data/cache/1Panel/frontend/src/views/website/website/components/https/index.vue`
- `/opt/data/cache/1Panel/frontend/src/api/interface/website.ts`
- `/opt/data/cache/1Panel/frontend/src/api/modules/website.ts`

## 总结

当前实现里，非 443 HTTPS 端口的“真源头”是 OpenResty 安装端口，而不是网站模型里的独立字段。

所以如果你的问题是“1Panel 能不能在 30001 上跑 HTTPS”：能，但要走 OpenResty 安装端口 / 域名端口这条链路；如果你的问题是“网站本身有没有一个可持久化的 `httpsPort`”：没有。
