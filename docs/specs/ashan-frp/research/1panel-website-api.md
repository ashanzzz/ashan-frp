# 1Panel website API 研究

本文聚焦 1Panel 的 website 模块中与网站列表、创建/更新、域名、反向代理、proxy cache、HTTPS 相关的后端 API、DTO、service 行为，以及前端页面调用链。目标是为 `ashan-frp` 后续对接或复刻同类能力提供可以直接映射的参考，而不是停留在概述层。

## 1. 结论摘要

1. 1Panel 的 website 能力不是集中在一个大 service 中实现，而是拆分在多个文件里：
   - `agent/app/service/website.go`：站点主流程、列表、更新、HTTPS、预检等
   - `agent/app/service/website_domain.go`：域名新增/查询/删除/更新
   - `agent/app/service/website_proxy.go`：代理配置创建/编辑/删除/启停、proxy cache、文件读写
2. 前端与后端接口已经明确对齐，且页面层级很清楚：
   - 网站列表页负责搜索、进入详情、基础更新
   - 站点详情页下的 Basic tabs 再拆域名、代理、HTTPS 等子模块
3. 域名、代理、HTTPS、proxy cache 都不是“纯数据库状态”，真实状态来自数据库 + nginx 配置文件 + include 目录 + WAF/防火墙副作用。
4. 代理启用/禁用不是改配置内容，而是把 `*.conf` 与 `*.bak` 互换，然后重载 nginx/openresty。
5. proxy cache 的开关本质上是写入/删除 nginx `proxy_cache_path`；读取时则是从站点 nginx 配置里反解析，而不是查数据库字段。
6. 域名新增会联动防火墙端口、WAF `sites.json`、nginx `listen/server_name`；删除时也会同步清理这些关联。
7. 对 `ashan-frp` 来说，最稳妥的建模方式是：站点对象、域名集合、代理 include 文件、cache 配置、HTTPS 配置分层管理，而不是只保留一张网站表。
8. 自动化可以覆盖大多数创建/更新/绑定/代理/缓存/HTTPS 操作；需要人工确认的是域名端口、SSL、cache 策略，以及任何会直接影响线上 nginx 文件的破坏性动作。

## 2. Endpoint inventory

### 2.1 网站列表 / 创建 / 更新 / 预检

- `POST /websites/search`
  - 前端：`searchWebsites(req)`
  - 用途：分页搜索网站列表
- `GET /websites/list`
  - 前端：`listWebsites()`
  - 用途：获取网站列表（更多用于下拉/选择场景）
- `GET /websites/:id`
  - 前端：`getWebsite(id)`
  - 用途：获取单个网站详情
- `POST /websites/check`
  - 前端：`preCheck({})`
  - 用途：创建前预检，返回冲突/依赖异常列表
- `POST /websites`
  - 前端：`createWebsite(req)`
  - 用途：创建网站
- `POST /websites/update`
  - 前端：`updateWebsite(req)`
  - 用途：更新网站基础信息

### 2.2 域名

- `GET /websites/domains/:websiteId`
  - 前端：`listDomains(id)`
  - 用途：查询某站点的域名列表
- `POST /websites/domains`
  - 前端：`createDomain(req)`
  - 用途：批量创建域名
- `POST /websites/domains/update`
  - 前端：`updateDomain(req)`
  - 用途：更新单个域名的 SSL 开关
- `POST /websites/domains/del`
  - 前端：`deleteDomain(req)`
  - 用途：删除域名

### 2.3 代理

- `POST /websites/proxies`
  - 前端：`getProxyConfig({ id })`
  - 用途：查询站点 proxy 配置列表
- `POST /websites/proxies/update`
  - 前端：`operateProxyConfig(req)`
  - 用途：创建/编辑/删除/启停代理的统一入口
- `POST /websites/proxies/status`
  - 前端：`updateProxyConfigStatus(req)`
  - 用途：启用/禁用代理
- `POST /websites/proxies/delete`
  - 前端：`deleteProxyConfig(req)`
  - 用途：删除代理配置
- `POST /websites/proxies/file`
  - 前端：`updateProxyConfigFile(req)`
  - 用途：直接编辑代理源文件

### 2.4 proxy cache

- `GET /websites/proxy/config/:id`
  - 前端：`getCacheConfig(id)`
  - 用途：读取 proxy cache 配置
- `POST /websites/proxy/config`
  - 前端：`updateCacheConfig(req)`
  - 用途：更新 proxy cache 配置
- `POST /websites/proxy/clear`
  - 前端：`clearProxyCache({ websiteID })`
  - 用途：清空 cache 目录并重载 openresty

### 2.5 HTTPS

- `GET /websites/:id/https`
  - 前端：`getHTTPSConfig(id)`
  - 用途：读取站点 HTTPS 配置
- `POST /websites/:id/https`
  - 前端：`updateHTTPSConfig(req)`
  - 用途：更新站点 HTTPS 配置

## 3. 请求 / 响应 DTO

### 3.1 网站搜索、列表、详情

`request.WebsiteSearch`
- `PageInfo`
- `name`
- `orderBy`：required，允许值 `primary_domain type status createdAt expire_date created_at favorite`
- `order`：required，允许值 `null ascending descending`
- `websiteGroupId`
- `type`

`response.WebsiteRes`
- `id`
- `createdAt`
- `protocol`
- `primaryDomain`
- `type`
- `alias`
- `remark`
- `status`
- `expireDate`
- `sitePath`
- `appName`
- `runtimeName`
- `sslExpireDate`
- `appInstallId`
- `childSites`
- `parentSite`
- `runtimeType`
- `favorite`
- `IPV6`

`response.WebsiteDTO`
- `errorLogPath`
- `accessLogPath`
- `sitePath`
- `appName`
- `runtimeName`
- `runtimeType`
- `siteDir`
- `openBaseDir`
- `algorithm`
- `UDP`
- `servers`
- 以及 `model.Website` 的全部基础字段

`response.WebsiteOption`
- `id`
- `primaryDomain`
- `alias`

### 3.2 网站创建 / 更新

`request.WebsiteCreate`
- required：`type`, `alias`, `webSiteGroupID`
- 其他常用字段：`remark`, `proxy`, `IPV6`, `domains`, `appType`, `appInstall`, `appID`, `appInstallID`, `runtimeID`, `taskID`, `parentWebsiteID`, `siteDir`
- 嵌套结构：
  - `RuntimeConfig`：`proxyType`, `port`
  - `FtpConfig`：`ftpUser`, `ftpPassword`
  - `DataBaseConfig`：`createDb`, `dbName`, `dbUser`, `dbPassword`, `dbHost`, `dbFormat`
  - `SSLConfig`：`enableSSL`, `websiteSSLID`
  - `StreamConfig`：`streamPorts`, `name`, `algorithm`, `udp`, `servers`

`request.WebsiteUpdate`
- required：`ID`, `PrimaryDomain`
- 其他字段：`Remark`, `webSiteGroupID`, `ExpireDate`, `IPV6`, `Favorite`

`request.WebsiteInstallCheckReq`
- `InstallIds[]`，前端创建页通常直接传 `{}`，让后端返回需要人工处理的预检项

`response.WebsitePreInstallCheck`
- `Name`
- `Status`
- `Version`
- `AppName`

注意：前端 TS interface 的字段名有时和 Go JSON tag 不完全一致；如果直接按 API 对接，建议以 Go DTO 的 `json:"..."` tag 为准。

### 3.3 域名

`request.WebsiteDomainCreate`
- `WebsiteID`：required
- `Domains[]`：required，批量创建

`request.WebsiteDomain`
- `Domain`：required
- `Port`
- `SSL`

`request.WebsiteDomainUpdate`
- `ID`：required
- `SSL`

`request.WebsiteDomainDelete`
- `ID`：required

返回值方面：
- 查询接口返回数组
- 创建接口在 service 层实际返回 `[]model.WebsiteDomain`
- 删除/更新成功后只返回成功态

### 3.4 代理

`request.WebsiteProxyReq`
- `ID`：required

`request.WebsiteProxyConfig`
- `ID`：required
- `Operate`：required，后端分发值为 `create` / `edit` / `delete` / `enable` / `disable`
- `Enable`
- `Cache`
- `CacheTime`
- `CacheUnit`
- `ServerCacheTime`
- `ServerCacheUnit`
- `Name`：required
- `Modifier`
- `Match`：required
- `ProxyPass`：required
- `ProxyHost`：required
- `Content`
- `FilePath`
- `Replaces`
- `SNI`
- `ProxySSLName`
- `SSLVerify`
- CORS 相关：`Cors`, `AllowOrigins`, `AllowMethods`, `AllowHeaders`, `AllowCredentials`, `Preflight`

`request.WebsiteProxyDel`
- `ID`：required
- `Name`：required

`request.WebsiteProxyStatusUpdate`
- `ID`：required
- `Name`：required
- `Status`：required

`request.NginxProxyUpdate`
- `WebsiteID`：required
- `Name`：required
- `Content`：required

`response.NginxProxyCache`
- `Open`
- `ShareCache`
- `ShareCacheUnit`
- `CacheLimit`
- `CacheLimitUnit`
- `CacheExpire`
- `CacheExpireUnit`

前端 `ProxyConfig` 还额外带一个 UI 状态字段 `browserCache`，用于把 `cacheTime` 显示成“启用 / 禁用 / 不修改”。

### 3.5 HTTPS

`request.WebsiteHTTPSOp`
- `WebsiteID`：required
- `Enable`
- `WebsiteSSLID`
- `Type`：required，允许值 `existed` / `auto` / `manual`
- `PrivateKey`
- `Certificate`
- `PrivateKeyPath`
- `CertificatePath`
- `ImportType`
- `HttpConfig`：required，允许值 `HTTPSOnly` / `HTTPAlso` / `HTTPToHTTPS`
- `SSLProtocol[]`
- `Algorithm`
- `Hsts`
- `HstsIncludeSubDomains`
- `HttpsPorts[]`
- `Http3`

`response.WebsiteHTTPS`
- `Enable`
- `HttpConfig`
- `SSL`
- `SSLProtocol`
- `Algorithm`
- `Hsts`
- `HstsIncludeSubDomains`
- `HttpsPorts`
- `HttpsPort`
- `Http3`

`frontend/src/views/website/website/config/basic/https/index.vue` 的默认值：
- `httpConfig = HTTPToHTTPS`
- `hsts = true`
- `hstsIncludeSubDomains = false`
- `SSLProtocol = ['TLSv1.3', 'TLSv1.2']`
- `httpsPort = '443'`
- `http3 = false`
- `algorithm` 预置一整串默认 cipher suite

### 3.6 Cache drawer（前端 UI）

`WebsiteCacheConfig`（前端接口层）
- `open`
- `cacheLimit`, `cacheLimitUnit`
- `shareCache`, `shareCacheUnit`
- `cacheExpire`, `cacheExpireUnit`

## 4. 实际调用链：网站列表 -> 创建/更新 -> 绑定域名 -> 配置代理

### 4.1 网站列表页

文件：`frontend/src/views/website/website/index.vue`

1. 页面初始化后调用 `searchWebsites(req)`。
2. `searchWebsites` 对应 `POST /websites/search`，返回分页结果。
3. 列表行上的“配置”按钮会进入站点详情页；“更新”只修改基础信息，不会碰域名/代理/HTTPS。
4. 创建按钮会打开创建抽屉 `create/index.vue`。

### 4.2 创建网站

文件：`frontend/src/views/website/website/create/index.vue`

1. 先做表单校验：
   - deployment + new app：校验 install 子表单
   - runtime 且不是 php：`port` 不能为 0
   - stream：校验 stream 子表单
2. 调用 `preCheck({})`，对应 `POST /websites/check`。
   - 如果返回数据，说明有冲突/依赖问题，前端会把 items 交给预检弹窗处理
   - 如果没有数据，继续提交
3. 提交前会做一次 payload 归一化：
   - `type === 'proxy'` 时，把 `proxyProtocol + proxyAddress` 拼成 `proxy`
   - 如果 `enableFtp` 为 false，就清空 `ftpUser` / `ftpPassword`
   - 如果 `type === 'stream'`，把 stream 表单里的 `name` / `algorithm` / `servers` 写回主表单
4. 生成 `taskID = uuidv4()`，写入 `website.value.taskID`
5. 调用 `createWebsite(website.value)`，对应 `POST /websites`
6. 创建成功后关闭弹窗，并打开任务日志

后端 `CreateWebsite` 的关键行为：
- alias 不能是 `default`
- 中文 alias 会先做 punycode 编码
- 会检查 alias 重复
- 非 stream 类型会先解析 `domains[]`
- stream 类型要求 `streamPorts`，并把该值作为流站点的端口集合
- 创建过程还会走 task 子任务、DB 创建、目录/配置写入等流程

### 4.3 站点详情页

文件：`frontend/src/views/website/website/config/index.vue`

1. 通过 `getWebsite(id)` 读取站点详情。
2. 详情页再按 Basic / Log / Resource 等子页分发数据。
3. Basic 页是核心配置页，按类型展示不同 tabs：
   - static/runtime：域名、站点路径、默认文档、连接限制、代理、负载均衡、基础认证、CORS、HTTPS、真实 IP、重写、防盗链、重定向、PHP、资源、其他
   - stream：只保留与 stream 相关的 tab

### 4.4 绑定域名

文件：`frontend/src/views/website/website/config/basic/domain/index.vue`

1. 页面加载时调用 `listDomains(id)`，对应 `GET /websites/domains/:websiteId`。
2. 创建域名时调用 `createDomain({ websiteID, domains: [...] })`，后端批量写入。
3. 切换域名 SSL 开关时调用 `updateDomain({ id, ssl })`。
4. 删除域名时调用 `deleteDomain({ id })`。
5. UI 约束：
   - `port == 80` 时 SSL switch 不能改
   - 只有一个域名时删除按钮禁用
6. 打开域名时，URL 拼接逻辑与协议/端口有关：
   - HTTP：非 80 端口追加端口
   - HTTPS：非 443 端口追加 HTTPS 端口
   - 如果域名记录自身是非 80 端口，页面会退回到 `http://domain:port`

后端 `CreateWebsiteDomain` / `DeleteWebsiteDomain` 的副作用很关键：
- 会异步处理防火墙端口
- 会同步更新 WAF `sites.json`
- 会修改 nginx `listen/server_name`
- 删除时如果只剩最后一个域名，会直接报错，不允许删

### 4.5 配置代理

文件：`frontend/src/views/website/website/config/basic/proxy/index.vue` 和 `.../proxy/create/index.vue`

1. 代理列表页调用 `getProxyConfig({ id })`，对应 `POST /websites/proxies`。
2. 代理配置文件来源不是数据库，而是站点 `proxy/*.conf` / `proxy/*.bak` 的解析结果。
3. 创建/编辑时走统一入口 `operateProxyConfig(proxy)`，对应 `POST /websites/proxies/update`。
4. 前端在提交前会：
   - 把 `proxyProtocol + proxyAddress` 拼回 `proxyPass`
   - 把 `replaces[]` 归并成 map
   - 把浏览器缓存/服务器缓存 UI 状态转为后端字段
5. 启停代理时调用 `updateProxyConfigStatus({ id, name, status })`。
   - `disable`：`name.conf -> name.bak`
   - `enable`：`name.bak -> name.conf`
6. 删除代理时调用 `deleteProxyConfig({ id, name })`，会同时删 `.conf` 和 `.bak`。
7. 编辑代理源文件时调用 `updateProxyConfigFile({ websiteID, name, content })`。
   - 失败会回滚原内容
8. `proxy cache` drawer 使用 `getCacheConfig(id)` / `updateCacheConfig(req)`。
9. `清空 proxy cache` 调用 `clearProxyCache({ websiteID })`，只清 cache 目录并 reload，不等于关闭缓存。

后端 `OperateProxy` 的行为可以直接理解为：
- `create` / `edit`：改 include 目录下的 nginx 文件并更新站点主 nginx include
- `enable` / `disable`：改文件后缀再 reload
- `delete`：删文件再 reload

### 4.6 HTTPS

文件：`frontend/src/views/website/website/config/basic/https/index.vue`

1. 页面加载时调用 `getHTTPSConfig(id)`。
2. 页面默认把 HTTPS 表单预设成一套可用的常见值：
   - HTTPToHTTPS
   - TLSv1.3 / TLSv1.2
   - HSTS 默认开启
   - HTTPS 端口默认 443
3. 保存时调用 `updateHTTPSConfig(form)`。
4. 关闭已经开启的 HTTPS 时，前端会弹确认框；后端会清理 SSL 相关 nginx 配置，并把站点协议切回 HTTP。

## 5. 具体可自动化的部分 vs 需要人工确认的部分

### 可以自动化

- 网站列表拉取、站点详情拉取、基础信息更新
- 创建网站 payload 组装与提交
- 已知域名集合的批量绑定/解绑
- 代理配置的创建、编辑、启停、删除、文件编辑
- proxy cache 的读写和清理
- HTTPS 配置读取与更新
- taskID 串联日志

### 需要人工确认

- `preCheck` 返回的冲突项、openresty 安装/运行状态
- 域名端口与协议选择，尤其是非 80 / 443 的情况
- SSL 证书来源、证书是否可复用、是否允许关闭 HTTPS
- proxyPass、SNI、SSL verify、CORS、浏览器缓存、server cache 的具体策略
- 任何直接影响线上流量的破坏性操作：
  - 删除最后一个域名
  - 删除代理
  - 启停代理
  - 清空 proxy cache
  - 关闭 HTTPS

### 建议的自动化边界

如果 `ashan-frp` 已经有明确的站点拓扑和配置源，上面的 API 基本可以端到端自动执行；
如果缺少域名、端口、证书、缓存策略里的任意一个关键输入，就应该停在确认层，而不是猜默认值。

## 6. 对 `ashan-frp` 的映射建议

1. 站点主对象、域名集合、代理 include 文件、cache 配置、HTTPS 配置应拆成独立状态源。
2. 代理状态应以文件 include / 文件后缀为准，不要只做数据库字段。
3. proxy cache 需要同时具备“写配置”和“反解析配置”的能力。
4. 域名删改要把防火墙 / WAF / nginx 的副作用纳入同一条任务链。
5. HTTPS 的“关闭”与“清缓存”都不是纯表单字段切换，它们都会触发 nginx / openresty 行为变化。
6. 如果要做批量化编排，推荐流程是：
   - 先完成网站创建和预检
   - 再批量绑定域名
   - 再创建/编辑代理和 cache
   - 最后补 HTTPS

## 7. 参考文件

后端：
- `/opt/data/cache/1Panel/agent/app/api/v2/website.go`
- `/opt/data/cache/1Panel/agent/app/api/v2/website_domain.go`
- `/opt/data/cache/1Panel/agent/app/dto/request/website.go`
- `/opt/data/cache/1Panel/agent/app/dto/response/website.go`
- `/opt/data/cache/1Panel/agent/app/service/website.go`
- `/opt/data/cache/1Panel/agent/app/service/website_domain.go`
- `/opt/data/cache/1Panel/agent/app/service/website_proxy.go`

前端：
- `/opt/data/cache/1Panel/frontend/src/api/modules/website.ts`
- `/opt/data/cache/1Panel/frontend/src/api/interface/website.ts`
- `/opt/data/cache/1Panel/frontend/src/views/website/website/index.vue`
- `/opt/data/cache/1Panel/frontend/src/views/website/website/create/index.vue`
- `/opt/data/cache/1Panel/frontend/src/views/website/website/config/index.vue`
- `/opt/data/cache/1Panel/frontend/src/views/website/website/config/basic/domain/index.vue`
- `/opt/data/cache/1Panel/frontend/src/views/website/website/config/basic/domain/create/index.vue`
- `/opt/data/cache/1Panel/frontend/src/views/website/website/config/basic/proxy/index.vue`
- `/opt/data/cache/1Panel/frontend/src/views/website/website/config/basic/proxy/create/index.vue`
- `/opt/data/cache/1Panel/frontend/src/views/website/website/config/basic/proxy/cache/index.vue`
- `/opt/data/cache/1Panel/frontend/src/views/website/website/config/basic/https/index.vue`
