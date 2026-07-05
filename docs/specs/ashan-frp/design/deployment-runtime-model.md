# Docker / Linux 运行模型与前后端协作设计

> 适用范围：`ashan-frp` 的部署方式、运行时边界、容器布局、环境变量分层、以及前后端协作模型。
> 目标：把项目固定为 Docker-first 的可部署产品，避免把 Windows 本机环境误当作生产运行前提。
> 约束：本文只定义运行模型和配置边界，不替代 `architecture.md`、`frontend-ui.md`、`frpc-runtime.md`、`job-event-model.md` 的职责。

## 1. 设计结论

`ashan-frp` 的长期交付形态应当是：

- **后端**：一个 Go 二进制，负责 HTTP API、SSE、job runner、状态存储和运行时控制。
- **前端**：由同一个 Go 二进制通过 `/ui/` 提供的静态 HTML / CSS / JavaScript。
- **部署**：Docker / Compose 是默认生产运行方式。
- **运行环境**：容器默认运行在 Linux 用户态；Windows 仅作为开发主机，不作为产品运行前提。

这意味着：

- 不引入必须依赖 Windows 桌面或 Windows 进程模型的方案。
- 不引入独立前端运行服务器作为默认路径。
- 不把开发便利性配置误写成生产假设。

## 2. 运行拓扑

### 2.1 默认拓扑

```text
浏览器
  └─ HTTP(S)
       └─ Docker 容器中的 Go 应用
            ├─ /api/v1 (JSON API)
            ├─ /api/v1/events/stream (SSE)
            ├─ /ui/ (embedded static frontend)
            └─ /app/data (state, logs, snapshots, runtime files)
```

### 2.2 容器内职责

单个容器内至少包含：

- Go API 服务
- job runner
- SSE broker
- 本地状态读写
- 嵌入式 UI 静态资源
- FRPC runtime manager（如果当前运行态启用）

### 2.3 不作为默认目标的形态

以下形态不作为默认生产路径：

- Windows 原生服务部署
- 需要独立前端服务器的双进程架构
- 需要手工维护多份运行配置的外部脚本模型
- 依赖宿主机路径语义才能工作的 UI 或脚本

## 3. Docker 部署模型

### 3.1 镜像边界

当前仓库的 Dockerfile 已经体现了正确方向：

- builder 阶段构建 Go 二进制
- runtime 阶段使用精简 Linux 镜像
- runtime 只保留证书、时区和下载/调试工具
- 应用二进制位于容器内固定工作目录

生产镜像应保持以下原则：

- 构建和运行分离
- 运行镜像尽量小
- 不把构建工具留在运行镜像里
- 运行时不依赖宿主机上的 Go、Node、Python 或 Windows 工具链

### 3.2 Compose 作为默认入口

`compose.yaml` 应承担以下职责：

- 定义容器端口映射
- 定义数据卷挂载
- 定义应用级环境变量
- 定义健康检查
- 定义最少必要的重启策略

Compose 不应承担业务逻辑：

- 不在 Compose 中编码资源同步规则
- 不在 Compose 中写死业务默认值
- 不依赖宿主机路径的特殊语义

### 3.3 数据卷布局

建议固定为：

- `/app/data/state.db`：主状态库
- `/app/data/frpc/`：运行时工作目录
- `/app/data/frpc/bin/`：frpc 二进制或下载缓存
- `/app/data/frpc/conf/`：生成配置
- `/app/data/frpc/logs/`：stdout/stderr / 运行日志
- `/app/data/backups/`：备份与快照
- `/app/data/tmp/`：临时文件

原则：

- 所有必须持久化的状态都在 `/app/data` 下
- 容器重建后，主状态不丢失
- 临时文件和构建产物不进入持久层

## 4. 环境变量分层

### 4.1 容器运行层

这层配置决定容器如何启动和监听：

- `HTTP_ADDR`
- `DATA_DIR`
- `DATABASE_DSN`
- 容器端口映射
- 健康检查参数

它们属于部署层，应该由 Docker / Compose 提供。

### 4.2 应用配置层

这层配置属于应用行为：

- `EncryptionKey`
- 默认 API 基础路径
- UI 基础路径
- 上游凭据
- 同步策略
- 运行时策略

它们应通过应用设置或受控环境注入，而不是在前端硬编码。

### 4.3 前端展示层

前端只能读取后端提供的数据和必要的运行时元信息：

- 当前版本
- API 基础路径
- UI 基础路径
- 任务状态
- 实时事件流

前端不应：

- 自己决定同步策略
- 自己推导业务真相
- 依赖宿主机目录布局

## 5. 前后端协作模型

### 5.1 后端是唯一事实源

后端负责：

- 保存期望态
- 产生 job
- 执行异步副作用
- 写回观测态
- 发出 SSE 事件
- 提供页面所需的所有可查询视图

### 5.2 前端只做展示与触发

前端负责：

- 展示列表、详情、状态、差异
- 触发 API 动作
- 订阅 SSE 更新
- 将异步状态可视化

前端不负责：

- 数据一致性决策
- 外部系统调用
- 秘钥解密
- 任务状态机判断

### 5.3 API 与 UI 的关系

应保持以下模式：

- 列表 / 详情 / 统计：REST 查询
- 创建 / 修改 / 动作：REST 提交，返回 job 信息
- 长任务进度：SSE
- 页内刷新：可用轮询作为 SSE 降级

这意味着：

- UI 和 API 共用同一个后端合同
- 如果某个动作在 UI 上可见，就应该能由 API 表达
- 如果某个动作需要 job，就不要在前端直接模拟完成

## 6. Linux 运行前提

### 6.1 为什么把 Linux 作为默认运行前提

Docker 运行时最终落在 Linux 用户态，原因是：

- 容器路径、权限、进程、信号和 healthcheck 语义都以 Linux 为主
- Linux 容器和 Compose 更接近实际生产部署模型
- 后端服务、shell 命令和网络检查在 Linux 容器里更稳定

### 6.2 Windows 的角色

Windows 只作为：

- 本地开发主机
- 代码编辑环境
- Docker Desktop 承载环境

Windows 不应成为：

- 业务代码的默认路径假设来源
- 部署脚本的唯一运行前提
- UI 或后端逻辑的兼容性基准

## 7. 前端 Web 应该怎么做

### 7.1 保持 embedded UI

当前最稳妥的方向是继续保持：

- Go 提供 `/ui/`
- 浏览器运行 HTML / CSS / JavaScript
- 静态资源嵌入到后端二进制
- 不引入独立前端 server 作为默认 runtime

### 7.2 页面职责

前端应该围绕三个核心面板组织：

- **概览**：健康、异常、待处理、同步新鲜度
- **资源**：节点、隧道、网站映射
- **运营**：jobs、events、audit、runtime

### 7.3 交互原则

- 所有异步动作必须有即时反馈
- 高风险操作要显式确认
- 详情优先抽屉，避免频繁跳页
- 错误信息要可操作，不要只有“失败了”
- 页面布局应适合运维高频操作，不追求装饰性复杂度

## 8. 部署与验证标准

### 8.1 部署标准

一个合格的部署应该满足：

- Docker Compose 一条命令可启动
- 容器健康检查稳定通过
- `/api/v1/health` 和 `/api/v1/version` 可访问
- `/ui/` 可访问
- `/api/v1/docs` 和 `/api/v1/openapi.json` 可访问
- `/api/docs` 与 `/api/openapi.json` 兼容跳转可用

### 8.2 验证标准

每次涉及运行模型变化时，至少验证：

- 容器是否能启动
- 健康检查是否通过
- API 路由是否一致
- UI 是否能从容器直接加载
- Docker 卷是否保留状态

## 9. 明确不做

- 不把 Windows 本机路径作为生产配置默认值
- 不把独立前端服务器作为默认 runtime
- 不在 Compose 中写业务逻辑
- 不让前端绕过 API 直接改写后端状态
- 不让外部脚本成为长期运行模型的一部分

## 10. 结论

`ashan-frp` 应当以 Docker-first、Linux runtime、单体后端、嵌入式前端的方式交付。

这样做的好处是：

- 部署模型单一
- 验证模型清晰
- UI 和 API 共用同一事实源
- 后续重构和扩展不会被 Windows 特性锁死

后续所有实现都应优先满足这份运行模型，再考虑新增能力。
