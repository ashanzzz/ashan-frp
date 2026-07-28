# Cloudflare 凭据识别、域名选择与保存设计

> 状态：已批准的产品设计；实现与维护工具必须遵守。
> 适用范围：Ashan FRP 设置中心、Cloudflare API 客户端、凭据存储和 DNS 操作。

## 1. 产品定位

Ashan FRP 是个人自托管项目，只有已登录的本地管理员能够进入设置中心。产品在这一受控边界内优先保证可见、易排障和不遗忘：

- Cloudflare 和 ChmlFrp 已保存密钥在设置页**默认直接以完整明文显示**，不要求点击眼睛按钮。
- 密钥在 SQLite 中仍必须加密保存；“明文显示”只改变已认证设置 API 和设置页的呈现方式。
- 密钥不得进入日志、审计详情、URL、错误消息、前端持久化存储或公开接口。
- 设置页应明确提示：能够访问管理台的管理员可以直接看到完整密钥。

这一规则是明确的个人项目产品决策，不得被后续 AI 工具以“通用最佳实践”为由擅自改回掩码或只写不回显。若未来改成多人或公网共享管理台，必须另行设计并得到项目所有者明确批准。

## 2. 支持的 Cloudflare 认证方式

### 2.1 Zone / Account API Token

- 请求头：`Authorization: Bearer ***`
- 新格式通常以 `cfut_` 开头，但旧 Token 不能只靠字符串外观判断。
- 优先使用 Token，因为它可以限制 Zone、权限、IP 和有效期。

### 2.2 Global API Key

- 请求头：`X-Auth-Email: <Cloudflare 登录邮箱>` 与 `X-Auth-Key: <global key>`。
- 新格式通常以 `cfk_` 开头；旧格式仍可能没有可识别前缀。
- Global API Key 拥有用户级全局权限，是 Cloudflare 标记的 legacy 方式，但本项目按所有者要求支持。
- Global API Key 模式必须填写 Cloudflare 登录邮箱。

## 3. 自动识别规则

设置页不要求用户先理解两种认证方式。用户输入密钥后，后端按以下顺序识别：

1. `cfut_` 前缀：按 API Token 验证。
2. `cfk_` 前缀：要求邮箱，按 Global API Key 验证。
3. 无明确前缀：先按 API Token 验证；若失败且已填写邮箱，再按 Global API Key 验证。
4. 无明确前缀且 Token 验证失败、邮箱为空：返回“需要 Cloudflare 登录邮箱后继续识别”，不保存。
5. 两种方式都失败：返回稳定、脱敏的认证失败错误，不回显密钥或上游原始响应体。

字符串前缀只能用于缩短探测流程，最终认证类型必须以真实 Cloudflare API 请求成功为准。

## 4. 域名识别与自动保存流程

统一使用一个“检测并保存”动作：

1. 用户输入密钥；Global API Key 或自动识别场景可同时输入邮箱。
2. 后端识别认证方式并调用 Cloudflare `GET /zones`。
3. 若没有可访问 Zone：失败，不保存。
4. 若只有一个 Zone：
   - 自动选择该 Zone；
   - 读取该 Zone 的 DNS 记录，验证实际 DNS 权限；
   - 全部通过后原子保存认证类型、邮箱、Zone ID、Zone 名称和加密密钥。
5. 若有多个 Zone：
   - 返回 Zone 菜单和已识别认证类型；
   - 前端要求用户选择，不擅自选第一个，也不提前保存；
   - 用户选择后再次提交，后端确认所选 Zone 确实属于本次凭据，验证 DNS 读取权限，然后原子保存。
6. 成功响应返回已保存状态和完整明文密钥，设置页立即显示；无需再点击全局“保存设置”。

## 5. 数据模型

Cloudflare 凭据需要持久化：

- `auth_method`：`api_token` 或 `global_api_key`
- `account_email`：Global API Key 所需；API Token 模式为空
- `zone_id`：Cloudflare 内部操作使用
- `zone_name`：设置页和产品流程展示使用
- `encrypted_secret`：密钥密文
- `last_verified_at` / `last_error`
- `credential_ref` / `credential_revision` / `token_mask`：日志和审计使用的安全身份信息

Zone ID 与 Zone Name 都应保存，避免每次 DNS 操作都重新列出全部 Zone。用户界面只把 Zone Name 当作主要域名展示。

## 6. API 行为

### `POST /api/v1/settings/integrations/cloudflare/configure`

请求：

```json
{
  "secret": "完整 API Token 或 Global API Key",
  "email": "Global API Key 对应登录邮箱，可空",
  "zone_id": "多 Zone 选择后的 Zone ID，可空"
}
```

单 Zone成功：

```json
{
  "data": {
    "status": "saved",
    "auth_method": "api_token",
    "account_email": "",
    "zone_id": "...",
    "zone_name": "example.com",
    "secret": "完整明文密钥"
  }
}
```

多 Zone 待选择：

```json
{
  "data": {
    "status": "zone_selection_required",
    "auth_method": "global_api_key",
    "zones": [{"id": "...", "name": "a.example"}, {"id": "...", "name": "b.example"}]
  }
}
```

待选择响应不得持久化新密钥。保存成功必须发生在 DNS 读取验证成功之后。

## 7. 前端交互

- 密钥输入框为 `type="text"`，加载设置页后默认显示完整值。
- 邮箱字段始终可见，但 API Token 模式允许留空，并说明它仅用于 Global API Key。
- 主按钮文案为“检测并保存”。
- 多 Zone 时显示原生域名下拉菜单；用户选中后立即自动重新验证并保存所选域名。
- 成功状态显示：认证方式、域名、最近验证时间。
- 错误必须指出下一步：补邮箱、检查权限、选择域名或更换密钥。

## 8. 性能约束

设置页、弹窗和下拉菜单不得使用 `backdrop-filter` / `backdrop-blur-*`。实测嵌套背景模糊会让 70 个节点下拉框变化从约 33ms 恶化到 280–300ms。

视觉层级使用：

- 高不透明度深色背景
- 普通边框
- 轻量静态阴影
- 简短颜色/透明度 transition

下拉选择、弹窗输入和导航点击必须避免触发昂贵的背景重合成。

## 9. 验收标准

- API Token 单 Zone：识别、DNS 读取测试、自动保存一次完成。
- API Token 多 Zone：不提前保存，选择后保存。
- Global API Key：通过邮箱 + Key 认证并支持多 Zone 选择。
- 设置页重新加载后，Cloudflare 与 ChmlFrp 密钥默认完整明文显示。
- SQLite 中不存在明文密钥；日志与审计不包含密钥。
- 前端源码和产物中不存在 `backdrop-filter` / `backdrop-blur`。
- Go 测试、Go vet、Node UI 测试、生产构建和真实浏览器性能验证全部通过。

## 10. 回显与更新边界

- `GET /api/v1/settings` 与成功的 Cloudflare configure 响应允许向已认证单管理员返回完整密钥，并必须带 `Cache-Control: no-store`。
- `PATCH /api/v1/settings` 与 Cloudflare verify 响应不得回显密钥。
- 通用 settings PATCH 不得修改 Cloudflare `auth_method`、邮箱、Zone ID 或 Zone 名称；这些已验证字段只由 configure 事务写入。
- Zone 列表必须按 Cloudflare `result_info.total_pages` 完整分页，不能只处理前 50 个 Zone。
