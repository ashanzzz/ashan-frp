# Docker 容器端口发现与冲突规则研究

## 结论先行

- `ContainerHelper.exposedPorts` 是这套 API 里唯一的结构化端口输入；`Port` 的 `hostIP`、`hostPort`、`containerPort`、`protocol` 才是可以直接用于同步的字段。
- `ContainerInfo.ports` 只是列表页的字符串展示/回读字段，适合做校验，不适合作为唯一来源。
- `publishAllPorts` 表示让 Docker 自动分配宿主机端口；在容器启动并回读之前，`hostPort` 可能为空或未知，因此不能提前生成稳定的网站映射。
- 多端口容器应该按“一个 `Port` 记录 = 一个映射单元”处理，而不是把整台容器当成单一端口对象。
- 冲突判断要分两层：公共监听层看 `(hostIP, hostPort, protocol)`，业务身份层看“容器名 + containerPort + protocol”；`containerID` 只做当前实例校验，不建议作为长期唯一键。

## 本次研究的可见数据面

这次只阅读了下面三份缓存文件，所以结论严格基于 API / DTO / 控制器契约，不包含 service 层里隐藏的 Docker inspect 组装逻辑：

- `/opt/data/cache/1Panel/frontend/src/api/modules/container.ts`
- `/opt/data/cache/1Panel/frontend/src/api/interface/container.ts`
- `/opt/data/cache/1Panel/agent/app/api/v2/container.go`

### 写入侧可见接口

`createContainer()` 和 `updateContainer()` 都使用 `Container.ContainerHelper`，说明容器端口配置的写入入口是统一的。

`ContainerHelper` 里和端口相关的字段：

- `hostname`
- `domainName`
- `publishAllPorts`
- `exposedPorts: Array<Port>`
- `networks: Array<ContainerNetwork>`
- `ports` 不存在于写入结构里，说明写入侧要依赖结构化 `exposedPorts`

`Port` 的字段：

- `host`
- `hostIP`
- `containerPort`
- `hostPort`
- `protocol`

### 读取侧可见接口

`loadContainerInfo(name)` 返回 `Container.ContainerHelper`，这意味着增量同步可以先按容器名取回结构化配置，再做差异比较。

`ContainerInfo` 里和端口/网络相关的字段：

- `ports: Array<string>`
- `network: Array<ContainerNetwork>`
- `isFromApp`
- `isFromCompose`

`ContainerNetwork` 的字段：

- `network`
- `ipv4`
- `ipv6`
- `macAddr`

### 对同步最重要的语义分层

| 字段 | 含义 | 同步时怎么用 |
| --- | --- | --- |
| `exposedPorts` | 结构化端口定义 | 主来源，直接建模 |
| `hostIP` | 宿主机绑定 IP | 公共监听层的一部分 |
| `hostPort` | 宿主机端口 | 公共监听层的一部分 |
| `containerPort` | 容器内端口 | 业务身份的一部分 |
| `protocol` | 传输协议 | 必须保留，不能只看端口号 |
| `host` | 便利展示字段 | 只做展示，不做主键 |
| `ports` | 运行态字符串列表 | 只做回读校验，不做唯一来源 |
| `network` | 容器网络及 IP | 反代 target 选址候选 |
| `hostname` / `domainName` | Docker 容器元数据 | 不等于网站域名，默认不要混用 |

## 如何抽取 hostPort / containerPort / protocol / exposedPorts

### 1. 以 `exposedPorts` 作为主数据

当你已经拿到 `ContainerHelper` 时，端口抽取顺序应当是：

1. 直接读取 `exposedPorts`。
2. 对每个 `Port` 记录做归一化。
3. 以归一化后的结果生成同步对象。

归一化建议：

- `protocol` 统一转成小写。
- `containerPort`、`hostPort` 统一按字符串存储，但比较时按数值排序。
- `hostIP` 如果为空，视为“未显式指定绑定 IP”，不要强行把它当成业务身份。
- `host` 不参与 identity，它只是可重建的展示值。

### 2. `publishAllPorts=true` 时不要猜 hostPort

`publishAllPorts` 的语义是“让 Docker 自动发布端口”。这类容器在同步早期通常会出现两种情况：

- `exposedPorts` 已知，但 `hostPort` 还没回读到最终值。
- `hostPort` 已经回读，但会随着重建变化。

因此：

- 不要在 `hostPort` 未知时提前固化网站映射。
- 如果必须先落库，应该把该记录标记为 pending / unresolved，而不是生成一个可能错误的稳定网站。

### 3. `ContainerInfo.ports` 只能做回读校验

`ContainerInfo.ports` 是字符串数组，说明它更适合前端展示和运行态核对。

推荐做法：

- 同步写入时只信结构化 `Port`。
- 回读核对时才用 `ports` 验证 Docker 实际发布结果。
- 不要把 `ports` 当成长久主键，因为字符串格式一旦变化，历史映射就会漂移。

### 4. `ContainerInfo.network` 用来补 reverse-proxy target

`network` 里带有 `ipv4` / `ipv6`，这意味着反向代理的 upstream 可以优先直接指向容器网络 IP，而不一定要绕宿主机端口。

建议：

- 优先选一个稳定可达的容器 IPv4。
- 如果有多个网络，使用固定的网络优先级，而不是依赖数组顺序。
- 如果容器没有可用 IP，再退回到 `hostIP:hostPort`。

## 多端口容器怎么处理

### 推荐规则：一个 Port 记录对应一个映射单元

如果一个容器暴露多个端口，就把每个 `Port` 记录视为独立的映射候选：

- `80/tcp`
- `443/tcp`
- `8080/tcp`
- `53/udp`

不要把整台容器压成单一端口，否则后续冲突和增量更新都很难稳定。

### 多端口时的网站命名

当容器只暴露一个端口时，可以用容器名作为网站名。

当容器暴露多个端口时，网站名应当带端口后缀，避免冲突和歧义：

- `container-name`
- `container-name-80-tcp`
- `container-name-443-tcp`
- `container-name-53-udp`

这样做的好处是：

- 容器重建后不会因为 `hostPort` 改变而改名。
- 同一容器的多个端口能稳定并存。
- 增量同步时容易做 1:1 比对。

### 多端口时的 domain 规则

domain 建议和网站名使用同一套 canonical slug，并在多端口场景下加端口后缀：

- 单端口：`container-name.example.com`
- 多端口：`container-name-80-tcp.example.com`

如果已经存在人工指定的 domain，应优先保持人工值，不要让自动同步反复改写。

### 多端口时的 reverse-proxy target

每个网站映射只对应一个 target：

- 优先：`containerIPv4:containerPort`
- 退回：`hostIP:hostPort`
- 再退回：宿主机本地可达地址 + `hostPort`

注意：

- 反代 target 用的是“实际可达地址”，不是 `host` 这个展示字段。
- 如果容器网络 IP 可用，target 不应该依赖随机 hostPort，这样更稳定。

## 端口冲突规则

### 冲突分成两层看

#### 1. 公共监听冲突

同一个 `(hostIP, hostPort, protocol)` 只能归属一个有效映射。

如果两个容器都想占同一个公共监听端口：

- 这是硬冲突。
- 不能自动把老映射踢掉。
- 新映射应该进入 pending / conflict 状态，等待人工决策。

#### 2. 业务身份冲突

如果两个记录的 `containerPort + protocol` 一样，但 `hostPort` 不同：

- 这通常意味着绑定方式变化，而不是新服务出现。
- 对同一容器名来说，应该更新已有映射，而不是新建一条重复网站。

### 同容器内的多端口冲突

同一个容器里出现多个端口时：

- 只要 `containerPort` 不同，就应视为不同映射。
- 如果 `containerPort` 相同但 `protocol` 不同，也要保留为不同映射。
- 如果 `Port` 记录重复，先去重，再同步。

### `publishAllPorts` 带来的特殊情况

`publishAllPorts=true` 时，Docker 可能给每个暴露端口分配不同的随机宿主机端口。

这意味着：

- `hostPort` 可能在容器启动后才稳定。
- 同一个容器重建后，`hostPort` 可能变化。
- 因此 `hostPort` 只能做运行时属性，不能做持久 identity。

## website name / domain / reverse-proxy target 的选择规则

### 1. website name

建议按下面的优先级生成：

1. 规范化后的容器名。
2. 如果是多端口容器，再加 `-<containerPort>-<protocol>` 后缀。
3. 任何时候都不要把 `hostPort` 放进基础名称里，因为 hostPort 会变。

### 2. domain

建议 domain 与 website name 同源：

- 从网站名派生。
- 再加一个统一的 base domain。
- 这样 domain 的稳定性和网站名一致。

如果已经有人工配置的 domain：

- 视为人工覆盖。
- 自动同步只更新 target，不主动改 domain。

### 3. reverse-proxy target

target 的优先级建议如下：

1. 容器网络 IPv4 + `containerPort`
2. 宿主机 `hostIP` + `hostPort`
3. 宿主机本地回退地址 + `hostPort`

这个优先级的原因是：

- 容器网络 IP 不依赖公共端口占用，最稳定。
- `hostPort` 可能会因重建、冲突处理或 `publishAllPorts` 变化而漂移。
- 只要容器网络能直达，target 就应该尽量和宿主机端口解耦。

### 4. `hostname` / `domainName` 不等于网站域名

`ContainerHelper.hostname` 和 `ContainerHelper.domainName` 是 Docker 容器自身的元数据字段。

建议把它们当作：

- 容器运行信息
- 诊断信息
- 可能的辅助标签

不要默认把它们直接映射成对外网站域名，否则后续会把 Docker 元数据和外部站点配置混在一起。

## 增量同步的幂等规则

### 推荐的稳定键

同步系统最好保存两个键：

- `canonicalKey = containerName + containerPort + protocol`
- `runtimeKey = containerID + hostIP + hostPort + containerPort + protocol`

用途分工：

- `canonicalKey` 用来判断“这是同一个业务映射吗？”
- `runtimeKey` 用来判断“当前 Docker 实例是否发生了替换？”

### 幂等判定

每次同步都先构建 `desired` 集合，再和已有 `actual` 集合比较：

- `desired ∩ actual` 且 target 相同：no-op
- `desired ∩ actual` 但 target 改变：更新已有记录，不新建
- `desired - actual`：创建新记录
- `actual - desired`：删除或标记失效，取决于你的保留策略

### 不要触发抖动的规则

- 端口数组先排序再比较。
- 重复端口先去重再比较。
- 网络数组先选定优先网络，再比较。
- 不要让 Docker 返回顺序变化触发网站重建。
- 不要让容器重建后的 `containerID` 变化触发网站改名。

### 容器重建时怎么保持稳定

如果容器名不变，但 `containerID` 变了：

- 视为同一个业务对象的实例替换。
- 保持网站名和 domain 不变。
- 仅更新 target / runtime fingerprint。

如果容器名变了：

- 视为新业务对象。
- 按新容器名生成新的 canonicalKey。
- 旧记录按删除/归档策略处理。

## 建议的同步流程

1. 通过容器名拉取结构化配置。
2. 读取 `exposedPorts`，生成规范化 port tuples。
3. 读取 `network`，选择可达的容器 IP。
4. 按 canonicalKey 与已有映射比对。
5. 无冲突则创建/更新网站记录。
6. 遇到 `(hostIP, hostPort, protocol)` 冲突时，保留现有 owner，新记录进入冲突队列。
7. 最后再用 `ContainerInfo.ports` 做运行态核对，而不是做主逻辑输入。

## 这三份缓存文件能确认、但不能替代的部分

能确认的：

- 容器端口是结构化的 `Port[]`。
- 端口包含 `hostIP`、`hostPort`、`containerPort`、`protocol`。
- 容器网络 IP 是可读取的。
- `ports` 是列表页的字符串表达。

不能从这三份文件直接确认的：

- service 层具体如何从 Docker inspect 组装 `Port[]`
- `ContainerInfo.ports` 的字符串格式到底是什么
- 网站创建/更新层是否已经内置了某种端口优先级

所以这篇文档里关于 collision、name、domain、target 的内容，都是基于当前 API 契约给出的推荐同步规则，而不是对隐藏实现的逐行复述。

## 总结

当前这套容器 API 已经把端口同步需要的核心信息暴露出来了：`exposedPorts` 负责结构化输入，`ContainerInfo.network` 负责目标地址候选，`ports` 只负责回读展示。

如果要做增量同步，最重要的原则是：不要用 hostPort 作为身份，不要用字符串列表做主数据，网站名和 domain 要围绕稳定的容器名 + containerPort + protocol 生成，冲突则以公共监听端口为准处理。
