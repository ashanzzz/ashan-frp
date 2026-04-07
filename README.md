# ChmlFrp 自动隧道管理系统

> 适用于 Unraid NAS 的 ChmlFrp 自动化运维脚本套件

---

## 项目背景

| 项目 | 内容 |
|------|------|
| **运行平台** | Unraid NAS |
| **管理地址** | `192.168.8.11` |
| **脚本目录** | `/mnt/user/Hdd_Disk_Share/脚本日志/chmlfrp/` |
| **frpc 配置** | `/mnt/user/appdata/frpc/frpc.toml` |

**核心问题**：ChmlFrp 节点可能随时离线，需要自动化故障切换和自愈。

---

## 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                     Unraid 系统 (192.168.8.11)              │
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              User Scripts (定时触发)                    │  │
│  └───────────────────────┬───────────────────────────────┘  │
│                          │                                   │
│                          ▼                                   │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                    chmlfrp.sh                          │  │
│  │                    (控制器/决策中心)                    │  │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌──────────┐  │  │
│  │  │ health  │ │failover │ │ fastest │ │ manual   │  │  │
│  │  │ 健康检查  │ │故障切换  │ │最快节点  │ │手动指定   │  │  │
│  │  └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘  │  │
│  └───────┼───────────┼───────────┼───────────┼────────┘  │
│          │           │           │           │            │
│          └───────────┴─────┬─────┴───────────┘            │
│                            ▼                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                  new_fix_flow.sh                        │  │
│  │                    (执行器)                            │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────┐  │  │
│  │  │节点测速   │ │隧道同步   │ │DNS同步   │ │Docker  │  │  │
│  │  │选优      │ │删除/创建  │ │Cloudflare│ │容器重建│  │  │
│  │  └──────────┘ └──────────┘ └──────────┘ └────────┘  │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                      │                    │
                      ▼                    ▼
              ┌─────────────┐      ┌─────────────┐
              │  ChmlFrp   │      │ Cloudflare  │
              │  API       │      │    DNS      │
              │ cf-v2.uapis.cn     │    API      │
              └─────────────┘      └─────────────┘
```

---

## OAuth2 认证机制（核心）

### 为什么要用 OAuth2？

ChmlFrp 升级为 QZhua 统一认证系统后，API 调用需要使用 OAuth2 的 access_token。

### Token 类型与有效期

| Token 类型 | 有效期 | 用途 |
|------------|--------|------|
| **access_token** | ~10分钟（599秒） | API 调用凭证 |
| **refresh_token** | 一次性，用完即刷新获得新的 | 刷新获取新 access_token |

### Token 生命周期

```
┌─────────────────────────────────────────────────────────────┐
│                    Token 完整生命周期                         │
│                                                              │
│  1. 首次授权 (oauth_reauth)                                  │
│     │                                                       │
│     ▼                                                       │
│  ┌─────────────────────┐                                    │
│  │  设备码授权流程      │ ──► 获得 access_token             │
│  │  (需浏览器操作1次)   │     + refresh_token               │
│  └──────────┬──────────┘                                    │
│             │                                                │
│             ▼                                                │
│  2. 正常使用 (脚本全自动)                                    │
│     │                                                       │
│     ├── access_token 有效 ──► 直接使用                        │
│     │                                                       │
│     └── access_token 过期 ──► 自动用 refresh_token 刷新       │
│                                   │                          │
│                                   ▼                          │
│                              获得新 access_token              │
│                              + 新的 refresh_token (旧的失效)   │
│                                                              │
│  3. refresh_token 失效（未及时保存新的）                      │
│     │                                                       │
│     ▼                                                       │
│  ┌─────────────────────┐                                    │
│  │  需重新运行          │                                    │
│  │  oauth_reauth       │                                    │
│  └─────────────────────┘                                    │
└─────────────────────────────────────────────────────────────┘
```

### OAuth2 失效处理流程

```
┌─────────────────────────────────────────────────────────────┐
│                 get_access_token() 执行流程                    │
│                                                              │
│  1. 检查是否启用 OAuth2                                      │
│     ├── 未启用 ──► 使用旧 token（兼容模式）                   │
│     └── 已启用 ──► 继续步骤2                                 │
│                                                              │
│  2. 检查 access_token 是否存在                               │
│     ├── 不存在 ──► 尝试 refresh_token 刷新                    │
│     │         └── 失败 ──► 报错退出，提示重新授权            │
│     └── 存在 ──► 继续步骤3                                   │
│                                                              │
│  3. 检查 access_token 是否过期                                │
│     ├── 未过期 ──► 直接使用                                  │
│     └── 已过期 ──► 尝试 refresh_token 刷新                    │
│                 └── 失败 ──► 报错退出，提示重新授权            │
│                                                              │
│  4. 返回有效的 access_token                                  │
└─────────────────────────────────────────────────────────────┘
```

### 失效时的错误信息

当 OAuth2 完全失效时，脚本会输出清晰提示：

```
[ERROR] ========================================
[ERROR] OAuth2 Token 刷新失败，需要重新授权！
[ERROR] 请运行以下命令完成授权：
[ERROR]   ./chmlfrp.sh oauth_reauth
[ERROR] ========================================
```

---

## 快速上手

### 第一步：配置凭据

编辑 `userdata.txt`：

```json
{
  "cloudflare": {
    "email": "your@email.com",
    "api_token": "YOUR_CF_API_TOKEN",
    "zone_id": "YOUR_CF_ZONE_ID"
  },
  "chmlfrp": {
    "username": "YOUR_USERNAME",
    "password": "YOUR_PASSWORD",
    "oauth2": {
      "enabled": true,
      "client_id": "019d534218e67f8a862056c1efb869db",
      "client_secret": "0a98ee0b7c69daa4c4922bae9be5df95eff6",
      "access_token": "",
      "refresh_token": "",
      "token_expires_at": 0
    }
  }
}
```

### 第二步：首次授权（必须执行）

```bash
cd /mnt/user/Hdd_Disk_Share/脚本日志/chmlfrp
./chmlfrp.sh oauth_reauth
```

终端会显示：
```
========================================
请在 5 分钟内完成以下授权操作：
========================================

1. 在浏览器打开以下链接：
   https://account-api.qzhua.net/oauth-device-verify?user_code=XXXX-XXXX

2. 或访问: https://account-api.qzhua.net/oauth-device-verify

3. 输入用户代码: XXXX-XXXX

4. 使用 QZhua 账号登录并授权

========================================
```

在浏览器完成授权后，脚本自动保存 token。

### 第三步：创建计划任务

在 Unraid **User Scripts** 中创建 3 个任务：

```bash
# 任务1：健康检查（每1-2分钟）
bash /mnt/user/Hdd_Disk_Share/脚本日志/chmlfrp/chmlfrp.sh health

# 任务2：故障自愈（每5-10分钟）
bash /mnt/user/Hdd_Disk_Share/脚本日志/chmlfrp/chmlfrp.sh failover

# 任务3：主动选最优（每天1次或每6小时）
bash /mnt/user/Hdd_Disk_Share/脚本日志/chmlfrp/chmlfrp.sh fastest
```

---

## 命令参考

### chmlfrp.sh 主控制器

| 命令 | 说明 |
|------|------|
| `./chmlfrp.sh health` | 健康检查，写入状态文件 |
| `./chmlfrp.sh failover` | 仅离线时自动切线修复 |
| `./chmlfrp.sh fastest` | 主动切到最快节点（带冷却） |
| `./chmlfrp.sh manual "节点名"` | 手动指定节点切换 |
| `./chmlfrp.sh userinfo` | 同步用户详情 |
| `./chmlfrp.sh nodes` | 刷新节点候选列表 |
| `./chmlfrp.sh oauth_refresh` | 手动刷新 OAuth2 token |
| `./chmlfrp.sh oauth_reauth` | **重新授权**（OAuth2失效时使用） |

### new_fix_flow.sh 执行器

```bash
# 标准修复流程
./new_fix_flow.sh

# 强制执行（跳过前置检查）
./new_fix_flow.sh --force-run

# 仅同步 DNS
./new_fix_flow.sh --dns-only

# 干跑模式（只打印，不执行）
./new_fix_flow.sh --dry-run

# 指定节点
./new_fix_flow.sh --node "成都电信"
```

---

## 工作原理

### 三种运行模式

| 模式 | 频率 | 触发条件 | 行为 |
|------|------|----------|------|
| **health** | 每1-2分钟 | 定时 | 检查 frpc 状态，写入状态文件 |
| **failover** | 每5-10分钟 | 检测到离线 | 自动切换到最快节点 |
| **fastest** | 每天/每6小时 | 冷却时间够 | 主动优化到最快节点 |

### health（健康检查）

检查项目：
- Docker 容器是否存在、运行中
- frpc.toml 配置是否有效
- TCP 端口连通性
- 日志错误关键词

### failover（故障自愈）

```
检测到离线状态
    │
    ▼
测速所有在线节点（ping RTT）
    │
    ▼
按延迟排序，尝试最优节点
    │
    ├── 成功 ──► 重建隧道+DNS+Docker
    │
    └── 失败 ──► Ban 此节点，尝试下一个
               │
               └── 都失败 ──► 报错退出
```

### fastest（主动选最优）

```
检查冷却时间（距上次切换是否>15分钟）
    │
    ├── 不够 ──► 退出
    │
    └── 够 ──► 测速所有节点
                │
                ├── 有更优节点 ──► 切换
                │
                └── 没有 ──► 退出
```

---

## 配置文件

### userdata.txt（必需）

```json
{
  "cloudflare": {
    "email": "your@email.com",
    "api_token": "YOUR_CF_API_TOKEN",
    "zone_id": "YOUR_CF_ZONE_ID"
  },
  "chmlfrp": {
    "username": "YOUR_USERNAME",
    "password": "YOUR_PASSWORD",
    "oauth2": {
      "enabled": true,
      "client_id": "YOUR_CLIENT_ID",
      "client_secret": "YOUR_CLIENT_SECRET",
      "access_token": "自动填充",
      "refresh_token": "自动填充",
      "token_expires_at": "自动填充"
    }
  }
}
```

### fixed_tunnels.txt（必需）

```json
[
  {
    "name": "web",
    "tunnel_local_ip": "192.168.1.10",
    "tunnel_local_port": "30001",
    "tunnel_type": "https",
    "tunnel_remote_port": "",
    "dns_domain_cname": "app",
    "dns_proxied": false
  },
  {
    "name": "ssh",
    "tunnel_local_ip": "192.168.1.10",
    "tunnel_local_port": "22",
    "tunnel_type": "tcp",
    "tunnel_remote_port": "40022",
    "dns_domain_cname": "ssh",
    "dns_proxied": false
  }
]
```

### settings.env（可选）

```bash
PRIMARY_DOMAIN="yourdomain.com"
FILTER_CHINA="all"
FILTER_TYPE="all"
FILTER_BUILD_SITE="yes"
COOLDOWN_SECONDS=900
BAN_SECONDS=3600
```

### exempt_names.txt（可选）

豁免名单，这些隧道不会被删除：

```
zerotier
test
```

---

## 故障排查

### OAuth2 Token 失效

**表现**：日志中出现授权错误

**解决**：重新运行授权

```bash
./chmlfrp.sh oauth_reauth
```

### 查看日志

```bash
# 查看健康状态
cat chmlfrp-frpc在线测试.txt

# 查看测速结果
cat chmlfrp节点测速.txt

# 查看详细日志
cat 日志-新修复流程.log
```

---

## 文件结构

```
chmlfrp/
├── chmlfrp.sh              # 主控制器
├── new_fix_flow.sh         # 执行器
├── settings.env            # 自动化策略
├── userdata.txt           # 凭据（含 OAuth2）
├── fixed_tunnels.txt      # 隧道定义
├── exempt_names.txt       # 豁免名单
├── backup/                # 备份
└── *.txt                  # 运行时状态文件
```

---

## 安全注意

- `userdata.txt` 包含敏感信息，**不要**提交到公共仓库
- `refresh_token` 可长期使用，妥善保存
- access_token 有效期约 10 分钟，脚本会自动刷新
