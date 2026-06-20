# 内置 frpc 运行时设计

> 适用范围：`ashan-frp` 管理台中的 FRP 客户端运行时。
> 目标：将当前“外部脚本 + 独立 frpc 容器”的运行方式，收敛为 `ashan-frp` 自身托管的 **内置 frpc 运行时**。
> 约束：本设计只定义产品边界、进程模型、状态模型、运维面和落地阶段，不直接引入未验证的 ChmlFrp 私有 API 细节。

---

## 1. 设计结论

`ashan-frp` 的目标形态应当是：

- **前端**：FRP 管理台。
- **后端**：控制面 + 状态库 + 作业系统。
- **运行时**：由 `ashan-frp` 自己托管 `frpc` 子进程。

也就是说，后续不再把“外置 shell 脚本 + 单独 frpc Docker 容器”当作长期产品边界；那套旧路径只作为历史参考，不作为新系统的常驻兼容层。

### 为什么必须内置 frpc

1. **状态一致性**
   - 现在的脚本、外部容器、配置文件分散在不同位置。
   - 管理台无法天然知道“当前运行的是哪份 frpc 配置、哪个节点、哪次切换”。

2. **可观察性**
   - 如果 frpc 只是外部容器，管理台只能间接读日志/状态文件。
   - 内置后，管理台可以直接采集：当前进程 PID、启动时间、退出码、stderr、当前配置 hash、最近心跳。

3. **可控性**
   - 切换节点、重载配置、健康检查、失败回退，都应该由 job runner 驱动，而不是靠外部脚本隐式完成。

4. **产品边界清晰**
   - 用户看到的是一个完整产品，而不是“文档 + shell + 配置 + 容器名约定”的拼装物。

---

## 2. 非目标

以下内容不在第一阶段内置 frpc 范围：

- 不做 FRP 服务端（`frps`）。
- 不做多实例 frpc 集群编排。
- 不做与旧 shell 流程长期并存的双运行时兼容层。
- 不做“同时支持外置 frpc 容器和内置 frpc 子进程”的长期保留。

如果旧方案要下线，就直接迁移到新方案；**不保留长期兼容模式**。

---

## 3. 目标架构

## 3.1 逻辑分层

```text
前端 UI
  └─ 节点 / 隧道 / frpc 运行时 / 作业 / 日志

API 服务
  └─ 保存期望态、触发 job、提供查询视图

Job Runner
  └─ 生成 frpc 配置、切换节点、重载/重启 frpc、写入事件

FRPC Runtime Manager
  └─ 托管 frpc 子进程、stdout/stderr、pid、退出码、健康状态

本地状态库
  └─ tunnels / nodes / settings / jobs / job_events / sync_state / snapshots

外部系统
  └─ ChmlFrp API / 1Panel / Cloudflare / Docker
```

## 3.2 物理部署建议

第一阶段采用**单体部署**：

- 一个后端服务容器（FastAPI + Job Runner + FRPC Runtime Manager）
- 一个 SQLite 数据库文件
- 一个前端静态站点（可与后端同容器，也可单独构建后由后端托管）
- 一个随镜像分发或在启动时校验下载的 `frpc` 二进制

### 关键原则

- `frpc` 是**管理台内部子进程**，不是另一个独立长期容器。
- 配置文件由管理台生成和覆盖，不允许再由外部脚本偷偷改。
- 最终只能有**一条运行路径**：`期望态 → job → runtime manager → frpc`。

---

## 4. FRPC Runtime Manager 设计

## 4.1 核心职责

FRPC Runtime Manager 负责：

- 生成并写入当前有效的 `frpc.toml`
- 拉起 `frpc` 子进程
- 停止 / 重启 / 热重载 `frpc`
- 采集 stdout / stderr / 退出码
- 保存当前运行配置 hash、节点信息、启动时间、最近探活时间
- 在进程异常退出时向 `job_events` / `audit_log` 写入证据

## 4.2 不负责的事

- 不直接决定“该切到哪个节点”
- 不直接调用前端
- 不保存业务真理
- 不绕过 job runner 自己做策略决策

## 4.3 进程模型

建议采用单实例托管：

- 同一时间只允许 **1 个活动 frpc 进程**。
- 所有切换动作通过同一个互斥锁串行化。
- `restart` / `reload` / `switch-node` 不能并发执行。

### 运行状态机

```text
stopped
  -> starting
  -> running
  -> degraded
  -> restarting
  -> failed
  -> stopped
```

含义：

- `stopped`：未运行
- `starting`：配置已写入，正在启动进程
- `running`：进程存在且健康检查通过
- `degraded`：进程存在，但探活异常或日志出现错误征兆
- `restarting`：因切换/修复/配置变更而重启
- `failed`：启动失败、退出码异常、或探活连续失败

---

## 5. 配置与文件边界

## 5.1 配置来源

`frpc.toml` 不再人工维护；它由以下本地状态生成：

- 目标节点
- 启用的 tunnels
- 全局 transport / auth / log 设置
- 用户在 settings 中保存的固定策略

## 5.2 本地文件建议

建议固定到应用数据目录：

- `data/frpc/frpc.toml`
- `data/frpc/frpc.pid`
- `data/frpc/logs/stdout.log`
- `data/frpc/logs/stderr.log`
- `data/frpc/bin/frpc`（若采用内置分发）

## 5.3 配置写入规则

- 先生成候选配置
- 做 schema / 基础字段校验
- 写入临时文件
- 原子替换正式配置
- 再执行 reload / restart

禁止：

- 直接覆盖线上配置后再赌它能启动
- 允许外部脚本持续改同一份文件

---

## 6. 数据模型补充

为支持内置 frpc，建议新增或扩展以下概念：

## 6.1 `settings`

新增运行时设置：

- `frpc_enabled`
- `frpc_binary_source`
- `frpc_binary_version`
- `frpc_log_level`
- `frpc_healthcheck_interval`
- `frpc_restart_backoff`
- `frpc_work_dir`

## 6.2 `sync_state`

增加 frpc 运行态相关记录：

- `subject_type = frpc_runtime`
- `desired_hash`
- `observed_hash`
- `last_error_code`
- `last_error_message`
- `last_attempt_at`
- `last_job_id`

## 6.3 `snapshots`

保存：

- 生成后的 `frpc.toml`
- 当前运行节点摘要
- 最近一次 `frpc -c ...` 探测结果
- 启动前后日志片段

## 6.4 新作业类型

建议新增：

- `frpc.render_config`
- `frpc.start`
- `frpc.reload`
- `frpc.restart`
- `frpc.stop`
- `frpc.switch_node`
- `frpc.health_check`
- `frpc.recover`

---

## 7. 前端壳子应该长什么样

内置 frpc 后，前端必须把它当成一等资源，而不是“隐藏在脚本后面”。

## 7.1 新增/强化的页面区域

### 仪表盘

增加 `FRPC Runtime` 卡片：

- 当前状态：running / degraded / failed / stopped
- 当前节点
- 当前配置 hash
- 最近重启时间
- 最近错误摘要

### 节点页

要能看见：

- 当前 frpc 绑定节点
- 最近切换时间
- 切换失败次数
- 是否被手动固定

### 隧道页

要能看见：

- 某条隧道是否已进入当前 frpc 运行配置
- 期望态 vs 已下发态
- 对应最近一次 reload / restart job

### 设置页

增加 frpc 运行时配置块：

- 二进制版本
- 日志级别
- 自动恢复策略
- 探活间隔
- 切换策略

### 日志页

增加筛选：

- `source = frpc-runtime`
- `source = frpc-process`
- `source = frpc-health`

---

## 8. 后端接口边界

建议补充以下 API：

- `GET /frpc/runtime`
  - 当前状态、节点、pid、启动时间、配置 hash、最近错误
- `POST /frpc/runtime/start`
- `POST /frpc/runtime/stop`
- `POST /frpc/runtime/restart`
- `POST /frpc/runtime/reload`
- `POST /frpc/runtime/switch-node`
- `GET /frpc/runtime/logs`
- `GET /frpc/runtime/config`

注意：

- 这些接口只负责**创建作业或读状态**。
- 真正执行仍由 job runner 完成。

---

## 9. 健康检查与恢复策略

## 9.1 健康信号

至少结合以下信号：

- 进程是否存在
- 最近日志中是否出现认证/代理参数错误
- 配置 hash 是否与期望态一致
- 最近心跳任务是否成功
- 当前节点是否仍在线

## 9.2 自动恢复

建议分级：

1. **轻度异常**：仅 reload
2. **配置异常**：重新渲染配置后 restart
3. **节点异常**：重新选节点后 switch-node
4. **连续失败**：标记 `blocked`，等待人工介入

## 9.3 人工覆盖

若用户手动指定节点或临时停用自动恢复：

- `sync_state.status = manual_override`
- 自动切换 job 不得强行覆盖
- UI 必须显式展示“人工接管中”

---

## 10. 二进制分发策略

第一阶段推荐：**镜像内固定版本 + 启动时校验**。

### 方案 A：镜像内置

优点：

- 最稳定
- 可复现
- 部署简单

缺点：

- 升级需要重建镜像

### 方案 B：启动时下载并校验

优点：

- 升级灵活

缺点：

- 增加供应链和校验复杂度

### 建议结论

先走 **A：镜像内置**，版本升级做成显式发布，不要一开始就做自动下载自更新。

---

## 11. 与当前仓库现状的关系

当前仓库里：

- 已有 shell 脚本时代的 `chmlfrp.sh` / `new_fix_flow.sh`
- 已有后端 scaffold、job runner、1Panel adapter、website sync
- 已有前端页面和交互设计文档
- **但还没有真正的前端代码壳，也没有内置 frpc runtime 实现**

所以当前阶段结论是：

- **研究和架构方向：基本清楚**
- **后端基础壳：已起好**
- **前端产品壳：只有设计，代码未落**
- **内置 frpc：方向已定，但尚未开始实现**

---

## 12. 分阶段落地计划

## Phase 1：先把壳子补齐

目标：形成可运行的“前后端管理台壳”。

- 建立前端工程（React + Vite + TypeScript）
- 落仪表盘 / 节点 / 隧道 / 网站映射 / 作业 / 设置页壳
- 增加 `frpc runtime` 查询接口占位
- 增加 SSE 状态刷新

交付结果：

- 能看到完整后台
- 能读到后端真实数据
- 但 frpc 仍未内置执行

## Phase 2：内置 frpc 最小闭环

目标：让管理台自己拉起 frpc。

- 引入 FRPC Runtime Manager
- 生成 `frpc.toml`
- 支持 start / stop / restart / health check
- UI 展示运行状态

交付结果：

- 不再依赖外部独立 frpc 容器
- 至少能单节点稳定运行

## Phase 3：节点切换与恢复

目标：恢复旧脚本里最关键的自愈能力。

- switch-node job
- 节点健康检查
- 异常自动恢复
- 日志与事件时间线

交付结果：

- 具备基础 failover 能力
- 但策略仍偏保守，不追求全自动复杂决策

## Phase 4：替换旧脚本

目标：彻底下线旧 shell 主路径。

- 删除旧 runtime 主路径依赖
- 将脚本能力迁移成 job family
- 清理 README / 配置说明 / 运维入口

交付结果：

- 统一产品边界
- 没有“双系统并存”残留

---

## 13. 当前建议

基于现在的代码与设计成熟度，推荐下一步顺序是：

1. **先补前端代码壳**
2. **再做内置 frpc 最小 runtime**
3. **最后做切换/自愈策略**

原因：

- 现在后端基础已经有了，前端代码反而是缺口。
- 如果先硬做 frpc runtime，没有 UI 和状态页，后续验证会很痛苦。
- 先把“看得见、点得到、能观察”的壳子补上，第二阶段实现 frpc 才更稳。
