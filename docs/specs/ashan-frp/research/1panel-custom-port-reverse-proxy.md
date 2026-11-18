# 1Panel 自定义端口反向代理与 ChmlFrp / FRPC 协同架构

## 1. 核心架构与设计哲学

在 Unraid 与公网穿透体系中，传统做法常常试图让反向代理占用 `80` / `443` 端口，或者直接让 FRP 穿透到内网 Docker 容器的明文端口上。但在本系统的生产实践中，采用了高度规范、强隔离的**“自定义端口独立反代 + 统一内网 TLS 终结”**架构：

```
[ 互联网用户 ]
      │ (HTTPS / 443)
      ▼
[ ChmlFrp 公网边缘节点 (如北京多线) ]
      │ (FRP 穿透隧道)
      ▼
[ Unraid 宿主机 ashan-frp (内置 FRPC 客户端) ]
      │ (HTTPS 连接至本地专用监听端口)
      ▼
[ 1Panel OpenResty (监听 192.168.8.11:3000X) ]
      │ - 终结 TLS 握手 (挂载 Let's Encrypt 证书)
      │ - 处理 Web 请求头 / WebSocket / 缓存
      │ (明文 HTTP 反向代理)
      ▼
[ 内部 Docker / 局域网服务 (如 Nextcloud:8666, ERPNext:6888, NewAPI:3001) ]
```

---

## 2. 为什么使用指定端口（3000X）而非 443？

1. **避免与宿主机端口冲突**：
   - Unraid 宿主机管理面板本身监听 80 / 443（或者需要保留给管理服务）。
   - 让 1Panel OpenResty 为每个映射服务分配一个唯一的专用内部端口（`30001` ~ `30008`），彻底杜绝全局 80/443 的端口竞争与冲突。

2. **独立虚拟主机与精准路由**：
   - 每个穿透服务在 1Panel 中拥有专属的监听端口：
     - `pw` (Vaultwarden)：`192.168.8.11:30001` → 后端 `http://192.168.8.11:4743`
     - `audio`：`192.168.8.11:30002`
     - `dsm` (群晖)：`192.168.8.11:30003` → 后端 `http://192.168.8.10:5001`
     - `drive` (Nextcloud)：`192.168.8.11:30004` → 后端 `http://192.168.8.11:8666`
     - `aiapi` (New API)：`192.168.8.11:30005` → 后端 `http://192.168.8.11:3001`
     - `erpnext` (ERPNext 15)：`192.168.8.11:30006` → 后端 `http://192.168.8.11:6888`
     - `api`：`192.168.8.11:30008`
   - 即使客户端或上游网络没有正确传递 SNI（Server Name Indication），基于独立端口的监听也能 100% 精准路由到目标网站，不会发生多站点串流。

3. **后端容器解耦（免去本地复杂证书配置）**：
   - 内部 Docker 容器（如 Nextcloud、ERPNext）无需自行配置 SSL 证书和复杂的 Web 服务器，保持原生明文 HTTP 即可。
   - 所有 SSL 证书申请、DNS-01 验证（Cloudflare）、自动续签全部收敛在 1Panel。

---

## 3. ChmlFrp 隧道协同配置规则

在 ChmlFrp 平台与 `ashan-frp` 隧道创建时，必须严格遵循以下规则：

- **穿透协议 (`porttype`)**：必须选择 **`https`**。
- **本地 IP (`localip`)**：填写 **`192.168.8.11`**。
- **本地端口 (`localport`)**：**必须填写 1Panel 的专属反代端口（如 30004）**，**严禁填写后端 Docker 容器端口（如 8666）**！
- **绑定域名 (`banddomain`)**：填写完整的外部域名（如 `drive.335356119.xyz`）。

当公网用户访问 `https://drive.335356119.xyz` 时：
1. 流量到达轻爪节点，由 FRP 穿透至 `192.168.8.11:30004`；
2. 1Panel OpenResty 在 30004 端口完成 TLS 证书握手（使用已申请的 Let's Encrypt 证书）；
3. 1Panel OpenResty 将请求转发给 `http://192.168.8.11:8666` 并正确注入 `X-Forwarded-Proto: https`、`Host: drive.335356119.xyz`；
4. Nextcloud 返回 200 OK，全链路畅通。

---

## 4. 1Panel v2 API 自动化与 MCP 契约

`ashan-frp` 已经将 1Panel 的自动化完整集成在后端：

1. **HMAC-MD5 认证协议**：
   - 时间戳：`ts = Math.floor(Date.now() / 1000)`
   - 签名：`token = md5("1panel" + ONEPANEL_API_KEY + ts)`
   - 请求头：`1Panel-Token: <token>`, `1Panel-Timestamp: <ts>`

2. **核心接口清单**：
   - 探活与状态：`GET /api/v2/dashboard/base/os`
   - 网站检索：`POST /api/v2/websites/search`
   - 反向代理建站：`POST /api/v2/websites`（指定 `domains: [{"domain": "192.168.8.11", "port": 3000X}]` 与 `websiteSSLID`）
   - 证书签发：`POST /api/v2/websites/ssl`（通过 DNS 挑战自动向 Let's Encrypt 申请 EC256 证书）
   - 站点删除：`POST /api/v2/websites/del`（带 `forceDelete: true`）
   - MCP 服务发现与域名绑定：`POST /api/v2/ai/mcp/search` 与 `POST /api/v2/ai/mcp/domain/bind`
