# CHANGELOG - ashan-frp

## 前端基础

### Added
- 初始化前端工程骨架：React + Vite + TypeScript。
- 路由覆盖 `/dashboard`、`/nodes`、`/tunnels`、`/website-mappings`、`/jobs`、`/logs`、`/settings`，默认重定向到仪表盘。
- 侧边栏 + 顶栏 Shell 布局（240px 侧边栏，可折叠感知的响应式布局）。
- 顶栏 SSE 连接状态指示器（已连接 / 轮询中 / 已断开）与手动刷新按钮。
- `useSSE` hook：SSE 连接，自动回退到 Polling，支持断线重连。
- `useGlobalState` hook / Provider：全局状态管理（当前页面、侧边栏折叠、SSE 状态、上次刷新时间）。
- `EmptyState` 通用空状态组件：支持图标、标题、描述、主次操作。
- `DetailDrawer` 通用详情抽屉组件：遮罩 + 动画面板 + 关闭按钮。
- `StatusTag` 状态标签组件：normal/warning/error/info/unknown 五种语义变体。
- `api.ts` 前端 API 层：预留对后端接口的封装，当前返回模拟响应。
- 七个路由占位页面均已就位：Dashboard、Nodes、Tunnels、WebsiteMappings、Jobs、Logs、Settings。
