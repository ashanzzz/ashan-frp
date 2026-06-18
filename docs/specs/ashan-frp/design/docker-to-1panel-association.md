# Docker→1Panel 一键关联流程设计

> 适用范围：`ashan-frp` 的 Docker 容器到 1Panel 网站关联编排。
> 约束：仅基于研究文档 `docs/specs/ashan-frp/research/docker-port-mapping.md`、`docs/specs/ashan-frp/research/1panel-website-api.md`、`docs/specs/ashan-frp/research/1panel-https-port.md`；不包含实现代码。

## 1. 目标

把“发现一个 Docker 容器 → 为其选择合适映射 → 创建或更新 1Panel 网站 → 绑定域名/代理 → 配置 HTTPS → 持久化关联关系”串成一条可重复执行、可回收、可人工接管的流程。

这个流程的设计重点不是“能创建一次网站”，而是：

- 同一个容器反复扫描不会重复建站
- 容器重建后能按稳定键恢复原站点
- 端口冲突时不会静默覆盖现有站点
- HTTPS 非 443 场景要明确区分“展示值”和“真实安装端口”
- 任一步失败后都能回到可重试状态，且不污染后续同步

## 2. 设计输入与约束

### 2.1 Docker 侧输入

研究结果已经明确：

- `ContainerHelper.exposedPorts` 是结构化端口主来源
- `Port` 里有 `hostIP`、`hostPort`、`containerPort`、`protocol`
- `ContainerInfo.ports` 只适合回读校验，不适合作为主数据
- `ContainerInfo.network` 提供容器网络 IP 候选，可优先用于反代 target
- `publishAllPorts=true` 时，`hostPort` 可能启动后才稳定，不能提前固化

因此，本流程的“容器发现”必须围绕结构化端口和网络信息展开，不能只看字符串展示值。

### 2.2 1Panel 侧输入

研究结果已经确认 1Panel website 模块的能力分层：

- 网站主流程：`/websites/search`、`/websites/list`、`/websites/:id`、`/websites/check`、`/websites`、`/websites/update`
- 域名：`/websites/domains/:websiteId`、`/websites/domains`、`/websites/domains/update`、`/websites/domains/del`
- 代理：`/websites/proxies`、`/websites/proxies/update`、`/websites/proxies/status`、`/websites/proxies/delete`、`/websites/proxies/file`
- proxy cache：`/websites/proxy/config/:id`、`/websites/proxy/config`、`/websites/proxy/clear`
- HTTPS：`/websites/:id/https` 的读取与更新

同时，研究也明确：

- 域名、代理、HTTPS、cache 都不是单纯数据库状态
- 域名操作会联动 nginx / WAF / 防火墙
- 代理启停是 `.conf` 与 `.bak` 的互换
- HTTPS 展示端口 `HttpsPort` 是派生值，不是网站表里的独立持久化列
- 非 443 HTTPS 的真实来源是 OpenResty 安装端口 `AppInstall.HttpsPort`

## 3. 核心对象

### 3.1 业务对象

本设计只定义逻辑对象，不要求新增具体实现结构：

- 容器：Docker 运行实例
- 映射候选：某个 `containerPort + protocol` 对应的端口记录
- 网站：1Panel 中承载域名、代理、HTTPS 的站点对象
- 关联记录：把“某容器的某个映射候选”绑定到“某个 1Panel 网站”的持久化关系

### 3.2 两类标识

为了避免重建抖动，建议逻辑上区分两个键：

- 稳定业务键：`containerName + containerPort + protocol`
- 运行时实例键：`containerID + hostIP + hostPort + containerPort + protocol`

用途：

- 稳定业务键：判断是不是“同一个业务映射”
- 运行时实例键：判断是不是“当前 Docker 实例替换了”

### 3.3 网站命名规则

研究结论建议：

- 单端口容器：网站名可直接用容器名
- 多端口容器：网站名要带 `-<containerPort>-<protocol>` 后缀
- 任何时候都不要把 `hostPort` 放进基础名称里，因为它会漂移

域名也应使用同一套 canonical slug：

- 单端口：`container-name.example.com`
- 多端口：`container-name-80-tcp.example.com`

如果已有人工指定域名，则自动流程应视为人工覆盖，不主动改写。

## 4. 一键关联的完整步骤

下面是必须遵循的精确步骤顺序。

### Step 1: discover container

用户可见动作：

- 在容器列表中选择目标容器
- 系统展示该容器可关联的端口与网络信息

后台动作：

1. 读取容器的结构化端口集合 `exposedPorts`
2. 读取容器网络 `network`
3. 回读 `ports` 只做核验，不作为主判断依据
4. 如果 `publishAllPorts=true` 且 `hostPort` 尚不稳定，标记该映射为 pending / unresolved

输出给用户的候选项至少应包括：

- 容器名
- `containerPort`
- `protocol`
- `hostIP`
- `hostPort`（若已稳定）
- 容器网络 IPv4 候选

### Step 2: choose mapping

用户可见动作：

- 选择一个或多个端口映射
- 选择是否沿用已有网站
- 选择是否采用自动域名、自动代理、自动 HTTPS

后台动作：

1. 对每个候选映射构建稳定业务键
2. 按规则排序并去重
3. 判断是否已有同键关联
4. 识别公共监听冲突 `(hostIP, hostPort, protocol)`
5. 把可自动执行与需人工确认的项分离

选择规则：

- 只要 `containerPort` 或 `protocol` 不同，就视为不同映射
- 如果同一个容器暴露多个端口，每个 `Port` 都是独立映射单元
- `hostPort` 不参与业务身份，只参与回读和冲突判定

### Step 3: create/update website

用户可见动作：

- 系统显示“创建网站”或“更新已有网站”
- 若发生冲突，提示用户是否进入人工处理

后台动作：

1. 若不存在关联网站：先走 `POST /websites/check` 预检
2. 无冲突后创建网站 `POST /websites`
3. 若已存在关联网站：走 `POST /websites/update` 更新基础信息
4. 网站基础字段只更新研究中明确属于基础更新的项，避免误触域名/代理/HTTPS 的独立状态

这里的关键原则是：

- 同一个稳定业务键只对应一个网站对象
- 如果容器重建但稳定业务键未变，优先更新已有网站，而不是新建重复网站
- 如果容器名变了，则视为新业务对象，旧记录进入归档/失效处理

### Step 4: bind domain/proxy

用户可见动作：

- 选择自动生成或手工输入域名
- 选择是否启用反向代理
- 系统提示端口/协议是否会影响域名访问方式

后台动作：

1. 域名：调用 `POST /websites/domains` 批量创建或绑定
2. 域名开关：需要时调用 `POST /websites/domains/update`
3. 域名删除：调用 `POST /websites/domains/del`
4. 代理：调用 `POST /websites/proxies/update` 创建/编辑/删除/启停
5. 代理启停本质是 `.conf` / `.bak` 切换后重载 nginx/openresty
6. proxy cache 如需同步，调用 `GET/POST /websites/proxy/config` 系列接口

target 选择规则：

- 优先：容器网络 IPv4 + `containerPort`
- 退回：`hostIP + hostPort`
- 再退回：宿主机本地可达地址 + `hostPort`

域名与代理的绑定中，人工指定值优先级更高：

- 已有人为指定的 domain，自动同步只更新 target，不主动改 domain
- 代理文件如果已存在人工编辑痕迹，自动流程应只做最小化同步

### Step 5: configure HTTPS

用户可见动作：

- 打开或关闭 HTTPS
- 看到 HTTPS 展示端口
- 必要时选择证书来源

后台动作：

1. 读取 `GET /websites/:id/https`
2. 以网站域名和 OpenResty 安装端口推导展示端口
3. 更新 `POST /websites/:id/https`
4. 触发站点协议、证书、HSTS、HTTP→HTTPS 跳转、nginx 配置写回
5. 重载 nginx/openresty

重要约束：

- `HttpsPort` 是展示值，不是网站表里的独立持久化列
- 真正决定非 443 HTTPS 的，是 OpenResty 安装实例的 `AppInstall.HttpsPort`
- 非 443 场景下，HTTP→HTTPS 重定向必须显式保留端口
- HTTP/3 的 `Alt-Svc` 仍然硬编码为 `:443`，因此非 443 + HTTP/3 是已知风险点，设计上应默认禁用或要求人工确认

### Step 6: persist linkage

用户可见动作：

- 系统提示“关联完成”
- 用户之后可在同一容器上重复执行，不会重新建一套无关对象

后台动作：

1. 保存“容器稳定业务键 ↔ 网站 ID ↔ 目标映射 ↔ 域名 ↔ HTTPS 状态”的持久化关联
2. 保存当前运行时指纹，便于后续识别容器重建
3. 保存自动化结果与人工覆盖痕迹的分离状态
4. 回读 `ContainerInfo.ports`、网站详情、域名列表、HTTPS 展示值做最终核验

持久化重点不是把所有状态都写成一张表，而是把“可自动重建的关系”与“人工覆盖的决策”分开存。

## 5. 用户可见动作 vs 后台作业

### 用户可见动作

- 选择容器
- 选择端口映射
- 确认是否沿用已有网站
- 确认域名与端口
- 确认是否启用反代、缓存、HTTPS
- 处理冲突提示
- 处理证书/端口/破坏性操作确认框

### 后台作业

- 拉取容器结构化端口与网络信息
- 生成 canonical key / runtime key
- 预检冲突
- 创建或更新网站
- 创建/更新域名
- 写入或切换代理文件
- 更新 proxy cache 配置
- 写回 HTTPS 配置
- 重载 nginx/openresty
- 回读并核验最终状态
- 持久化关联和人工覆盖标记

原则上，只有“选择”和“破坏性确认”需要用户面对界面，其余都应该由后台编排完成。

## 6. 冲突处理与人工覆盖规则

### 6.1 公共监听冲突

判定条件：

- 同一个 `(hostIP, hostPort, protocol)` 只能有一个有效映射

处理方式：

- 这是硬冲突
- 不能自动踢掉老映射
- 新映射进入 pending / conflict 状态
- UI 必须要求人工决策

### 6.2 业务身份冲突

判定条件：

- `containerPort + protocol` 相同，但 `hostPort` 不同

处理方式：

- 通常视为绑定方式变化，不视为新业务出现
- 如果容器名相同，应更新已有网站，不新建重复站点

### 6.3 多端口冲突

判定条件：

- 同容器多个端口同时存在
- 或同容器出现重复端口记录

处理方式：

- 不同 `containerPort`：分别作为独立映射
- `containerPort` 相同但 `protocol` 不同：保留为不同映射
- 重复记录先去重再同步

### 6.4 人工覆盖规则

以下内容一旦有人手工指定，自动流程默认不覆盖：

- domain
- proxy 配置中的人工内容
- HTTPS 关闭/启用决策
- SSL、证书、HSTS、HTTP3 等破坏性或高风险设置

自动同步的职责是“保持一致”，不是“替人改意图”。

## 7. 幂等、重试与重建后的 reconciliation

### 7.1 幂等规则

每次同步都要先构建 desired 集合，再与 actual 集合比较：

- `desired ∩ actual` 且 target 相同：no-op
- `desired ∩ actual` 但 target 改变：更新已有记录，不新建
- `desired - actual`：创建新记录
- `actual - desired`：删除或标记失效，取决于保留策略

### 7.2 去抖规则

为了避免 Docker 返回顺序变化导致抖动，比较前必须：

- 端口数组排序后再比较
- 重复端口先去重再比较
- 网络数组先选定优先网络再比较
- 不要让 `containerID` 变化自动触发改名

### 7.3 容器重建后的恢复

如果容器名不变但 `containerID` 变了：

- 视为同一个业务对象的实例替换
- 保持网站名和 domain 不变
- 仅更新 target 和运行时指纹

如果容器名变了：

- 视为新业务对象
- 按新 canonical key 建立新关联
- 旧记录按删除/归档策略处理

### 7.4 重试语义

- 可重试：读取、预检、创建站点前的准备、域名/代理/HTTPS 的幂等更新
- 需人工介入：公共端口冲突、最后一个域名删除、关闭 HTTPS、清空 cache、代理源文件直接改写失败等

重试时，系统必须先做回读核验，再决定是继续更新已有对象还是回滚到人工待处理状态。

## 8. 回滚与失败恢复

### 8.1 失败分层

失败可分为四层：

1. 容器发现失败
2. 网站对象创建/更新失败
3. 域名 / 代理 / cache 失败
4. HTTPS 配置失败

每层的失败都不应污染前一层已经成功的状态。

### 8.2 回滚原则

- 创建网站后若域名绑定失败，已创建的网站可保留，但关联状态应标记为未完成
- 域名创建后若代理失败，不自动删除已成功的域名，除非用户明确要求回滚
- HTTPS 配置失败后，不要假定网站已经成功切换到目标协议，必须重新读取站点配置确认真实状态
- 代理编辑失败时，若原内容已回滚，应以回读结果为准，不以请求结果为准

### 8.3 失败后的状态机建议

建议至少区分：

- `pending`：等待继续执行
- `unresolved`：字段不全或 `hostPort` 未稳定
- `conflict`：公共监听冲突
- `linked`：关联完成
- `partial`：部分步骤成功，需恢复
- `manual`：人工接管
- `failed`：不可自动恢复，需要重新发起

### 8.4 恢复顺序

恢复时应按以下顺序回查：

1. 网站是否已存在
2. 域名是否已绑定
3. 代理文件是否落盘并启用
4. HTTPS 是否已真正启用
5. 关联记录是否已经持久化

这样可以最大限度避免重复创建和状态漂移。

## 9. 预检与默认值策略

### 9.1 预检

在创建或更新之前，必须执行冲突检查：

- 网站 alias 是否冲突
- 域名端口是否允许
- 是否存在公开监听端口冲突
- OpenResty / app install 的 HTTPS 端口是否可用
- 是否触发已知高风险路径（例如非 443 + HTTP/3）

### 9.2 默认值

可自动填充的默认值只限于研究文档明确提到的常见默认行为，例如：

- HTTPS 默认 `HTTPToHTTPS`
- HSTS 默认开启
- SSL 协议默认 `TLSv1.3` / `TLSv1.2`
- HTTPS 端口展示值默认 443

但默认值只能用于 UI 初始化，不能覆盖人工输入或硬冲突判断。

## 10. 最终约束

1. 不把 `hostPort` 当身份
2. 不把 `ContainerInfo.ports` 当主数据
3. 不把 `HttpsPort` 当网站表持久化列
4. 不在公共监听冲突时自动覆盖旧映射
5. 不在已有人工 domain 上强行改写自动域名
6. 不把非 443 + HTTP/3 当作安全默认
7. 不把一次成功的操作误当成永久成功，必须回读核验

## 11. 交付边界

本设计只定义流程、状态、冲突和恢复策略，不包含：

- 代码实现
- 前端交互细节代码
- 数据库迁移脚本
- API 封装代码

如果后续进入实现阶段，建议先把“稳定键 / 关联状态 / 冲突状态”三类模型落到文档和数据结构上，再拆前端与后台编排。
