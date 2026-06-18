# ChmlFrp 自动切换 / 隧道同步领域模型

本文把 `chmlfrp.sh`、`new_fix_flow.sh`、状态文件和 OAuth2 认证统一抽象成一套可执行的领域模型。目标不是复述脚本实现细节，而是回答：系统里有哪些核心对象、谁是事实源、哪些状态可自动化、哪些必须人工确认，以及哪些旧行为要保留、哪些要明确废弃。

## 1. 先给结论

1. 这套系统的核心不是“一个隧道列表”，而是四层对象：
   - 期望态：固定隧道定义
   - 运行态：当前隧道快照、当前节点、节点快照
   - 判定态：健康状态、封禁状态、同步结果、节点问题
   - 执行态：创建、删除、改名、重建、重启、OAuth 刷新 / 重新授权
2. `chmlfrp.sh` 是控制器，只负责判断与调度：健康检查、故障切换、择优切换、reconcile、手动切换、节点刷新、userinfo 同步、OAuth 刷新 / 重新授权。
3. `new_fix_flow.sh` 是执行器，只负责真正落资源：读取固定隧道、做预检、生成同步计划、创建 / 重建隧道和 DNS、重启 frpc、写回同步结果。
4. 删除隧道只能算“尽力而为”的辅助路径，不能作为全量同步成立的唯一前提。
5. 状态文件和快照文件不是日志附件，而是控制器判断“是否换节点、是否 ban、是否需要人工介入”的事实来源。

## 2. 事实源、派生状态与决策记忆

| 资产 | 作用 | 语义 |
| --- | --- | --- |
| `fixed_tunnels.txt` | 期望态源文件 | 描述“应该存在什么” |
| `chmlfrp-fixed-tunnels-normalized.txt` | 规范化固定隧道 | 供执行器直接消费的 canonical 版本 |
| `chmlfrp-tunnels-raw.txt` | 运行态原始快照 | 面板 / API 返回的真实资源视图 |
| `chmlfrp-tunnels-normalized.txt` | 运行态规范化快照 | 便于与固定定义做稳定比较 |
| `chmlfrp-nodes-all.txt` | 节点全集快照 | 原始节点目录 |
| `chmlfrp-nodes-filtered.txt` | 节点候选快照 | 过滤后可用于选点的集合 |
| `chmlfrp-node-refresh-state.txt` | 节点刷新状态 | 记录上次刷新时间、文件路径和规模变化 |
| `chmlfrp-health-status.txt` | 健康状态 | 控制器当前判断 |
| `chmlfrp-ban-state.txt` | 封禁状态 | 记录短 ban / 长 ban / 冷却 |
| `chmlfrp-node-issues.txt` | 节点问题记录 | 追加式诊断账本 |
| `chmlfrp-sync-result.txt` | 同步结果 | 执行器与控制器之间最重要的回传对象 |
| `chmlfrp-sync-issues.json` | 同步问题记录 | 记录具体异常分类和细节 |
| `chmlfrp-source-snapshot.txt` | 源文件指纹缓存 | 记录 fixed / exempt 文件的 mtime + hash |
| `chmlfrp-userinfo.txt` | 用户详情缓存 | 登录 / userdetail 的 JSON 快照 |
| `userdata.txt` | 认证与平台配置 | 兼容旧 token，也承载 OAuth2 配置 |

要点：
- 期望态与运行态永远不完全等价，不能用当前快照直接替代固定配置。
- `sync result`、`ban state`、`node issues` 不是纯日志，而是决策记忆。
- `source snapshot`、`node refresh state` 这类文件的价值在于减少重复执行和抖动。

## 3. 核心领域实体

### 3.1 FixedTunnelDefinition（固定隧道定义）

固定隧道定义是系统最重要的期望态对象。

它表示“最终应该存在的资源”，而不是“当前面板里已经有什么”。执行器只认规范化后的固定定义，不认历史临时状态。

### 3.2 CurrentTunnelSnapshot（当前隧道快照）

当前隧道快照来自面板 API 或本地缓存抓取结果，表示“现实里已经存在什么”。

它能告诉控制器：
- 当前节点上实际挂了哪些隧道
- 隧道名是否与固定定义偏离
- 是否需要 reconcile 或重建

它不是真理，只是一次观察。

### 3.3 NodeSnapshot（节点快照）

节点快照表示可用节点全集的观测结果，通常来自 `/node` 接口以及后续的 `nodeinfo` 详情补充。

它包含在线状态、IP、地区、权限组、建站能力、备注和 ping 可达性等信息。

### 3.4 NodeCandidate（候选节点）

候选节点是从节点快照里进一步筛选出来的“可尝试切换对象”。

候选节点必须满足：
- 在线
- 未被 ban
- 未处于冷却窗口
- ping 可达且质量足够
- 不与当前资源状态发生明显冲突

候选节点不是最终保证可用的节点，只是更值得尝试的节点。

### 3.5 CurrentNode（当前节点）

当前节点是由 `frpc.toml` 的 `server_addr` 反查出来的运行落点。

常见解析顺序：
1. 本地节点缓存
2. 当前隧道快照 / 节点快照
3. `nodeinfo` 或隧道列表 API 反查

它决定：
- 当前是否需要 reconcile
- 当前是否需要切到别的节点
- 切换是否真的成功

### 3.6 HealthState（健康状态）

健康状态至少分两条轴：
- 业务健康：`online` / `offline` / `degraded`
- 代理健康：`ok` / `unknown` / `degraded` / 冲突类原因

这两个轴必须拆开，因为：
- 业务在线，不代表代理参数没有错
- 代理冲突，不一定意味着整台节点离线
- 某些配置类问题应停止自动切换，转人工修正

### 3.7 SyncResult（同步结果）

同步结果是执行器向控制器回传的决策输入。

典型字段：
- `status`
- `classification`
- `message`
- `node`
- `detail`
- `ts`

它不是普通日志，而是“下一步该 ban 谁、该换节点还是该人工修”的依据。

### 3.8 BanState（封禁状态）

封禁状态表示某个节点在一段时间内不应再次尝试。

典型结构：
- `banned[]`
- 每个条目含 `name`、`reason`、`classification`、`detail`、`until`

短 ban 用于避免短期抖动；长 ban 用于屏蔽“对这类资源模型不友好”的节点。

### 3.9 OAuthState / UserIdentity（认证状态）

认证态决定系统能否访问受保护接口。

它既包含旧 token 时代的 `chmlfrp.token`，也包含 OAuth2 模式下的：
- `enabled`
- `client_id`
- `client_secret`
- `access_token`
- `refresh_token`
- `token_expires_at`

认证态是横切依赖，不是资源冲突。

## 4. 固定隧道记录语法

### 4.1 规范化输入示例

```json
[
  {
    "name": "example_https",
    "tunnel_local_ip": "192.168.1.10",
    "tunnel_local_port": "30001",
    "tunnel_type": "https",
    "tunnel_remote_port": "",
    "dns_domain_cname": "app",
    "dns_proxied": false
  },
  {
    "name": "example_tcp",
    "tunnel_local_ip": "192.168.1.10",
    "tunnel_local_port": "22",
    "tunnel_type": "tcp",
    "tunnel_remote_port": "40022",
    "dns_domain_cname": "ssh",
    "dns_proxied": false
  }
]
```

### 4.2 语法规则

固定隧道文件的 canonical 语法是 JSON 数组，每个元素至少应提供：
- `name`
- `tunnel_local_ip`
- `tunnel_local_port`
- `tunnel_type`
- `dns_domain_cname`
- `dns_proxied`

协议相关规则：
- `tunnel_type` 取值通常是 `http`、`https`、`tcp`、`udp`
- `tcp` / `udp` 必须显式提供 `tunnel_remote_port`
- `http` / `https` 可以不填 `tunnel_remote_port`
- 远程端口不再自动随机分配

### 4.3 派生字段与标准化规则

执行器不会把原始名字直接当成稳定身份，而是会生成运行名（runtime name）。

标准化规则：
- 小写化
- 非法字符替换为 `-`
- 合并多余的 `-`
- 去掉首尾多余分隔符
- 删除尾部数字后缀及其相邻分隔符
- 结果为空时回退为 `tunnel`

这意味着：
- `runtime_name` 是派生字段，不是输入事实
- 稳定比较依赖标准化后的名字，而不是原始字符串
- 名字后缀中的数字不应参与固定身份判断

### 4.4 兼容性边界

当前执行器只消费 canonical 字段集。

历史样本里可能出现 `tunnel_status`、`dns_status`、或其他注释性字段，但它们不属于新的规范字段，不应作为固定身份的一部分，也不应作为执行器的输入契约。

换句话说：
- 文件名兼容可以保留
- 字段兼容不再保留

## 5. 节点、隧道、健康、封禁、刷新与同步的语义

### 5.1 隧道快照语义

隧道快照分两种：
- 原始快照：面板 / API 返回的直接结果
- 规范化快照：按固定规则整理后的可比较结果

比较时重点看：
- 规范化隧道名
- 本地 IP / 端口
- 协议类型
- 远程端口
- DNS 域名与是否代理

`tunnel_id_by_name()`、`tunnel_name_exists_in_remote_state()` 这类逻辑都基于标准化后的名字比较，而不是原始字符串。

### 5.2 节点快照与候选选择语义

节点刷新有单独的节流语义：
- `NODE_REFRESH_SECONDS` 默认 3600 秒
- `node_refresh_needed()` 决定是否重新拉取节点列表
- `chmlfrp-node-refresh-state.txt` 记录上一次刷新时间和结果

节点筛选的核心不只是“在线”，还包括质量评估：
- `nodeinfo` 必须返回可用详情
- 节点必须在线
- ping 必须通过
- 评分公式基于平均延迟和丢包率

当前实现的候选评分：
- `score = avg_ms + loss_pct * 30`
- 分数越低越优

### 5.3 当前健康状态语义

`health_check()` 写入的状态文件是一个 JSON：

```json
{
  "status": "online",
  "reason": "ok",
  "details": "container running; config ok; probes ok",
  "proxy_status": "ok",
  "proxy_reason": "ok",
  "ts": 1234567890
}
```

关键点：
- `status` 代表业务健康
- `proxy_status` 代表代理 / 配置健康
- 两者可以不同

常见原因：
- `docker_missing`
- `container_missing`
- `container_not_running`
- `frpc_config_missing`
- `frpc_config_invalid`
- `tcp_connect_fail`
- `http_healthcheck_fail`
- `server_connect_fail`
- `config_mismatch`
- `server_node_conflict`

规则：
- `config_mismatch` 是配置类问题，应停止自动切换
- `server_node_conflict`、`node_unusable`、`router_conflict`、`node_proxy_conflict`、`node_proxy_already_exists` 通常意味着需要换节点

### 5.4 封禁语义

封禁不是惩罚，而是保护系统不重复踩同一个坑。

默认策略：
- `BAN_SECONDS` 默认 3600 秒，用于短 ban
- `NODE_UNUSABLE_BAN_SECONDS` 默认 2592000 秒，用于长 ban

`ban_node()` 与 `ban_node_long()` 的差异：
- 短 ban：常规失败后的短期冷却
- 长 ban：节点级不可用、路由冲突、代理冲突等长期问题

`chmlfrp-node-issues.txt` 是追加式诊断账本；`chmlfrp-ban-state.txt` 是可执行的封禁状态。

### 5.5 刷新语义

系统里至少有两种“刷新”：

1. 节点刷新
   - 依据 `NODE_REFRESH_SECONDS`
   - 通过 `chmlfrp-node-refresh-state.txt` 判断是否过期
   - 目标是更新候选节点集合

2. 源文件刷新
   - 依据 `chmlfrp-source-snapshot.txt`
   - 通过 fixed 文件和 exempt 文件的 mtime + hash 判断是否变化
   - 目标是判断 fixed 隧道是否需要重新同步

另外，在线测试文件也参与预检：
- 如果 `chmlfrp-health-status.txt` 里的状态是 `online`
- 但文件时间戳超过 900 秒
- 则视为过期，不能继续当成可信的“在线证明”

### 5.6 同步结果语义

`chmlfrp-sync-result.txt` 不是单纯日志，而是“本次执行的裁决书”。

典型 classification：
- `node_proxy_conflict`
- `node_proxy_already_exists`
- `node_unusable`
- `router_conflict`
- `name_conflict_deleted`
- `name_conflict_renamed`
- `name_conflict_retry_failed`
- `fix_flow_failed`
- `post_switch_offline`
- `config_mismatch`
- `server_node_conflict`

控制器会根据这些分类决定：
- 是否长 ban 当前节点
- 是否继续换节点
- 是否停止自动化并要求人工介入

## 6. 控制器流转：periodic sync 与 health 语义

`chmlfrp.sh` 的核心流程可以理解为：

1. 读取或刷新配置与缓存
2. 必要时刷新节点列表
3. 先对当前节点做 reconcile / 同步检查
4. 再检查健康状态与代理状态
5. 根据健康与同步结果决定：继续、切换、ban、还是人工介入

### 6.1 `reconcile` 的语义

`reconcile_current_node()` 的语义是：
- 不主动选新节点
- 只对当前节点做一次固定隧道同步检查
- 目的是消除“固定定义已经变了，但当前节点还没重建”的漂移

如果同步结果提示节点侧冲突，控制器会把它视为节点问题，而不是普通离线。

### 6.2 `failover` 的语义

`auto_failover()` 的行为：
- 即使节点还在线，也要先检查固定隧道是否变化
- 如果代理层有问题，先尝试对当前节点 reconcile
- 如果仍不行，再切换到其他候选节点
- 如果是配置类问题，停止自动切换，要求人工修正

这里有一个关键分岔：
- `status_requires_manual_fix(reason)` 只对配置类错误刹车
- `proxy_requires_switch(proxy_reason)` 则决定是否应该切换节点

### 6.3 `fastest` 的语义

`auto_fastest()` 的行为：
- 先尊重冷却窗口
- 再做健康与代理层判断
- 按评分选择最优节点
- 如果最优节点就是当前冲突节点，会先长 ban 再重新选点

### 6.4 `manual` 的语义

`manual_switch()` 用于用户明确知道目标节点时的直接切换：
- 允许参数指定节点名
- 也可从 `manual_node.txt` 读取
- 跳过自动择优

## 7. 执行器流转：new_fix_flow 的语义

`new_fix_flow.sh` 是真正的资源变更执行器。

### 7.1 输入与输出

输入：
- 固定隧道定义文件
- 可选的 exempt / 豁免文件
- 节点候选文件
- 用户详情文件 / OAuth2 配置
- frpc 配置
- 远端 API / Cloudflare DNS 状态

输出：
- `chmlfrp-fixed-tunnels-normalized.txt`
- `chmlfrp-tunnels-raw.txt`
- `chmlfrp-tunnels-normalized.txt`
- `cloudflare-dns-raw.txt`
- `cloudflare-dns-normalized.txt`
- `chmlfrp-sync-issues.json`
- `chmlfrp-sync-result.txt`
- `chmlfrp-source-snapshot.txt`

### 7.2 模式开关

`new_fix_flow.sh` 的几个重要开关：
- `--dry-run`：只打印计划，不真正调用 API / 删除 / 创建 / 重启 Docker
- `--force-run`：跳过 mtime / 在线测试 / frpc 日志等前置检查
- `--dns-only`：只同步 DNS，不动隧道，不重启 frpc
- `--clean-invalid`：只清理不在固定配置中的资源，不新建
- `--prefer-frpc-node`：优先沿用当前 frpc 对应节点（前提是 nodeinfo 在线且 ping 通过）
- `--node NAME`：显式指定目标节点

### 7.3 预检语义

预检的目标不是“省一步”，而是减少误操作。

预检包括：
- 固定隧道文件是否存在
- 豁免文件是否存在，不存在则按空文件处理
- fixed / exempt 文件的 mtime + hash 是否变化
- 在线测试文件是否仍然可信
- frpc 日志里是否出现配置类错误

如果：
- fixed / exempt 文件都没变
- 在线测试仍然有效
- frpc 日志没有配置错误

则可认为“预检通过”，后续再核对远端快照。

### 7.4 不再保留的执行假设

执行器明确不再做这些事：
- 不再为 `tcp` / `udp` 自动随机分配远程端口
- 不再把旧字段兼容当成执行器契约
- 不再把删除成功当作全量同步的核心前提

### 7.5 同步问题记录与结果回写

`record_sync_issue()` 与 `write_sync_result()` 的职责分开：
- `sync_issues` 记录具体异常证据
- `sync_result` 记录本次裁决与控制器可消费的结果

这让控制器可以根据分类做自动化决策，而不是去解析一整段文本日志。

## 8. OAuth2 与旧 token 的兼容语义

### 8.1 旧 token 路径的保留

`userdata.txt` 里仍可能存在旧时代的 `chmlfrp.token`。

这条路径必须保留，因为：
- 历史配置还在用
- 不是所有环境都已经切到 OAuth2
- 旧路径属于兼容层，不应因为新认证模式上线而立刻失效

### 8.2 OAuth2 路径

当 `oauth2.enabled=true` 时，系统按 OAuth2 语义运行：
- 读取 `client_id` / `client_secret`
- 使用 `access_token` 调用接口
- 依据 `token_expires_at` 判断是否快过期
- 必要时通过 `refresh_token` 刷新

默认缓冲：
- `TOKEN_EXPIRE_BUFFER` 为 60 秒

### 8.3 刷新与重新授权

`get_access_token()` 的行为：
1. 先看当前 token 是否存在
2. 若缺失，尝试刷新
3. 若已接近过期，也尝试刷新
4. 刷新失败时提示执行 `oauth_reauth`

`oauth_refresh`：
- 明确触发 refresh_token 刷新

`oauth_reauth`：
- 走 device code 授权流程
- 通过浏览器完成一次新的授权
- 再把新的 access / refresh token 写回配置

### 8.4 用户详情同步

`userinfo_sync()` 会使用 access token 去同步用户详情。

如果用户详情不存在或 JSON 不合法，会尝试自动同步；这一步是认证态的一部分，而不是资源同步的一部分。

### 8.5 日志与安全边界

Token 类字段只应被当作认证态，不应被当作业务实体。

日志里必须遮蔽 token，不应把真实 token 直接写出到持久化文档或控制台输出中。

## 9. 保留的旧行为 vs 明确废弃的旧行为

| 结论 | 保留 / 丢弃 | 说明 |
| --- | --- | --- |
| 控制器 / 执行器分离 | 保留 | `chmlfrp.sh` 负责判断，`new_fix_flow.sh` 负责落地 |
| 文件化状态机 | 保留 | `health`、`ban`、`refresh`、`sync result` 都以文件作为事实源 |
| 旧 `chmlfrp.token` 兼容 | 保留 | 仅在 OAuth2 未启用时继续有效 |
| `fixed_tunnels.txt` 兼容文件名 | 保留 | 兼容旧文件名 `chmlfrp固定隧道.txt`，但字段规范不再兼容旧 schema |
| `dry-run` / `force-run` | 保留 | 一个只看计划，一个强制绕过预检 |
| `dns-only` / `clean-invalid` | 保留 | 作为执行器的重要局部模式 |
| 节点冷却 / ban | 保留 | 需要防抖和防重复踩坑 |
| 删除隧道作为唯一同步前提 | 丢弃 | 删除只能是辅助路径，不是正确性的核心 |
| TCP / UDP 随机远程端口 | 丢弃 | 必须显式填写 `tunnel_remote_port` |
| 旧固定隧道字段兼容 | 丢弃 | canonical 语法只认新字段集 |
| 原始名字尾部数字参与身份判断 | 丢弃 | 标准化后会剔除尾部数字后缀 |
| “在线就无需重建” | 丢弃 | fixed 定义变化时，在线也要 reconcile |
| “删除成功就代表同步成功” | 丢弃 | 同步正确性不再依赖单次删除成功 |
| 仅靠日志判断一切 | 丢弃 | 需要结构化 `sync result` 与状态文件 |
| 过期在线测试文件仍可继续当作在线 | 丢弃 | 超过 900 秒就视为不可信 |

## 10. 观察样本说明

从最近的样本快照看，固定定义、当前隧道和当前节点可以在平稳态下保持一致，健康状态也可能是 `online`。

但这只是一次观察，不是永恒不变的事实。

这个模型的重点是：它能在状态漂移、节点冲突和认证过期的情况下，继续把固定隧道维持成可用状态。

## 11. 小结

ChmlFrp 的核心不是“会不会切节点”，而是“如何在不可靠删除接口和复杂节点质量差异下，持续把固定隧道维持成可用状态”。

所以，这个领域模型最重要的结论是：
- 固定隧道定义是目标
- 当前节点与快照是观察
- health / ban / refresh / sync result 是决策记忆
- `chmlfrp.sh` 负责判断，`new_fix_flow.sh` 负责落地
- OAuth2 是横切依赖，但旧 token 兼容必须保留

只要这几层分清，后续无论是补文档还是继续实现，路径都会更清晰。
