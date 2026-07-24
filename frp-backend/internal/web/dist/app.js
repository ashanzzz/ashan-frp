const APP_ROOT_ID = 'app';
const API_PREFIX = '/api/v1';
const STATE = {
  apiBase: API_PREFIX, uiBase: '/ui', version: null, health: null, dashboard: null, settings: null,
  nodes: [], tunnels: [], websites: [], jobs: [], auditLogs: [], authTokens: [], frpcRuntime: null,
  selectedJob: null, selectedJobEvents: [], authMe: null, activePage: 'control', error: '', notice: '',
  loading: false, actionBusy: '', lastLoadedAt: null, sessionMode: 'unknown', loginUsername: '', loginPassword: '',
  recoveryOpen: false, recoveryCopyStatus: '', controlModalOpen: false, controlForm: { name: '', subdomain: '', protocol: 'https', localIp: '192.168.1.1', localPort: '', remotePort: '', nodeId: '', cfProxied: true },
};

const PAGE_META = {
  control: { title: '总控台', kicker: 'CONTROL CENTER', subtitle: '域名映射 · 隧道状态 · 四级健康监控 · 快捷操作' },
  dashboard: { title: '统计', kicker: 'ANALYTICS', subtitle: '集中查看服务健康度、资源状态和待处理任务。' },
  dns: { title: 'DNS 记录', kicker: 'DNS', subtitle: '按主域名分组查看 Cloudflare 解析与穿透关联状态。' },
  domains: { title: '域名', kicker: 'DOMAINS', subtitle: '统一查看已接入域名、HTTPS 和映射关系。' },
  frp: { title: 'FRP 运行时', kicker: 'FRP', subtitle: '管理 FRPC 进程，并核对节点和隧道运行状态。' },
  website: { title: '网站隧道', kicker: 'WEBSITE TUNNELS', subtitle: 'HTTP/HTTPS 隧道与网站映射的交付视图。' },
  jobs: { title: '任务中心', kicker: 'JOBS', subtitle: '查看异步任务执行状态和事件时间线。' },
  nodes: { title: '节点', kicker: 'NODES', subtitle: '查看节点来源、区域、连接与健康状态。' },
  tunnels: { title: '隧道', kicker: 'TUNNELS', subtitle: '管理已配置的转发规则与上线状态。' },
  websites: { title: '网站映射', kicker: 'WEBSITE', subtitle: '查看域名到站点代理的映射与同步状态。' },
  logs: { title: '日志', kicker: 'LOGS', subtitle: '审计用户与系统的重要变更记录。' },
  settings: { title: '系统设置', kicker: 'SETTINGS', subtitle: '核对运行配置、集成凭据与登录会话。' },
};
const NAV_ITEMS = [['control','总控台'],['dashboard','统计'],['dns','DNS 记录'],['frp','FRP 运行时'],['nodes','节点'],['jobs','任务'],['logs','日志'],['settings','设置']];
const RECOVERY_COMMANDS = { local: './ashan-frp admin reset-password', docker: 'docker compose exec -it ashan-frp /app/ashan-frp admin reset-password' };

function $(id) { return document.getElementById(id); }
function esc(value) { return String(value ?? '').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;').replaceAll("'",'&#39;'); }
function safeArray(value) { return Array.isArray(value) ? value : []; }
function fmt(value, fallback = '—') { if (value === null || value === undefined || value === '') return fallback; if (typeof value === 'boolean') return value ? '是' : '否'; return String(value); }
function fmtTime(value) { if (!value) return '—'; const date = new Date(value); return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString('zh-CN', { hour12: false }); }
function apiUrl(path) { return `${STATE.apiBase}${path.startsWith('/') ? path : `/${path}`}`; }
function statusBadge(status) { const text = String(status || 'unknown'); const value = text.toLowerCase(); const cls = ['healthy','online','synced','enabled','succeeded','running','active','ready'].includes(value) ? 'good' : ['degraded','pending','queued','retry_wait','blocked','starting','stopping'].includes(value) ? 'warn' : ['offline','error','failed','canceled','archived','disabled','conflict','stopped'].includes(value) ? 'bad' : ''; return `<span class="badge ${cls}">${esc(text)}</span>`; }
function shortID(value) { const text = String(value || ''); return text.length > 18 ? `${text.slice(0, 10)}…${text.slice(-6)}` : text || '—'; }
function emptyState(title, detail) { return `<div class="empty-state"><strong>${esc(title)}</strong><span>${esc(detail)}</span></div>`; }
function metric(label, value, detail = '') { return `<div class="metric"><span>${esc(label)}</span><strong>${esc(fmt(value, '0'))}</strong>${detail ? `<small>${esc(detail)}</small>` : ''}</div>`; }
function actionButton(label, action, options = {}) { const key = `${action}:${options.id || ''}`; const disabled = STATE.actionBusy ? 'disabled' : ''; return `<button class="${options.secondary ? 'secondary' : ''} ${options.ghost ? 'ghost' : ''}" data-action="${esc(action)}" data-id="${esc(options.id || '')}" ${disabled}>${STATE.actionBusy === key ? '处理中…' : esc(label)}</button>`; }
function apiError(err) { return err?.message || String(err || '请求失败'); }

async function request(path, options = {}) {
  const response = await fetch(apiUrl(path), { credentials: 'same-origin', headers: { 'Content-Type': 'application/json', ...(options.headers || {}) }, ...options });
  const text = await response.text();
  let parsed = null;
  try { parsed = text ? JSON.parse(text) : null; } catch { parsed = text; }
  if (!response.ok) { const error = new Error(parsed?.error?.message || (typeof parsed === 'string' ? parsed : `HTTP ${response.status}`)); error.status = response.status; error.code = parsed?.error?.code; throw error; }
  return parsed;
}

async function loadSnapshot() {
  STATE.loading = true; STATE.error = ''; render();
  try {
    const [version, health, session] = await Promise.all([
      request('/version').catch(() => ({ data: {} })), request('/health').catch(() => ({ data: {} })),
      request('/auth/session', { cache: 'no-store' }).catch(() => ({ data: { authenticated: false } })),
    ]);
    STATE.version = version?.data || null; STATE.health = health?.data || null; STATE.authMe = session?.data?.authenticated ? session.data.account : null; STATE.sessionMode = STATE.authMe ? 'authenticated' : 'anonymous';
    if (STATE.authMe) {
      const [dashboard, settings, nodes, tunnels, websites, jobs, runtime, audit, tokens] = await Promise.all([
        request('/dashboard').catch(() => ({ data: {} })), request('/settings').catch(() => ({ data: {} })), request('/nodes').catch(() => ({ data: { nodes: [] } })),
        request('/tunnels').catch(() => ({ data: { tunnels: [] } })), request('/website-mappings').catch(() => ({ data: { website_mappings: [] } })),
        request('/jobs').catch(() => ({ data: { jobs: [] } })), request('/frpc/runtime').catch(() => ({ data: {} })), request('/audit').catch(() => ({ data: { audit_logs: [] } })), request('/auth/tokens').catch(() => ({ data: { tokens: [] } })),
      ]);
      STATE.dashboard = dashboard?.data || null; STATE.settings = settings?.data || null; STATE.nodes = safeArray(nodes?.data?.nodes ?? nodes?.data);
      STATE.tunnels = safeArray(tunnels?.data?.tunnels ?? tunnels?.data); STATE.websites = safeArray(websites?.data?.website_mappings ?? websites?.data);
      STATE.jobs = safeArray(jobs?.data?.jobs ?? jobs?.data); STATE.frpcRuntime = runtime?.data || null; STATE.auditLogs = safeArray(audit?.data?.audit_logs ?? audit?.data);
      STATE.authTokens = safeArray(tokens?.data?.tokens ?? tokens?.data);
    } else {
      Object.assign(STATE, { dashboard: null, settings: null, nodes: [], tunnels: [], websites: [], jobs: [], frpcRuntime: null, auditLogs: [], authTokens: [], selectedJob: null, selectedJobEvents: [] });
    }
    STATE.lastLoadedAt = new Date();
  } catch (err) { STATE.error = apiError(err); }
  finally { STATE.loading = false; render(); }
}

function isVisiblePage(id) { return NAV_ITEMS.some(([pageID]) => pageID === id); }
function normalizeActivePage() { if (!isVisiblePage(STATE.activePage)) STATE.activePage = 'control'; }
function renderNav() { return NAV_ITEMS.map(([id, title]) => `<button class="nav-item ${STATE.activePage === id ? 'active' : ''}" data-page="${id}">${esc(title)}</button>`).join(''); }
function pageCard(id, body) { return `<section class="view ${STATE.activePage === id ? 'active' : ''}" data-view="${id}">${body}</section>`; }
function viewHeader(id, actions = '') { const meta = PAGE_META[id] || PAGE_META.dashboard; return `<div class="view-head"><div><div class="eyebrow">${esc(meta.kicker)}</div><h2>${esc(meta.title)}</h2><p>${esc(meta.subtitle)}</p></div><div class="view-actions">${actions}</div></div>`; }
function renderTable(headers, rows, emptyTitle, emptyDetail) { return `<div class="table-wrap">${rows.length ? `<table><thead><tr>${headers.map((header) => `<th>${esc(header)}</th>`).join('')}</tr></thead><tbody>${rows.join('')}</tbody></table>` : emptyState(emptyTitle, emptyDetail)}</div>`; }
function infoRows(rows) { return `<div class="meta-list">${rows.map(([label, value, raw]) => `<div class="meta-row"><span>${esc(label)}</span><span${raw ? ' class="mono"' : ''}>${raw ? esc(value) : value}</span></div>`).join('')}</div>`; }
function integrationState(key) { const values = STATE.settings?.integrations || {}; return values[key] || values[key === 'chmlfrp' ? 'chmlFrp' : key] || {}; }
function associatedWebsite(tunnelID) { return STATE.websites.find((website) => website.tunnel_id === tunnelID); }
function isWebsiteTunnel(tunnel) { return ['http','https'].includes(String(tunnel.protocol || '').toLowerCase()) || Boolean(tunnel.full_domain || associatedWebsite(tunnel.id)); }
function distinctDomains() { const map = new Map(); for (const tunnel of STATE.tunnels) if (tunnel.full_domain) map.set(tunnel.full_domain, { domain: tunnel.full_domain, tunnel }); for (const website of STATE.websites) { for (const domain of [website.primary_domain, ...safeArray(website.domains)].filter(Boolean)) if (!map.has(domain)) map.set(domain, { domain, website }); } return [...map.values()].sort((a,b) => a.domain.localeCompare(b.domain)); }

function renderDashboard() {
  const health = STATE.dashboard?.health || STATE.health || {}; const failed = STATE.jobs.filter((job) => ['failed','blocked'].includes(String(job.status).toLowerCase())).length;
  const cards = [metric('隧道总数', STATE.tunnels.length, `${STATE.tunnels.filter((t) => t.actual_state === 'running').length} 个运行中`), metric('节点健康', STATE.nodes.filter((node) => node.health_status === 'healthy').length, `${STATE.nodes.length} 个节点`), metric('待处理任务', health.queued_jobs ?? STATE.jobs.filter((job) => job.status === 'queued').length, `${failed} 个失败或阻塞`), metric('网站映射', STATE.websites.length, `${distinctDomains().length} 个域名`)];
  const recentJobs = STATE.jobs.slice(0, 6).map((job) => `<tr data-job-id="${esc(job.id)}"><td class="mono">${esc(shortID(job.id))}</td><td>${esc(job.title || job.kind || '任务')}</td><td>${statusBadge(job.status)}</td><td>${esc(fmtTime(job.updated_at || job.created_at))}</td></tr>`);
  const audits = STATE.auditLogs.slice(0, 6).map((log) => `<tr><td>${esc(log.action || 'system')}</td><td>${esc(log.account_name || '系统')}</td><td class="mono">${esc(shortID(log.resource_id))}</td><td>${esc(fmtTime(log.created_at))}</td></tr>`);
  return pageCard('dashboard', `${viewHeader('dashboard', actionButton('刷新数据','reload',{secondary:true}))}<div class="metric-grid">${cards.join('')}</div><div class="split-grid"><div class="panel"><div class="panel-head"><h3>最近任务</h3><button class="link-button" data-page="jobs">查看全部</button></div>${renderTable(['任务','类型','状态','更新时间'], recentJobs, '暂无任务', '同步、配置与运行操作产生的任务会显示在这里。')}</div><div class="panel"><div class="panel-head"><h3>最近操作</h3><button class="link-button" data-page="logs">查看日志</button></div>${renderTable(['动作','操作者','资源','时间'], audits, '暂无审计日志', '登录、配置及管理操作会在这里留痕。')}</div></div>`);
}

function renderDNS() {
  const records = STATE.tunnels.filter((tunnel) => tunnel.full_domain || tunnel.dns_domain_cname).map((tunnel) => `<tr><td><strong>${esc(tunnel.full_domain || '未设置域名')}</strong></td><td class="mono">${esc(tunnel.dns_domain_cname || '待同步')}</td><td>${tunnel.dns_proxied || tunnel.cf_proxied ? statusBadge('proxied') : statusBadge('dns only')}</td><td>${statusBadge(tunnel.actual_state || tunnel.desired_state)}</td><td>${tunnel.last_error_message ? `<span class="error-text">${esc(tunnel.last_error_message)}</span>` : '—'}</td></tr>`);
  return pageCard('dns', `${viewHeader('dns', actionButton('刷新数据','reload',{secondary:true}))}<div class="panel"><div class="panel-head"><h3>解析记录</h3><span class="muted">数据来自隧道配置，不在此页伪造 DNS 写入操作。</span></div>${renderTable(['域名','CNAME 目标','代理','隧道状态','最近异常'], records, '暂无 DNS 记录', '为隧道设置完整域名并完成同步后，解析状态会显示在这里。')}</div>`);
}

function renderDomains() {
  const rows = distinctDomains().map(({ domain, tunnel, website }) => `<tr><td><strong>${esc(domain)}</strong></td><td>${tunnel ? statusBadge(tunnel.actual_state || tunnel.desired_state) : statusBadge(website?.status || 'mapped')}</td><td>${website?.https_enabled || tunnel?.onepanel_ssl_enabled ? '已启用' : '未启用'}</td><td>${esc(tunnel?.name || website?.name || website?.site_name || '—')}</td><td class="mono">${esc(shortID(tunnel?.id || website?.id))}</td></tr>`);
  return pageCard('domains', `${viewHeader('domains')}<div class="metric-grid">${metric('已接入域名', distinctDomains().length)}${metric('启用 HTTPS', distinctDomains().filter(({tunnel,website}) => tunnel?.onepanel_ssl_enabled || website?.https).length)}${metric('含网站映射', STATE.websites.length)}${metric('待处理异常', STATE.tunnels.filter((t) => t.last_error_message).length)}</div><div class="panel">${renderTable(['域名','状态','HTTPS','关联对象','ID'], rows, '尚未接入域名', '在隧道或网站映射中配置域名后，这里会自动归集。')}</div>`);
}

function renderFRP() {
  const runtime = STATE.frpcRuntime || {}; const enabled = STATE.tunnels.filter((tunnel) => tunnel.desired_state === 'enabled').length;
  const runtimeInfo = infoRows([['进程状态', statusBadge(runtime.status || 'unknown')], ['健康状态', statusBadge(runtime.health_status || 'unknown')], ['健康检查', esc(fmtTime(runtime.last_healthcheck))], ['最近错误', esc(runtime.last_error || runtime.health_reason || '—')]]);
  const nodes = STATE.nodes.slice(0, 8).map((node) => `<tr><td>${esc(node.display_name || node.canonical_name || node.id)}</td><td>${esc(node.provider || '—')}</td><td>${statusBadge(node.health_status || node.status)}</td><td>${esc(node.region || '—')}</td></tr>`);
  return pageCard('frp', `${viewHeader('frp', `${actionButton('启动 FRPC','frpc-start')}${actionButton('重启','frpc-restart',{secondary:true})}${actionButton('停止','frpc-stop',{ghost:true})}`)}<div class="split-grid"><div class="panel"><div class="panel-head"><h3>FRPC 运行时</h3>${statusBadge(runtime.status || 'unknown')}</div>${runtimeInfo}</div><div class="panel"><div class="metric-grid compact">${metric('已启用隧道', enabled)}${metric('运行中隧道', STATE.tunnels.filter((t) => t.actual_state === 'running').length)}${metric('可用节点', STATE.nodes.filter((n) => n.health_status === 'healthy').length)}${metric('最近异常', STATE.tunnels.filter((t) => t.last_error_message).length)}</div></div></div><div class="panel"><div class="panel-head"><h3>节点状态</h3>${actionButton('同步节点','nodes-sync',{secondary:true})}</div>${renderTable(['节点','提供方','健康状态','区域'], nodes, '暂无节点', '完成节点同步或创建节点后，FRP 调度资源会显示在这里。')}</div>`);
}

function renderWebsiteTunnels() {
  const tunnels = STATE.tunnels.filter(isWebsiteTunnel); const rows = tunnels.map((tunnel) => { const website = associatedWebsite(tunnel.id); return `<tr><td><strong>${esc(tunnel.full_domain || website?.primary_domain || tunnel.name)}</strong></td><td>${esc(tunnel.protocol || 'http')}</td><td class="mono">${esc(`${tunnel.local_ip || '127.0.0.1'}:${tunnel.local_port || '?'}`)}</td><td>${statusBadge(tunnel.actual_state || tunnel.desired_state)}</td><td>${website ? statusBadge(website.status || 'mapped') : '未映射'}</td><td>${actionButton('部署','tunnel-provision',{id:tunnel.id,secondary:true})}</td></tr>`; });
  return pageCard('website', `${viewHeader('website')}<div class="panel"><div class="panel-head"><h3>HTTP / HTTPS 隧道</h3><span class="muted">部署会复用后端任务队列和既有配置。</span></div>${renderTable(['访问域名','协议','本地目标','隧道状态','网站映射','操作'], rows, '暂无网站隧道', '创建 HTTP/HTTPS 隧道或添加网站映射后，这里会展示交付状态。')}</div>`);
}

function renderJobs() {
  const jobs = STATE.jobs; const selected = STATE.selectedJob; const events = STATE.selectedJobEvents;
  const rows = jobs.map((job) => `<tr class="${selected?.id === job.id ? 'selected-row' : ''}" data-job-id="${esc(job.id)}"><td class="mono">${esc(shortID(job.id))}</td><td>${esc(job.title || job.kind || '任务')}</td><td>${statusBadge(job.status)}</td><td>${esc(job.target_type || '—')}</td><td>${esc(fmtTime(job.updated_at || job.created_at))}</td></tr>`);
  const detail = selected ? `<div class="detail-card"><div class="panel-head"><h3>${esc(selected.title || selected.kind || selected.id)}</h3>${statusBadge(selected.status)}</div>${infoRows([['任务 ID', selected.id, true], ['目标', `${selected.target_type || '—'} / ${selected.target_id || '—'}`, true], ['尝试次数', `${selected.attempt_count || 0} / ${selected.max_attempts || 0}`], ['开始时间', esc(fmtTime(selected.started_at || selected.created_at))], ['错误信息', esc(selected.error_message || '—')]])}</div>` : emptyState('选择一个任务', '点击上方列表可查看该任务的详细信息和事件时间线。');
  const timeline = `<div class="event-log">${events.length ? events.map((event) => `<div class="event-item"><div class="panel-head"><strong>${esc(event.kind || event.event_type || '事件')}</strong><span class="muted">${esc(fmtTime(event.created_at))}</span></div><div>${esc(event.message || event.level || '—')}</div></div>`).join('') : emptyState('暂无事件', '任务生成事件后会按当前账号权限显示在这里。')}</div>`;
  return pageCard('jobs', `${viewHeader('jobs', actionButton('刷新任务','reload',{secondary:true}))}<div class="panel">${renderTable(['任务 ID','任务','状态','目标类型','更新时间'], rows, '暂无任务', '执行同步、部署或运行时操作后会生成任务。')}</div><div class="split-grid"><div class="panel">${detail}</div><div class="panel"><div class="panel-head"><h3>任务事件</h3>${selected ? actionButton('刷新事件','job-refresh',{id:selected.id,secondary:true}) : ''}</div>${timeline}</div></div>`);
}

function renderNodes() {
  const rows = STATE.nodes.map((node) => `<tr><td><strong>${esc(node.display_name || node.canonical_name || node.id)}</strong><br><small class="mono">${esc(shortID(node.id))}</small></td><td>${esc(node.provider || '—')}</td><td>${esc(node.node_type || '—')}</td><td>${statusBadge(node.health_status || node.status)}</td><td>${esc(node.region || '—')}</td><td>${esc(node.endpoint_url || '—')}</td></tr>`);
  return pageCard('nodes', `${viewHeader('nodes', actionButton('同步节点','nodes-sync'))}<div class="panel">${renderTable(['节点','提供方','类型','健康状态','区域','端点'], rows, '暂无节点', '点击“同步节点”后，已配置集成中的可用节点会显示在这里。')}</div>`);
}

function renderTunnels() {
  const rows = STATE.tunnels.map((tunnel) => `<tr><td><strong>${esc(tunnel.name || tunnel.id)}</strong><br><small class="mono">${esc(shortID(tunnel.id))}</small></td><td>${esc(tunnel.protocol || tunnel.tunnel_type || '—')}</td><td>${esc(tunnel.full_domain || '—')}</td><td class="mono">${esc(`${tunnel.local_ip || '127.0.0.1'}:${tunnel.local_port || '?'}`)}</td><td>${statusBadge(tunnel.actual_state || tunnel.desired_state)}</td><td>${actionButton('部署','tunnel-provision',{id:tunnel.id,secondary:true})}</td></tr>`);
  return pageCard('tunnels', `${viewHeader('tunnels')}<div class="panel"><div class="panel-head"><h3>转发规则</h3><span class="muted">创建和编辑由后端 API 提供；本页优先提供安全的状态核对与部署动作。</span></div>${renderTable(['隧道','协议','域名','本地目标','状态','操作'], rows, '暂无隧道', '通过 API 或上游集成创建隧道后会显示在这里。')}</div>`);
}

function renderWebsites() {
  const rows = STATE.websites.map((website) => `<tr><td><strong>${esc(website.primary_domain || website.name || website.id)}</strong></td><td>${esc(safeArray(website.domains).join(', ') || '—')}</td><td>${website.https_enabled ? '已启用' : '未启用'}</td><td>${website.proxy_enabled ? '已启用' : '未启用'}</td><td>${statusBadge(website.status || 'unknown')}</td><td>${actionButton('同步','website-sync',{id:website.id,secondary:true})}</td></tr>`);
  return pageCard('websites', `${viewHeader('websites')}<div class="panel"><div class="panel-head"><h3>站点映射</h3><span class="muted">同步操作将请求后端任务队列处理对应映射。</span></div>${renderTable(['主域名','附加域名','HTTPS','反向代理','状态','操作'], rows, '暂无网站映射', '将网站与隧道绑定后，域名、HTTPS 和代理状态会显示在这里。')}</div>`);
}

function renderLogs() {
  const rows = STATE.auditLogs.map((log) => `<tr><td>${esc(fmtTime(log.created_at))}</td><td><strong>${esc(log.action || 'system')}</strong></td><td>${esc(log.account_name || '系统')}</td><td>${esc(log.resource_type || '—')}</td><td class="mono">${esc(shortID(log.resource_id))}</td><td>${esc(log.ip_address || '—')}</td></tr>`);
  return pageCard('logs', `${viewHeader('logs', actionButton('刷新日志','reload',{secondary:true}))}<div class="panel"><div class="panel-head"><h3>审计记录</h3><span class="muted">仅展示由当前登录管理员可访问的记录。</span></div>${renderTable(['时间','动作','操作者','资源类型','资源 ID','来源 IP'], rows, '暂无审计日志', '登录、配置修改、同步和部署操作会在这里留下审计记录。')}</div>`);
}

function renderCloudflareSettings() {
  const cloudflare = integrationState('cloudflare');
  const configured = Boolean(cloudflare.configured || cloudflare.has_api_token);
  const zone = STATE.tempCfZone !== undefined ? STATE.tempCfZone : (cloudflare.zone_name || cloudflare.identifier || '');
  const savedToken = cloudflare.api_token || '';
  const tempToken = STATE.tempCfToken !== undefined ? STATE.tempCfToken : savedToken;
  const busy = STATE.actionBusy ? 'disabled' : '';
  const zoneDisplay = zone ? `已绑定 Zone: <strong>${esc(zone)}</strong>` : '<span class="muted">自动探测 Zone (或在弹窗中选择)</span>';
  return `<div class="panel integration-settings-card"><div class="panel-head"><div><h3>Cloudflare DNS</h3><span class="muted">Token 仅能输入新值，系统不会显示或导出已保存 Token。</span></div>${statusBadge(configured ? 'configured' : 'not configured')}</div><form id="cloudflare-settings-form-old" class="integration-form"><label>API Token<input id="cloudflare-api-token" name="cloudflare-api-token" type="text" value="${esc(tempToken)}" autocomplete="off" placeholder="${configured ? '留空则保留当前 Token' : '粘贴 Cloudflare API Token'}" /></label><div style="margin-bottom:16px; font-size:14px;">${zoneDisplay}</div><p class="muted integration-help">需要目标 Zone 的 DNS 读取和编辑权限；保存后会验证 Token、Zone 与 DNS 读取权限。为避免产生测试记录，写权限会在实际 DNS 操作时由 Cloudflare 强制校验。</p><div class="settings-actions"><button type="submit" ${busy}>${STATE.actionBusy === 'cloudflare-save' ? '保存中…' : '保存并验证'}</button><button type="button" class="secondary" id="cloudflare-verify-btn" ${busy}>${STATE.actionBusy === 'cloudflare-verify' ? '验证中…' : '验证已保存 Token'}</button></div></form>${renderCfZoneModal()}</div>`;
}

function buildSettingsPayload(cloudflare) {
  const current = STATE.settings || {};
  return { general: current.general || {}, sync: current.sync || {}, queue: current.queue || {}, frpc_runtime: current.frpc_runtime || {}, integrations: { ...(current.integrations || {}), cloudflare } };
}

async function saveCloudflareSettings() {
  const token = $('cloudflare-api-token')?.value || '';
  STATE.tempCfToken = token;
  const current = integrationState('cloudflare');
  if (!token && !(current.configured || current.has_api_token)) { STATE.error = '请先输入 Cloudflare API Token。'; render(); return; }
  STATE.actionBusy = 'cloudflare-save'; STATE.error = ''; STATE.notice = ''; render();
  try {
    const zoneToSave = STATE.tempCfZone !== undefined ? STATE.tempCfZone : '';
    await request('/settings', { method: 'PATCH', body: JSON.stringify(buildSettingsPayload({ ...current, zone_name: zoneToSave, api_token: token })) });
    const verification = await request('/settings/integrations/cloudflare/verify', { method: 'POST' });
    await loadSnapshot();
    if (verification?.data?.valid) {
      STATE.notice = 'Cloudflare 配置已保存，Token、Zone 和 DNS 读取权限验证通过。';
      STATE.tempCfZone = undefined;
      STATE.tempCfToken = undefined;
    } else {
      STATE.error = `Cloudflare 配置已保存，但验证失败：${verification?.data?.message || '未知错误'}`;
    }
  } catch (err) {
    if (err.code === 'MULTIPLE_ZONES') {
      STATE.cfZoneModalOpen = true;
      try {
        const res = await request('/settings/integrations/cloudflare/zones', { method: 'POST', body: JSON.stringify({ token }) });
        STATE.cfZones = res?.data?.zones || [];
      } catch (e) {
        STATE.error = `拉取 Zones 失败：${apiError(e)}`;
      }
    } else {
      STATE.error = `Cloudflare 配置保存失败：${apiError(err)}`;
    }
  }
  finally { STATE.actionBusy = ''; render(); }
}

async function verifyCloudflareSettings() {
  STATE.tempCfToken = $('cloudflare-api-token')?.value || '';
  STATE.tempCfZone = $('cloudflare-zone')?.value || '';
  STATE.actionBusy = 'cloudflare-verify'; STATE.error = ''; STATE.notice = ''; render();
  try {
    const response = await request('/settings/integrations/cloudflare/verify', { method: 'POST' });
    STATE.notice = response?.data?.valid ? 'Cloudflare Token、Zone 和 DNS 读取权限验证通过。' : `Cloudflare 验证失败：${response?.data?.message || '未知错误'}`;
    await loadSnapshot();
  } catch (err) { STATE.error = `Cloudflare 验证失败：${apiError(err)}`; }
  finally { STATE.actionBusy = ''; render(); }
}
function renderSettings() {
  const runtime = STATE.settings?.frpc_runtime || {}; const integrations = [['chmlfrp','chmlfrp'], ['onepanel','OnePanel'], ['cloudflare','Cloudflare']].map(([key,label]) => { const item = integrationState(key); return `<tr><td><strong>${label}</strong></td><td>${item.configured || item.has_password || item.has_api_token ? statusBadge('configured') : statusBadge('not configured')}</td><td>${esc(item.identifier || item.username || item.base_url || item.zone_name || '—')}</td><td>${esc(fmtTime(item.updated_at || item.last_verified))}</td><td>${item.last_error ? `<span class="error-text">${esc(item.last_error)}</span>` : '—'}</td></tr>`; });
  const tokenRows = STATE.authTokens.map((token) => `<tr><td class="mono">${esc(shortID(token.id))}</td><td>${esc(token.token_type || 'session')}</td><td>${token.revoked_at ? statusBadge('revoked') : statusBadge('active')}</td><td>${esc(fmtTime(token.last_used_at))}</td><td>${esc(fmtTime(token.expires_at))}</td><td>${token.revoked_at ? '—' : actionButton('吊销','token-revoke',{id:token.id,ghost:true})}</td></tr>`);
  return pageCard('settings', `${viewHeader('settings', actionButton('刷新设置','reload',{secondary:true}))}<div class="split-grid"><div class="panel"><div class="panel-head"><h3>运行配置</h3>${statusBadge(runtime.frpc_enabled ? 'enabled' : 'disabled')}</div>${infoRows([['日志级别', esc(runtime.frpc_log_level || '—')], ['自动恢复', esc(runtime.auto_recover_strategy || '—')], ['健康检查间隔', esc(runtime.frpc_healthcheck_interval || '—')], ['同步轮询间隔', esc(STATE.settings?.sync?.sync_poll_interval || '—')]])}</div><div class="panel"><div class="panel-head"><h3>安全提示</h3></div><p class="muted">修改密码需要验证旧密码。忘记管理员密码时，请在服务器终端运行恢复命令；恢复会吊销所有旧会话。</p><button class="secondary" id="forgot-password-btn">忘记密码？查看恢复命令</button></div></div>${renderCloudflareSettings()}<div class="panel"><div class="panel-head"><h3>集成状态</h3></div>${renderTable(['集成','状态','标识','更新时间','最近错误'], integrations, '暂无集成信息', '保存集成配置后会显示凭据状态，不会显示密钥内容。')}</div><div class="panel"><div class="panel-head"><h3>登录会话</h3><span class="muted">吊销会使对应 Session 或 API Token 立即失效。</span></div>${renderTable(['令牌 ID','类型','状态','最近使用','过期时间','操作'], tokenRows, '暂无活跃会话', '成功登录后会自动创建会话记录。')}</div>`);
}

function loginPanel() { return `<div class="login-card"><div class="login-intro"><h3>进入 Ashan FRP 运营台</h3><p>使用唯一管理员账号登录，查看和管理 FRP、DNS 与网站服务。</p></div><form id="login-form" class="login-form"><label>管理员用户名<input id="login-username" autocomplete="username" value="${esc(STATE.loginUsername)}" required /></label><label>密码<input id="login-password" type="password" autocomplete="current-password" required /></label><div class="login-actions"><button type="submit">登录运营台</button></div><div class="login-help"><span>忘记密码？</span><button type="button" class="link-button" id="forgot-password-btn">查看终端恢复方法</button></div></form></div>`; }
function recoveryDialog() { if (!STATE.recoveryOpen) return ''; return `<div class="recovery-backdrop" id="recovery-backdrop"><section class="recovery-dialog" id="recovery-dialog" role="dialog" aria-modal="true" aria-labelledby="recovery-title" tabindex="-1"><div class="recovery-head"><div><div class="eyebrow">CREDENTIAL RECOVERY</div><h2 id="recovery-title">管理员凭据恢复</h2></div><button class="icon-button" id="recovery-close-btn" aria-label="关闭">×</button></div><div class="recovery-notice"><strong>系统不提供网页密码重置，也无法查看当前密码。</strong><span>Ashan FRP 只允许一个管理员。需要具有服务器或容器终端权限，使用下面的命令重新设置管理员用户名和密码。</span></div><ol class="recovery-steps"><li>在部署服务器上执行恢复命令。</li><li>按提示输入新的管理员用户名和密码；密码不会在终端回显。</li><li>恢复完成后，所有旧 Session 和 API Token 会立即失效。</li></ol><div class="command-group"><div class="command-head"><strong>直接运行</strong><button class="secondary" data-copy-recovery="local">复制</button></div><pre><code>${esc(RECOVERY_COMMANDS.local)}</code></pre></div><div class="command-group"><div class="command-head"><strong>Docker Compose</strong><button class="secondary" data-copy-recovery="docker">复制</button></div><pre><code>${esc(RECOVERY_COMMANDS.docker)}</code></pre></div><div class="recovery-foot"><span class="copy-status">${esc(STATE.recoveryCopyStatus)}</span><button id="recovery-confirm-btn">我知道了</button></div></section></div>`; }

async function loadSelectedJob(jobID) { if (!jobID) return; try { const response = await request(`/jobs/${encodeURIComponent(jobID)}`); STATE.selectedJob = response?.data?.job || null; STATE.selectedJobEvents = safeArray(response?.data?.events); } catch (err) { STATE.error = `任务详情加载失败：${apiError(err)}`; } render(); }
async function runAction(action, id = '') {
  const routes = { 'frpc-start': ['/frpc/start','POST'], 'frpc-stop': ['/frpc/stop','POST'], 'frpc-restart': ['/frpc/restart','POST'], 'nodes-sync': ['/nodes/sync','POST'], 'tunnel-provision': [`/tunnels/${encodeURIComponent(id)}/provision`,'POST'], 'website-sync': [`/website-mappings/${encodeURIComponent(id)}/sync`,'POST'], 'token-revoke': [`/auth/tokens/${encodeURIComponent(id)}/revoke`,'POST'] };
  if (action === 'reload') return loadSnapshot(); if (action === 'job-refresh') return loadSelectedJob(id); const route = routes[action]; if (!route) return;
  STATE.actionBusy = `${action}:${id}`; STATE.error = ''; STATE.notice = ''; render();
  try { const response = await request(route[0], { method: route[1] }); STATE.notice = response?.data?.message || '操作已提交。'; await loadSnapshot(); }
  catch (err) { STATE.error = `操作失败：${apiError(err)}`; }
  finally { STATE.actionBusy = ''; render(); }
}
function openRecoveryDialog() { STATE.recoveryOpen = true; STATE.recoveryCopyStatus = ''; render(); setTimeout(() => $('recovery-dialog')?.focus(), 0); }
function closeRecoveryDialog() { STATE.recoveryOpen = false; STATE.recoveryCopyStatus = ''; render(); setTimeout(() => $('forgot-password-btn')?.focus(), 0); }
async function copyRecoveryCommand(kind) { const command = RECOVERY_COMMANDS[kind]; if (!command) return; try { if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(command); else { const textarea = document.createElement('textarea'); textarea.value = command; textarea.setAttribute('readonly',''); textarea.style.position = 'fixed'; textarea.style.opacity = '0'; document.body.appendChild(textarea); textarea.select(); document.execCommand('copy'); textarea.remove(); } STATE.recoveryCopyStatus = '恢复命令已复制。'; } catch { STATE.recoveryCopyStatus = '复制失败，请手动选择命令。'; } render(); setTimeout(() => $('recovery-dialog')?.focus(), 0); }
async function submitLogin() { const username = $('login-username')?.value.trim() || ''; const password = $('login-password')?.value || ''; STATE.loginUsername = username; if (!username || !password) { STATE.error = '请输入管理员用户名和密码。'; render(); return; } try { await request('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) }); STATE.loginPassword = ''; await loadSnapshot(); } catch (err) { STATE.loginPassword = ''; STATE.error = apiError(err); render(); } }

// ── Control Center (总控台) ──

function controlHealthLevel(condition) { return condition === 'good' ? 'good' : condition === 'warn' ? 'warn' : 'bad'; }

function getControlHealth() {
  const runtime = STATE.frpcRuntime || {};
  const tunnels = safeArray(STATE.tunnels);
  const settings = STATE.settings || {};
  const integrations = settings.integrations || {};
  
  // FRPC process status
  const frpcStatus = runtime.status || runtime.engine_status || 'unknown';
  const frpcProcess = ['running','healthy'].includes(frpcStatus?.toLowerCase()) ? 'good' : ['starting','stopping','reloading'].includes(frpcStatus?.toLowerCase()) ? 'warn' : 'bad';
  
  // FRPC config sync status
  const hasError = tunnels.some(t => t.actual_state === 'error' || t.last_error_message);
  const hasPending = tunnels.some(t => t.actual_state === 'pending' || t.actual_state === 'provisioning');
  const frpcConfig = hasError ? 'bad' : hasPending ? 'warn' : 'good';
  
  // ChmlFrp status
  const chmlfrp = integrations.chmlfrp || {};
  const chmlfrpConfigured = settingsConfigured(chmlfrp, 'has_password');
  const chmlfrpStatus = chmlfrpConfigured ? (chmlfrp.last_error ? 'warn' : 'good') : 'bad';
  
  // DNS/Cloudflare status
  const cloudflare = integrations.cloudflare || {};
  const dnsConfigured = settingsConfigured(cloudflare, 'has_api_token');
  const dnsStatus = dnsConfigured ? (cloudflare.last_error ? 'warn' : 'good') : 'bad';
  
  return { frpcProcess, frpcConfig, chmlfrpStatus, dnsStatus };
}

function getControlHealthLabel(health) {
  const labels = {
    frpcProcess: { good: '运行中', warn: '启动中', bad: '已停止' },
    frpcConfig: { good: '已同步', warn: '同步中', bad: '有异常' },
    chmlfrpStatus: { good: '已连接', warn: '有警告', bad: '未配置' },
    dnsStatus: { good: '已连接', warn: '有警告', bad: '未配置' },
  };
  return labels;
}

function tunnelStatusDots(tunnel) {
  const runtime = STATE.frpcRuntime || {};
  const settings = STATE.settings || {};
  const integrations = settings.integrations || {};
  
  const frpcOk = ['running','healthy'].includes((runtime.status || runtime.engine_status || '').toLowerCase());
  const configOk = tunnel.actual_state === 'enabled' || tunnel.actual_state === 'running';
  const configPending = tunnel.actual_state === 'pending' || tunnel.actual_state === 'provisioning';
  const chmlOk = (integrations.chmlfrp || {}).configured && !(integrations.chmlfrp || {}).last_error;
  const chmlWarn = (integrations.chmlfrp || {}).configured && (integrations.chmlfrp || {}).last_error;
  const dnsOk = (integrations.cloudflare || {}).configured && !(integrations.cloudflare || {}).last_error;
  const dnsWarn = (integrations.cloudflare || {}).configured && (integrations.cloudflare || {}).last_error;
  
  const d1 = frpcOk ? 'good' : 'bad';
  const d2 = configOk ? 'good' : configPending ? 'warn' : 'bad';
  const d3 = chmlOk ? 'good' : chmlWarn ? 'warn' : 'bad';
  const d4 = dnsOk ? 'good' : dnsWarn ? 'warn' : 'bad';
  
  return `<div class="status-dots" title="FRPC进程 | 配置同步 | ChmlFrp | DNS">`
    + `<span class="micro-dot ${d1}"></span>`
    + `<span class="micro-dot ${d2}"></span>`
    + `<span class="micro-dot ${d3}"></span>`
    + `<span class="micro-dot ${d4}"></span>`
    + `</div>`;
}

function getNodeName(nodeId) {
  const node = safeArray(STATE.nodes).find(n => n.id === nodeId);
  return node ? (node.display_name || node.canonical_name || node.id) : (nodeId || '—');
}

function getPrimaryDomain() {
  const settings = STATE.settings || {};
  const cfZone = settings.integrations?.cloudflare?.identifier;
  if (cfZone) return cfZone;
  const tunnels = safeArray(STATE.tunnels);
  for (const t of tunnels) {
    if (t.full_domain && t.subdomain) {
      const suffix = t.full_domain.replace(t.subdomain, '');
      if (suffix.startsWith('.')) return suffix.slice(1);
    }
  }
  return '335356119.xyz';
}

function renderControlPage() {
  const health = getControlHealth();
  const labels = getControlHealthLabel();
  const tunnels = safeArray(STATE.tunnels);
  const domain = getPrimaryDomain();
  
  // Health capsules
  const capsules = [
    { key: 'frpcProcess', icon: '⚡', title: 'FRPC 进程', detail: labels.frpcProcess[health.frpcProcess] },
    { key: 'frpcConfig', icon: '📋', title: 'FRPC 配置', detail: labels.frpcConfig[health.frpcConfig] },
    { key: 'chmlfrpStatus', icon: '🐸', title: 'ChmlFrp', detail: labels.chmlfrpStatus[health.chmlfrpStatus] },
    { key: 'dnsStatus', icon: '🌐', title: 'DNS 服务', detail: labels.dnsStatus[health.dnsStatus] },
  ];
  
  const healthRow = `<div class="control-health-row">${capsules.map(c =>
    `<div class="health-capsule ${health[c.key]}">`
    + `<span class="capsule-dot"></span>`
    + `<div class="capsule-info">`
    + `<span class="capsule-title">${esc(c.icon)} ${esc(c.title)}</span>`
    + `<span class="capsule-detail">${esc(c.detail)}</span>`
    + `</div></div>`
  ).join('')}</div>`;

  const sysMetrics = STATE.health?.system_metrics || { goroutines: 0, memory_alloc_mb: 0, memory_sys_mb: 0, sqlite_open_conns: 0, sqlite_in_use_conns: 0, uptime_seconds: 0 };
  const formatUptime = (s) => {
    if (s < 60) return `${s}s`;
    if (s < 3600) return `${Math.floor(s/60)}m ${s%60}s`;
    return `${Math.floor(s/3600)}h ${Math.floor((s%3600)/60)}m`;
  };

  const metricCapsules = [
    { icon: '⏱️', title: '运行时间', detail: formatUptime(sysMetrics.uptime_seconds) },
    { icon: '🧵', title: 'Go 协程数', detail: `${sysMetrics.goroutines}` },
    { icon: '💾', title: '内存占用', detail: `${sysMetrics.memory_alloc_mb} MB / ${sysMetrics.memory_sys_mb} MB` },
    { icon: '🗄️', title: 'SQLite 状态', detail: `连接数 ${sysMetrics.sqlite_open_conns} (活跃 ${sysMetrics.sqlite_in_use_conns})` },
  ];

  const metricsRow = `<div class="control-health-row" style="margin-top: 16px;">${metricCapsules.map(c =>
    `<div class="health-capsule good">`
    + `<div class="capsule-info" style="margin-left: 0;">`
    + `<span class="capsule-title">${esc(c.icon)} ${esc(c.title)}</span>`
    + `<span class="capsule-detail">${esc(c.detail)}</span>`
    + `</div></div>`
  ).join('')}</div>`;
  
  // Table rows
  const rows = tunnels.map(tunnel => {
    const subdomain = tunnel.subdomain || tunnel.dns_domain_cname || tunnel.name || '';
    const fullUrl = tunnel.full_domain || (subdomain ? `${subdomain}.${domain}` : '');
    const protocol = tunnel.protocol || tunnel.tunnel_type || 'tcp';
    const localTarget = `${tunnel.local_ip || '127.0.0.1'}:${tunnel.local_port || '?'}`;
    const nodeName = getNodeName(tunnel.node_id);
    const cdnOn = tunnel.cf_proxied || tunnel.dns_proxied;
    const isWeb = ['http','https'].includes(protocol.toLowerCase());
    
    return `<tr>`
      + `<td>${tunnelStatusDots(tunnel)}</td>`
      + `<td><strong>${esc(tunnel.project_name || tunnel.name || shortID(tunnel.id))}</strong></td>`
      + `<td class="control-url mono"${isWeb ? ` data-url="${protocol.toLowerCase()}://${esc(fullUrl)}"` : ''}>${esc(fullUrl || '—')}</td>`
      + `<td>${statusBadge(protocol.toUpperCase())}</td>`
      + `<td class="mono">${esc(localTarget)}</td>`
      + `<td>${tunnel.remote_port ? esc(tunnel.remote_port) : '—'}</td>`
      + `<td>${esc(nodeName)}</td>`
      + `<td><span class="cdn-badge ${cdnOn ? 'cdn-on' : 'cdn-off'}">${cdnOn ? '🟠 CDN' : '⚪ 直连'}</span></td>`
      + `<td><div class="dns-row-actions">`
      + `<button class="secondary tiny-btn" data-action="tunnel-provision" data-id="${esc(tunnel.id)}">部署</button>`
      + `<button class="secondary tiny-btn" data-control-action="edit-mapping" data-id="${esc(tunnel.id)}">编辑</button>`
      + `<button class="ghost tiny-btn" data-control-action="delete-mapping" data-id="${esc(tunnel.id)}">删除</button>`
      + `</div></td>`
      + `</tr>`;
  });
  
  const table = renderTable(
    ['状态', '服务名称', '访问地址', '协议', '内网目标', '远程端口', '穿透节点', 'CDN', '操作'],
    rows,
    '暂无穿透映射',
    '点击「+ 新增映射」创建第一条穿透路由。'
  );
  
  const quickActions = `<div class="control-actions">`
    + `<button data-control-action="new-mapping">+ 新增映射</button>`
    + `${actionButton('🔄 重载 frpc', 'frpc-restart', {secondary: true})}`
    + `${actionButton('🌐 刷新 DNS', 'nodes-sync', {secondary: true})}`
    + `${actionButton('刷新数据', 'reload', {ghost: true})}`
    + `</div>`;
  
  return pageCard('control', `${viewHeader('control')}<div class="panel">${healthRow}${metricsRow}${table}${quickActions}</div>${renderControlModal()}`);
}

function renderControlModal() {
  if (!STATE.controlModalOpen) return '';
  const form = STATE.controlForm;
  const isEdit = Boolean(STATE.controlEditId);
  const nodes = safeArray(STATE.nodes);
  const domain = getPrimaryDomain();
  const isTcp = ['tcp','udp'].includes(form.protocol);
  
  return `<div class="control-modal-backdrop" id="control-backdrop">`
    + `<section class="control-modal" role="dialog" aria-modal="true">`
    + `<div class="panel-head"><div><div class="eyebrow">${isEdit ? 'EDIT MAPPING' : 'NEW MAPPING'}</div><h3>${isEdit ? '编辑穿透映射' : '新增穿透映射'}</h3></div>`
    + `<button class="icon-button" data-control-action="close" aria-label="关闭">✕</button></div>`
    + `<form id="control-form" class="control-form">`
    + `<label>服务名称<input id="ctrl-name" value="${esc(form.name)}" placeholder="如：NAS 控制台" required /></label>`
    + `<label>穿透协议<select id="ctrl-protocol"><option value="https"${form.protocol==='https'?' selected':''}>HTTPS</option><option value="http"${form.protocol==='http'?' selected':''}>HTTP</option><option value="tcp"${form.protocol==='tcp'?' selected':''}>TCP</option><option value="udp"${form.protocol==='udp'?' selected':''}>UDP</option></select></label>`
    + `<label>域名前缀<input id="ctrl-subdomain" value="${esc(form.subdomain)}" placeholder="如：nas" required /></label>`
    + `<label>域名后缀<select id="ctrl-domain" disabled><option value="${esc(domain)}">.${esc(domain)}</option></select></label>`
    + `<label>内网 IP<input id="ctrl-ip" value="${esc(form.localIp)}" placeholder="192.168.1.1" required /></label>`
    + `<label>内网端口<input id="ctrl-port" type="number" value="${esc(form.localPort)}" placeholder="80" required /></label>`
    + (isTcp ? `<label>远程端口<input id="ctrl-rport" type="number" value="${esc(form.remotePort)}" placeholder="40022" required /></label>` : '')
    + `<label>穿透节点<select id="ctrl-node">${nodes.length ? nodes.map(n => `<option value="${esc(n.id)}"${form.nodeId===n.id?' selected':''}>${esc(n.display_name || n.canonical_name || n.id)}</option>`).join('') : '<option value="">暂无可用节点</option>'}</select></label>`
    + `<label class="full-width">Cloudflare CDN<div style="display:flex;gap:12px;align-items:center"><label class="control-toggle"><input type="checkbox" id="ctrl-cdn"${form.cfProxied ? ' checked' : ''} /><span class="toggle-slider"></span></label><span style="color:var(--muted);font-size:12px">${form.cfProxied ? '🟠 CDN 加速已开启' : '⚪ DNS 直连模式'}</span></div></label>`
    + `<div class="form-actions full-width">`
    + `<button type="button" class="secondary" data-control-action="close">取消</button>`
    + `<button type="submit"${STATE.actionBusy ? ' disabled' : ''}>${STATE.actionBusy === 'control-save' ? '保存中…' : isEdit ? '保存修改并重新部署' : '创建并部署'}</button>`
    + `</div></form></section></div>`;
}

function bindControlUI() {
  // Control page event bindings
  document.querySelectorAll('[data-control-action]').forEach(btn => btn.addEventListener('click', async () => {
    const action = btn.dataset.controlAction;
    const id = btn.dataset.id;
    if (action === 'new-mapping') {
      STATE.controlEditId = null;
      STATE.controlModalOpen = true;
      STATE.controlForm = { name: '', subdomain: '', protocol: 'https', localIp: '192.168.1.1', localPort: '', remotePort: '', nodeId: safeArray(STATE.nodes)[0]?.id || '', cfProxied: true };
      render();
    }
    if (action === 'edit-mapping' && id) {
      const tunnel = safeArray(STATE.tunnels).find(t => t.id === id);
      if (tunnel) {
        STATE.controlEditId = id;
        STATE.controlModalOpen = true;
        STATE.controlForm = {
          name: tunnel.project_name || tunnel.name || '',
          subdomain: tunnel.subdomain || tunnel.dns_domain_cname || '',
          protocol: tunnel.protocol || tunnel.tunnel_type || 'https',
          localIp: tunnel.local_ip || '127.0.0.1',
          localPort: tunnel.local_port || '',
          remotePort: tunnel.remote_port || '',
          nodeId: tunnel.node_id || safeArray(STATE.nodes)[0]?.id || '',
          cfProxied: Boolean(tunnel.cf_proxied || tunnel.dns_proxied),
        };
        render();
      }
    }
    if (action === 'delete-mapping' && id) {
      const tunnel = safeArray(STATE.tunnels).find(t => t.id === id);
      const name = tunnel ? (tunnel.project_name || tunnel.name || id) : id;
      if (confirm(`确定要归档/删除穿透映射「${name}」吗？`)) {
        STATE.actionBusy = `control-delete:${id}`; STATE.error = ''; render();
        try {
          await request(`/tunnels/${encodeURIComponent(id)}`, { method: 'DELETE' });
          STATE.notice = `映射「${name}」已成功归档删除。`;
          await loadSnapshot();
        } catch (err) { STATE.error = `删除失败：${apiError(err)}`; }
        finally { STATE.actionBusy = ''; render(); }
      }
    }
    if (action === 'close') { STATE.controlModalOpen = false; STATE.controlEditId = null; render(); }
  }));
  
  // URL click to open in new tab
  document.querySelectorAll('.control-url[data-url]').forEach(td => td.addEventListener('click', () => {
    const url = td.dataset.url;
    if (url) window.open(url, '_blank');
  }));
  
  // Modal backdrop click to close
  const backdrop = $('control-backdrop');
  if (backdrop) backdrop.addEventListener('click', (e) => { if (e.target === backdrop) { STATE.controlModalOpen = false; STATE.controlEditId = null; render(); } });
  
  // Protocol change shows/hides remote port
  const protocolSelect = $('ctrl-protocol');
  if (protocolSelect) protocolSelect.addEventListener('change', (e) => { STATE.controlForm.protocol = e.target.value; render(); });
  
  // CDN toggle update
  const cdnCheckbox = $('ctrl-cdn');
  if (cdnCheckbox) cdnCheckbox.addEventListener('change', (e) => { STATE.controlForm.cfProxied = e.target.checked; render(); });
  
  // Form submit
  const form = $('control-form');
  if (form) form.addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = STATE.controlForm;
    f.name = $('ctrl-name')?.value.trim() || '';
    f.subdomain = $('ctrl-subdomain')?.value.trim() || '';
    f.localIp = $('ctrl-ip')?.value.trim() || '192.168.1.1';
    f.localPort = $('ctrl-port')?.value || '';
    f.remotePort = $('ctrl-rport')?.value || '';
    f.nodeId = $('ctrl-node')?.value || '';
    
    if (!f.name || !f.subdomain || !f.localPort) { STATE.error = '请填写所有必填字段。'; render(); return; }
    
    const domain = getPrimaryDomain();
    const payload = {
      name: f.name,
      project_name: f.name,
      subdomain: f.subdomain,
      full_domain: `${f.subdomain}.${domain}`,
      protocol: f.protocol,
      tunnel_type: f.protocol,
      node_id: f.nodeId,
      local_ip: f.localIp,
      local_port: Number(f.localPort),
      remote_port: ['tcp','udp'].includes(f.protocol) ? Number(f.remotePort) : 0,
      desired_state: 'enabled',
      dns_domain_cname: f.subdomain,
      dns_proxied: f.cfProxied,
      cf_proxied: f.cfProxied,
    };
    
    STATE.actionBusy = 'control-save'; STATE.error = ''; render();
    try {
      const isEdit = Boolean(STATE.controlEditId);
      const url = isEdit ? `/tunnels/${encodeURIComponent(STATE.controlEditId)}` : '/tunnels';
      const method = isEdit ? 'PATCH' : 'POST';
      const res = await request(url, { method, body: JSON.stringify(payload) });
      
      // Auto-provision tunnel after save/create
      const targetId = isEdit ? STATE.controlEditId : (res?.data?.id || res?.data?.tunnel?.id);
      if (targetId) {
        await request(`/tunnels/${encodeURIComponent(targetId)}/provision`, { method: 'POST' }).catch(() => {});
      }
      
      STATE.notice = `映射 ${f.subdomain}.${domain} ${isEdit ? '已保存并触发重新部署' : '已创建成功并自动部署'}！`;
      STATE.controlModalOpen = false;
      STATE.controlEditId = null;
      await loadSnapshot();
    } catch (err) { STATE.error = `保存失败：${apiError(err)}`; }
    finally { STATE.actionBusy = ''; render(); }
  });
}

function appShell() { normalizeActivePage(); const auth = STATE.authMe; const activeMeta = PAGE_META[STATE.activePage] || PAGE_META.control; const body = auth ? `<div class="layout"><aside class="sidebar"><div class="nav-group"><div class="nav-group-title">运营模块</div><div class="nav-list">${renderNav()}</div></div></aside><main class="content"><div class="view-stack">${renderControlPage()}${renderDashboard()}${renderDNS()}${renderFRP()}${renderNodes()}${renderJobs()}${renderLogs()}${renderSettings()}</div></main></div>` : `<div class="layout anonymous-layout"><main class="content"><div class="view-stack">${`<section class="view active" data-view="login">${viewHeader('dashboard')}<div class="panel">${loginPanel()}</div></section>`}</div></main></div>`; return `<div class="shell"><header class="hero"><div class="hero-left"><div class="eyebrow">Ashan FRP Console</div><h1 class="title">${esc(activeMeta.title)}</h1><p class="subtitle">${esc(activeMeta.subtitle)}</p><div class="section-gap login-state ${auth ? 'good' : 'warn'}"><span class="dot"></span><div class="text"><strong>${auth ? `已登录：${esc(auth.display_name || auth.login_name || auth.id)}` : '尚未登录'}</strong><span>API：${esc(STATE.apiBase)} · UI：${esc(STATE.uiBase)}</span></div></div>${STATE.error ? `<div class="error-box" style="display:block">${esc(STATE.error)}</div>` : ''}${STATE.notice ? `<div class="notice-box">${esc(STATE.notice)}</div>` : ''}</div><div class="toolbar"><span class="badge fresh">版本 ${esc(STATE.version?.version || '—')}</span><span class="badge ${STATE.health?.status === 'healthy' ? 'good' : 'warn'}">服务 ${esc(STATE.health?.status || '—')}</span><button class="secondary" data-action="reload">刷新</button></div></header>${body}</div>${recoveryDialog()}`; }
function render() { const root = $(APP_ROOT_ID); if (!root) return; root.innerHTML = appShell(); document.querySelectorAll('[data-page]').forEach((button) => button.addEventListener('click', () => { STATE.activePage = button.dataset.page; render(); })); document.querySelectorAll('[data-job-id]').forEach((row) => row.addEventListener('click', () => loadSelectedJob(row.dataset.jobId))); document.querySelectorAll('[data-action]').forEach((button) => button.addEventListener('click', () => runAction(button.dataset.action, button.dataset.id))); const login = $('login-form'); if (login) login.addEventListener('submit', (event) => { event.preventDefault(); submitLogin(); }); $('forgot-password-btn')?.addEventListener('click', openRecoveryDialog); $('recovery-close-btn')?.addEventListener('click', closeRecoveryDialog); $('recovery-confirm-btn')?.addEventListener('click', closeRecoveryDialog); $('recovery-backdrop')?.addEventListener('click', (event) => { if (event.target.id === 'recovery-backdrop') closeRecoveryDialog(); }); document.querySelectorAll('[data-copy-recovery]').forEach((button) => button.addEventListener('click', () => copyRecoveryCommand(button.dataset.copyRecovery))); const cloudflareForm = $('cloudflare-settings-form-old') || $('cloudflare-settings-form'); if (cloudflareForm) cloudflareForm.addEventListener('submit', (event) => { event.preventDefault(); saveCloudflareSettings(); }); $('cloudflare-verify-btn')?.addEventListener('click', verifyCloudflareSettings); $('cloudflare-load-zones-btn-old')?.addEventListener('click', loadCloudflareZones); }
function bootHtml(message) { return `<div class="boot-screen"><div class="boot-card"><div class="boot-kicker">Ashan FRP</div><h1>${esc(message)}</h1><p>正在加载运营数据与服务状态。</p></div></div>`; }
let setupDone = false; function setup() { const root = $(APP_ROOT_ID); if (!root) { document.body.innerHTML = bootHtml('缺少页面容器'); return; } if (setupDone) return; setupDone = true; document.addEventListener('keydown', (event) => { if (event.key === 'Escape' && STATE.recoveryOpen) closeRecoveryDialog(); }); root.innerHTML = bootHtml('运营台加载中…'); loadSnapshot(); }
document.addEventListener('DOMContentLoaded', setup);

Object.assign(STATE, {
  dnsRecords: [], dnsLoaded: false, dnsLoading: false, dnsLoadError: '', dnsFilter: '', dnsTypeFilter: 'all', dnsEditor: null, dnsDeleteRecord: null, dnsDeleteName: '',
});

const DNS_EDITABLE_TYPES = ['A', 'AAAA', 'CNAME', 'TXT', 'MX', 'CAA'];
function dnsManaged(record) { return String(record?.comment || '').startsWith('ashan-frp managed:'); }
function dnsZone() { const integration = integrationState('cloudflare'); const z = integration.zone_name || integration.identifier || ''; return (z && !z.includes('.') && z.length >= 20) ? '' : z; }
function dnsConfigured() { const integration = integrationState('cloudflare'); return Boolean(integration.configured || integration.has_api_token); }
function dnsEditorRecord(record) {
  const type = String(record?.type || 'A').toUpperCase();
  const data = record?.data || {};
  return { id: record?.id || '', type, name: record?.name || '', content: record?.content || '', ttl: Number(record?.ttl || 1), proxied: Boolean(record?.proxied), priority: record?.priority ?? '', caa: { flags: data.flags ?? 0, tag: data.tag || 'issue', value: data.value || '' }, managed: dnsManaged(record) };
}
function dnsFilteredRecords() {
  const needle = String(STATE.dnsFilter || '').trim().toLowerCase();
  return safeArray(STATE.dnsRecords).filter((record) => {
    const typeMatches = STATE.dnsTypeFilter === 'all' || record.type === STATE.dnsTypeFilter;
    const textMatches = !needle || [record.name, record.type, record.content, record.comment].some((value) => String(value || '').toLowerCase().includes(needle));
    return typeMatches && textMatches;
  });
}
function renderDNSRecordForm() {
  const editor = STATE.dnsEditor;
  if (!editor) return '';
  const isCAA = editor.type === 'CAA'; const isMX = editor.type === 'MX'; const canProxy = ['A','AAAA','CNAME'].includes(editor.type);
  return `<div class="dns-modal-backdrop"><section class="dns-modal" role="dialog" aria-modal="true" aria-labelledby="dns-editor-title"><div class="panel-head"><div><div class="eyebrow">CLOUDFLARE DNS</div><h3 id="dns-editor-title">${editor.id ? '编辑 DNS 记录' : '新增 DNS 记录'}</h3></div><button class="icon-button" data-dns-action="close-editor" aria-label="关闭">×</button></div>${editor.managed ? '<div class="dns-warning">该记录由隧道管理。保存后，后续隧道部署可能再次覆盖此记录。</div>' : ''}<form id="dns-record-form" class="integration-form"><label>记录类型<select id="dns-record-type">${DNS_EDITABLE_TYPES.map((type) => `<option value="${type}" ${editor.type === type ? 'selected' : ''}>${type}</option>`).join('')}</select></label><label>名称<input id="dns-record-name" value="${esc(editor.name)}" placeholder="www.example.com" required /></label>${isCAA ? `<div class="dns-field-grid"><label>Flags<input id="dns-caa-flags" type="number" min="0" max="255" value="${esc(editor.caa.flags)}" required /></label><label>Tag<select id="dns-caa-tag">${['issue','issuewild','iodef'].map((tag) => `<option value="${tag}" ${editor.caa.tag === tag ? 'selected' : ''}>${tag}</option>`).join('')}</select></label></div><label>Value<input id="dns-caa-value" value="${esc(editor.caa.value)}" placeholder="letsencrypt.org" required /></label>` : `<label>内容<input id="dns-record-content" value="${esc(editor.content)}" placeholder="${editor.type === 'TXT' ? 'TXT 内容' : '目标地址或主机名'}" required /></label>`}${isMX ? '<label>优先级<input id="dns-record-priority" type="number" min="0" max="65535" value="' + esc(editor.priority) + '" required /></label>' : ''}<div class="dns-field-grid"><label>TTL<select id="dns-record-ttl">${[[1,'自动'],[60,'1 分钟'],[300,'5 分钟'],[600,'10 分钟'],[3600,'1 小时'],[86400,'1 天']].map(([value,label]) => `<option value="${value}" ${Number(editor.ttl) === value ? 'selected' : ''}>${label}</option>`).join('')}</select></label>${canProxy ? `<label class="checkbox-label"><input id="dns-record-proxied" type="checkbox" ${editor.proxied ? 'checked' : ''} />启用 Cloudflare 代理</label>` : '<div class="muted dns-proxy-note">此记录类型不支持代理开关。</div>'}</div><div class="settings-actions"><button type="submit">${editor.id ? '保存修改' : '创建记录'}</button><button type="button" class="secondary" data-dns-action="close-editor">取消</button></div></form></section></div>`;
}
function renderDNSDeleteDialog() {
  const record = STATE.dnsDeleteRecord; if (!record) return '';
  return `<div class="dns-modal-backdrop"><section class="dns-modal dns-delete-modal" role="dialog" aria-modal="true"><div class="panel-head"><div><div class="eyebrow">DESTRUCTIVE ACTION</div><h3>删除 DNS 记录</h3></div><button class="icon-button" data-dns-action="close-delete" aria-label="关闭">×</button></div><p>将删除 <strong>${esc(record.type)} ${esc(record.name)}</strong>，内容为 <span class="mono">${esc(record.content || record.data?.value || '—')}</span>。</p>${dnsManaged(record) ? '<div class="dns-warning">这是由隧道管理的记录。删除后，后续隧道部署可能重新创建它。</div>' : ''}<form id="dns-delete-form" class="integration-form"><label>输入完整记录名称以确认<input id="dns-delete-name" value="${esc(STATE.dnsDeleteName)}" autocomplete="off" placeholder="${esc(record.name)}" required /></label><div class="settings-actions"><button type="submit" class="danger" ${STATE.dnsDeleteName === record.name ? '' : 'disabled'}>确认删除</button><button type="button" class="secondary" data-dns-action="close-delete">取消</button></div></form></section></div>`;
}
function renderDNS() {
  const zone = dnsZone(); const configured = dnsConfigured(); const records = dnsFilteredRecords();
  const types = ['all', ...new Set(safeArray(STATE.dnsRecords).map((record) => record.type).filter(Boolean))];
  const actions = configured ? `<button data-dns-action="refresh" class="secondary">${STATE.dnsLoading ? '加载中…' : '刷新记录'}</button><button data-dns-action="new">新增记录</button>` : '<button class="secondary" data-dns-action="open-settings">前往配置 Cloudflare</button>';
  const rows = records.map((record) => { const editable = DNS_EDITABLE_TYPES.includes(record.type); const content = record.type === 'CAA' ? `${record.data?.flags ?? 0} ${record.data?.tag || 'issue'} ${record.data?.value || ''}` : record.content; return `<tr><td><strong>${esc(record.name)}</strong>${dnsManaged(record) ? '<small class="managed-tag">由隧道管理</small>' : ''}</td><td>${statusBadge(record.type)}</td><td class="mono dns-content">${esc(content || '—')}</td><td>${record.proxiable ? (record.proxied ? statusBadge('proxied') : statusBadge('dns only')) : '—'}</td><td>${record.ttl === 1 ? '自动' : `${esc(record.ttl || '—')} 秒`}</td><td>${editable ? `<div class="dns-row-actions"><button class="secondary tiny-btn" data-dns-action="edit" data-dns-id="${esc(record.id)}">编辑</button><button class="ghost tiny-btn" data-dns-action="delete" data-dns-id="${esc(record.id)}">删除</button></div>` : '<span class="muted">只读</span>'}</td></tr>`; });
  let body = '';
  if (!configured) body = emptyState('尚未配置 Cloudflare', '请先在“设置”中保存 Zone 和 API Token，再读取该 Zone 的真实 DNS 记录。');
  else if (STATE.dnsLoadError) body = `<div class="dns-error">${esc(STATE.dnsLoadError)}</div>${emptyState('无法读取 DNS 记录', '请检查 Token、Zone 和 DNS Read 权限，然后重新验证或刷新。')}`;
  else body = `<div class="dns-toolbar"><label>搜索<input id="dns-filter" value="${esc(STATE.dnsFilter)}" placeholder="按名称、内容或类型搜索" /></label><label>类型<select id="dns-type-filter">${types.map((type) => `<option value="${esc(type)}" ${STATE.dnsTypeFilter === type ? 'selected' : ''}>${type === 'all' ? '全部类型' : esc(type)}</option>`).join('')}</select></label><span class="muted">共 ${safeArray(STATE.dnsRecords).length} 条，显示 ${records.length} 条</span></div>${renderTable(['名称','类型','内容','代理','TTL','操作'], rows, '该 Zone 暂无 DNS 记录', 'Cloudflare 返回的记录会直接显示在这里。')}`;
  return pageCard('dns', `${viewHeader('dns', actions)}<div class="panel"><div class="panel-head"><div><h3>Cloudflare DNS 记录</h3><span class="muted">Zone：${esc(zone || '未配置')} · ${configured ? 'Token 不会在页面中显示' : '等待配置'}</span></div>${configured ? statusBadge(STATE.dnsLoadError ? 'error' : 'connected') : statusBadge('not configured')}</div>${body}</div>${renderDNSRecordForm()}${renderDNSDeleteDialog()}`);
}
async function loadDNSRecords(force = false) {
  if (!dnsConfigured()) { STATE.dnsLoaded = true; STATE.dnsLoadError = ''; render(); return; }
  if (STATE.dnsLoading || (STATE.dnsLoaded && !force)) return;
  STATE.dnsLoading = true; STATE.dnsLoadError = ''; render();
  try { const response = await request('/dns/records'); STATE.dnsRecords = safeArray(response?.data?.records); STATE.dnsLoaded = true; }
  catch (err) { STATE.dnsRecords = []; STATE.dnsLoaded = true; STATE.dnsLoadError = apiError(err); }
  finally { STATE.dnsLoading = false; render(); }
}
function setDNSPage(page) { STATE.activePage = page; render(); if (page === 'dns') loadDNSRecords(); }
function currentDNSRecord(id) { return safeArray(STATE.dnsRecords).find((record) => record.id === id) || null; }
function dnsInputFromForm() { const editor = STATE.dnsEditor; const type = $('dns-record-type')?.value || editor.type; const payload = { type, name: $('dns-record-name')?.value.trim() || '', ttl: Number($('dns-record-ttl')?.value || 1) }; if (type === 'CAA') payload.caa = { flags: Number($('dns-caa-flags')?.value || 0), tag: $('dns-caa-tag')?.value || '', value: $('dns-caa-value')?.value.trim() || '' }; else payload.content = $('dns-record-content')?.value.trim() || ''; if (type === 'MX') payload.priority = Number($('dns-record-priority')?.value); if (['A','AAAA','CNAME'].includes(type)) payload.proxied = Boolean($('dns-record-proxied')?.checked); return payload; }
async function submitDNSRecord() { const editor = STATE.dnsEditor; const payload = dnsInputFromForm(); STATE.actionBusy = editor.id ? 'dns-update' : 'dns-create'; STATE.error = ''; render(); try { await request(editor.id ? `/dns/records/${encodeURIComponent(editor.id)}` : '/dns/records', { method: editor.id ? 'PATCH' : 'POST', body: JSON.stringify(payload) }); STATE.notice = editor.id ? 'DNS 记录已更新。' : 'DNS 记录已创建。'; STATE.dnsEditor = null; STATE.dnsLoaded = false; await loadDNSRecords(true); } catch (err) { STATE.error = apiError(err); } finally { STATE.actionBusy = ''; render(); } }
async function submitDNSDelete() { const record = STATE.dnsDeleteRecord; if (!record || STATE.dnsDeleteName !== record.name) return; STATE.actionBusy = 'dns-delete'; STATE.error = ''; render(); try { await request(`/dns/records/${encodeURIComponent(record.id)}`, { method: 'DELETE' }); STATE.notice = 'DNS 记录已删除。'; STATE.dnsDeleteRecord = null; STATE.dnsDeleteName = ''; STATE.dnsLoaded = false; await loadDNSRecords(true); } catch (err) { STATE.error = apiError(err); } finally { STATE.actionBusy = ''; render(); } }
function bindDNSUI() {
  if (STATE.activePage === 'dns' && !STATE.dnsLoaded && !STATE.dnsLoading) loadDNSRecords();
  document.querySelectorAll('[data-dns-action]').forEach((button) => button.addEventListener('click', () => { const action = button.dataset.dnsAction; const record = currentDNSRecord(button.dataset.dnsId); if (action === 'refresh') loadDNSRecords(true); if (action === 'new') { STATE.dnsEditor = dnsEditorRecord(null); render(); } if (action === 'edit' && record) { STATE.dnsEditor = dnsEditorRecord(record); render(); } if (action === 'delete' && record) { STATE.dnsDeleteRecord = record; STATE.dnsDeleteName = ''; render(); } if (action === 'close-editor') { STATE.dnsEditor = null; render(); } if (action === 'close-delete') { STATE.dnsDeleteRecord = null; STATE.dnsDeleteName = ''; render(); } if (action === 'open-settings') setDNSPage('settings'); }));
  $('dns-filter')?.addEventListener('input', (event) => { STATE.dnsFilter = event.target.value; render(); }); $('dns-type-filter')?.addEventListener('change', (event) => { STATE.dnsTypeFilter = event.target.value; render(); }); $('dns-record-type')?.addEventListener('change', (event) => { STATE.dnsEditor.type = event.target.value; render(); }); const form = $('dns-record-form'); if (form) form.addEventListener('submit', (event) => { event.preventDefault(); submitDNSRecord(); }); $('dns-delete-name')?.addEventListener('input', (event) => { STATE.dnsDeleteName = event.target.value; const submit = $('dns-delete-form button[type="submit"]'); if (submit) submit.disabled = STATE.dnsDeleteName !== STATE.dnsDeleteRecord?.name; }); const deleteForm = $('dns-delete-form'); if (deleteForm) deleteForm.addEventListener('submit', (event) => { event.preventDefault(); submitDNSDelete(); });
}
const ashanBaseRender = render;
render = function renderWithDNS() { ashanBaseRender(); bindDNSUI(); };

Object.assign(STATE, { auditFilter: '', auditOutcome: 'all' });
const baseRenderLogsSecure = renderLogs;
renderLogs = function renderSecureLogs() {
  const query = String(STATE.auditFilter || '').toLowerCase();
  const logs = safeArray(STATE.auditLogs).filter((item) => {
    if (STATE.auditOutcome !== 'all' && String(item.outcome || '') !== STATE.auditOutcome) return false;
    return !query || [item.action,item.account_name,item.request_id,item.trace_id,item.error_code,item.credential_ref,item.resource_id].some((value) => String(value || '').toLowerCase().includes(query));
  });
  const rows = logs.map((item) => {
    let detail = item.detail_json || '';
    try { detail = JSON.stringify(JSON.parse(detail), null, 2); } catch {}
    return '<tr><td>'+esc(fmtTime(item.created_at))+'</td><td><strong>'+esc(item.action || 'system')+'</strong><details><summary>查看详情</summary><pre class="audit-detail">'+esc(detail || '无安全详情')+'</pre></details></td><td>'+statusBadge(item.outcome || 'unknown')+'</td><td>'+esc(item.duration_ms || 0)+' ms</td><td class="mono">'+esc(item.request_id || '—')+'</td><td class="mono">'+esc(item.credential_ref || '—')+'</td><td>'+esc(item.error_code || '—')+'</td></tr>';
  });
  const toolbar = '<div class="audit-toolbar"><label>搜索<input id="audit-filter" value="'+esc(STATE.auditFilter)+'" placeholder="动作、请求 ID、错误码或凭据指纹" /></label><label>结果<select id="audit-outcome"><option value="all">全部</option><option value="success" '+(STATE.auditOutcome==='success'?'selected':'')+'>成功</option><option value="failure" '+(STATE.auditOutcome==='failure'?'selected':'')+'>失败</option></select></label><span class="muted">显示 '+logs.length+' / '+safeArray(STATE.auditLogs).length+' 条</span></div>';
  return pageCard('logs', viewHeader('logs', actionButton('刷新日志','reload',{secondary:true}))+'<div class="panel"><div class="panel-head"><div><h3>安全审计日志</h3><span class="muted">仅展示白名单安全字段；Token、密码、授权头和 Cookie 永不显示。</span></div></div>'+toolbar+renderTable(['时间','动作与详情','结果','耗时','请求 ID','凭据指纹','错误码'],rows,'暂无审计记录','登录、配置、验证、同步和部署操作会在这里留下可关联记录。')+'</div>');
};
const baseCloudflareSettingsSecure = renderCloudflareSettings;
renderCloudflareSettings = function renderCloudflareSettingsWithAudit() {
  const item = integrationState('cloudflare');
  const identity = '<div class="credential-identity"><span>Token 掩码 <strong>'+esc(item.token_mask || item.mask_hint || '未配置')+'</strong></span><span>凭据指纹 <code>'+esc(item.credential_ref || '—')+'</code></span><span>版本 '+esc(item.credential_revision || 0)+'</span><button type="button" class="secondary" id="cloudflare-audit-btn">查看验证记录</button></div>';
  return baseCloudflareSettingsSecure()+identity;
};
function bindAuditUI() {
  $('audit-filter')?.addEventListener('input', (event) => { STATE.auditFilter = event.target.value; render(); });
  $('audit-outcome')?.addEventListener('change', (event) => { STATE.auditOutcome = event.target.value; render(); });
  $('cloudflare-audit-btn')?.addEventListener('click', () => { STATE.auditFilter = 'cloudflare.credential'; STATE.auditOutcome = 'all'; STATE.activePage = 'logs'; render(); });
}
const renderWithAuditBase = render;
render = function renderWithAudit() { renderWithAuditBase(); bindAuditUI(); };

const renderWithControlBase = render;
render = function renderWithControl() { renderWithControlBase(); bindControlUI(); };


Object.assign(STATE, { dnsOpenGroup: '', dnsCNAMEFromTunnel: false });

const DNS_PHASE2_TEXT = {
  ungrouped: '\u672a\u5206\u7ec4',
  unboundNode: '\u672a\u7ed1\u5b9a\u8282\u70b9',
  managed: '\u53d7\u7a7f\u900f\u7ba1\u7406',
  managedTitle: '\u53d7\u7a7f\u900f\u7ba1\u7406',
  backendManaged: '\u540e\u7aef\u6258\u7ba1\u8bb0\u5f55',
  fromTunnel: '\u4ece\u96a7\u9053\u521b\u5efa CNAME',
  refresh: '\u5237\u65b0\u8bb0\u5f55',
  loading: '\u52a0\u8f7d\u4e2d\u2026',
  newRecord: '\u65b0\u589e\u8bb0\u5f55',
  openSettings: '\u524d\u5f80\u914d\u7f6e Cloudflare',
  setupTitle: '\u5c1a\u672a\u914d\u7f6e Cloudflare',
  setupCopy: '\u4fdd\u5b58 Zone \u548c API Token \u540e\uff0c\u6b64\u9875\u4f1a\u8bfb\u53d6\u771f\u5b9e Cloudflare DNS \u8bb0\u5f55\uff0c\u5e76\u6309\u4e3b\u57df\u540d\u6298\u53e0\u5c55\u793a\u3002',
  setupSteps: ['\u5728\u8bbe\u7f6e\u4e2d\u586b\u5199 Zone \u57df\u540d\u548c API Token\u3002','\u6267\u884c\u8bfb\u53d6\u9a8c\u8bc1\uff0c\u786e\u8ba4 DNS Read \u6743\u9650\u53ef\u7528\u3002','\u9700\u8981\u5199\u5165\u65f6\u518d\u542f\u7528 DNS Edit \u6743\u9650\u3002'],
  target: '\u76ee\u6807',
  node: '\u8282\u70b9',
  seconds: '\u79d2',
  auto: '\u81ea\u52a8',
  edit: '\u7f16\u8f91',
  delete: '\u5220\u9664',
  readonly: '\u53ea\u8bfb',
  records: '\u8bb0\u5f55',
  matched: '\u6761\u5339\u914d\u7b5b\u9009',
  groups: '\u4e3b\u57df\u540d\u5206\u7ec4',
  proxied: 'CDN \u4ee3\u7406',
  search: '\u641c\u7d22',
  type: '\u7c7b\u578b',
  allTypes: '\u5168\u90e8\u7c7b\u578b',
  focusTitle: 'Cloudflare DNS \u4e13\u6ce8\u89c6\u56fe',
  focusHint: '\u6309\u4e3b\u57df\u540d\u6298\u53e0\u5206\u7ec4\uff0c\u5e76\u81ea\u52a8\u5173\u8054\u7a7f\u900f\u96a7\u9053\u3002',
  noRecords: '\u8be5 Zone \u6682\u65e0 DNS \u8bb0\u5f55',
  noRecordsHint: 'Cloudflare \u8fd4\u56de\u7684\u8bb0\u5f55\u4f1a\u76f4\u63a5\u663e\u793a\u5728\u8fd9\u91cc\u3002',
  loadFail: '\u65e0\u6cd5\u8bfb\u53d6 DNS \u8bb0\u5f55',
  loadFailHint: '\u8bf7\u68c0\u67e5 Token\u3001Zone \u548c DNS Read \u6743\u9650\uff0c\u7136\u540e\u91cd\u65b0\u9a8c\u8bc1\u6216\u5237\u65b0\u3002',
  tunnelPick: '\u9009\u62e9\u96a7\u9053',
  noTunnel: '\u6682\u65e0\u53ef\u7528\u96a7\u9053',
  quickHint: '\u5feb\u6377\u6a21\u5f0f\u4f1a\u4ece\u96a7\u9053\u81ea\u52a8\u586b\u5165 CNAME \u540d\u79f0\u3001\u76ee\u6807\u548c CDN \u4ee3\u7406\u5f00\u5173\uff1b\u4fdd\u5b58\u540e\u4ecd\u901a\u8fc7 Cloudflare DNS API \u521b\u5efa\u771f\u5b9e\u8bb0\u5f55\u3002',
  editTitle: '\u7f16\u8f91 DNS \u8bb0\u5f55',
  createTitle: '\u65b0\u589e DNS \u8bb0\u5f55',
  close: '\u5173\u95ed',
  managedWarning: '\u8be5\u8bb0\u5f55\u7531\u96a7\u9053\u7ba1\u7406\u3002\u4fdd\u5b58\u540e\uff0c\u540e\u7eed\u96a7\u9053\u90e8\u7f72\u53ef\u80fd\u518d\u6b21\u8986\u76d6\u6b64\u8bb0\u5f55\u3002',
  recordType: '\u8bb0\u5f55\u7c7b\u578b',
  name: '\u540d\u79f0',
  content: '\u5185\u5bb9',
  txtPlaceholder: 'TXT \u5185\u5bb9',
  targetPlaceholder: '\u76ee\u6807\u5730\u5740\u6216\u4e3b\u673a\u540d',
  priority: '\u4f18\u5148\u7ea7',
  minute1: '1 \u5206\u949f',
  minute5: '5 \u5206\u949f',
  minute10: '10 \u5206\u949f',
  hour1: '1 \u5c0f\u65f6',
  day1: '1 \u5929',
  enableProxy: '\u542f\u7528 Cloudflare \u4ee3\u7406',
  noProxy: '\u6b64\u8bb0\u5f55\u7c7b\u578b\u4e0d\u652f\u6301\u4ee3\u7406\u5f00\u5173\u3002',
  save: '\u4fdd\u5b58\u4fee\u6539',
  create: '\u521b\u5efa\u8bb0\u5f55',
  cancel: '\u53d6\u6d88',
  recordName: '\u540d\u79f0',
  recordContent: '\u5185\u5bb9',
  proxy: '\u4ee3\u7406',
  operation: '\u64cd\u4f5c',
  emptyTitle: '\u6682\u65e0\u5339\u914d DNS \u8bb0\u5f55',
  emptyHint: '\u5c1d\u8bd5\u8c03\u6574\u641c\u7d22\u5173\u952e\u8bcd\u6216\u7c7b\u578b\u7b5b\u9009\u3002',
  filterPlaceholder: '\u6309\u540d\u79f0\u3001\u5185\u5bb9\u6216\u7c7b\u578b\u641c\u7d22',
  tokenSafe: 'Token \u4e0d\u4f1a\u5728\u9875\u9762\u4e2d\u663e\u793a',
  dot: '\u00b7',
  arrow: '\u2192',
  dash: '\u2014',
  closeSymbol: '\u00d7'
};

function dnsCanonicalName(value) { return String(value || '').trim().replace(/\.$/, '').toLowerCase(); }
function dnsRootForName(name) {
  const canonical = dnsCanonicalName(name);
  const zone = dnsCanonicalName(dnsZone());
  if (zone && (canonical === zone || canonical.endsWith(`.${zone}`))) return zone;
  const parts = canonical.split('.').filter(Boolean);
  return parts.length >= 2 ? parts.slice(-2).join('.') : canonical || DNS_PHASE2_TEXT.ungrouped;
}
function dnsNodeLabel(nodeID) {
  const node = safeArray(STATE.nodes).find((item) => item.id === nodeID);
  return node?.display_name || node?.canonical_name || node?.name || node?.id || DNS_PHASE2_TEXT.unboundNode;
}
function dnsTunnelDomain(tunnel) { return tunnel?.full_domain || tunnel?.dns_domain_cname || ''; }
function dnsTunnelTarget(tunnel) { return `${tunnel?.local_ip || '127.0.0.1'}:${tunnel?.local_port || '?'}`; }
function dnsTunnelOptions() { return safeArray(STATE.tunnels).filter((tunnel) => dnsTunnelDomain(tunnel)).sort((a, b) => dnsTunnelDomain(a).localeCompare(dnsTunnelDomain(b))); }
function dnsTunnelCNAMEContent(tunnel) {
  const domain = dnsCanonicalName(dnsTunnelDomain(tunnel));
  const candidates = [tunnel?.dns_target, tunnel?.cname_target, tunnel?.dns_record_content, tunnel?.cloudflare_target, tunnel?.dns_domain_cname];
  return candidates.find((value) => value && String(value).includes('.') && dnsCanonicalName(value) !== domain) || '';
}
function dnsTunnelBinding(record) {
  const recordName = dnsCanonicalName(record?.name);
  const comment = String(record?.comment || '').toLowerCase();
  return dnsTunnelOptions().find((tunnel) => {
    if (record?.id && tunnel.cf_record_id && record.id === tunnel.cf_record_id) return true;
    if ([tunnel.id, tunnel.name, tunnel.chmlfrp_tunnel_id, tunnel.chmlfrp_tunnel_name].some((value) => value && comment.includes(String(value).toLowerCase()))) return true;
    return [tunnel.full_domain, tunnel.dns_domain_cname].some((domain) => dnsCanonicalName(domain) === recordName);
  }) || null;
}
function dnsRecordContent(record) {
  if (record?.type === 'CAA') return `${record.data?.flags ?? 0} ${record.data?.tag || 'issue'} ${record.data?.value || ''}`;
  return record?.content || record?.data?.value || '';
}
function dnsManagedMeta(record) {
  const tunnel = dnsTunnelBinding(record);
  const managed = dnsManaged(record) || Boolean(tunnel);
  if (!managed) return { managed: false, tunnel: null, title: '' };
  const target = tunnel ? dnsTunnelTarget(tunnel) : DNS_PHASE2_TEXT.backendManaged;
  const node = tunnel ? dnsNodeLabel(tunnel.node_id) : DNS_PHASE2_TEXT.dash;
  return { managed, tunnel, title: `${DNS_PHASE2_TEXT.managedTitle}: ${tunnel?.name || record?.name || 'record'} ${DNS_PHASE2_TEXT.arrow} ${target} ${DNS_PHASE2_TEXT.dot} ${node}` };
}
function dnsGroupedRecords(records) {
  const groups = new Map();
  for (const record of records) {
    const key = dnsRootForName(record.name || '');
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(record);
  }
  return [...groups.entries()]
    .map(([key, groupRecords]) => ({ key, records: groupRecords.sort((a, b) => String(a.name || '').localeCompare(String(b.name || ''))) }))
    .sort((a, b) => a.key.localeCompare(b.key));
}
function dnsGroupOpen(group, index) { return STATE.dnsOpenGroup ? STATE.dnsOpenGroup === group.key : index === 0; }

const dnsBaseEditorRecord = dnsEditorRecord;
dnsEditorRecord = function dnsEditorRecordPhase2(record, options = {}) {
  const editor = dnsBaseEditorRecord(record);
  editor.fromTunnel = Boolean(options.fromTunnel);
  editor.tunnelId = options.tunnelId || '';
  if (editor.fromTunnel) {
    const tunnel = dnsTunnelOptions().find((item) => item.id === editor.tunnelId) || dnsTunnelOptions()[0];
    if (tunnel) Object.assign(editor, { type: 'CNAME', name: dnsTunnelDomain(tunnel), content: dnsTunnelCNAMEContent(tunnel), proxied: Boolean(tunnel.cf_proxied || tunnel.dns_proxied), tunnelId: tunnel.id });
  }
  return editor;
};
function syncDNSCNAMEFromTunnel(tunnelID) {
  const tunnel = dnsTunnelOptions().find((item) => item.id === tunnelID);
  if (!tunnel || !STATE.dnsEditor) return;
  Object.assign(STATE.dnsEditor, { type: 'CNAME', name: dnsTunnelDomain(tunnel), content: dnsTunnelCNAMEContent(tunnel), proxied: Boolean(tunnel.cf_proxied || tunnel.dns_proxied), tunnelId: tunnel.id });
}

renderDNSRecordForm = function renderDNSRecordFormPhase2() {
  const editor = STATE.dnsEditor;
  if (!editor) return '';
  const isCAA = editor.type === 'CAA';
  const isMX = editor.type === 'MX';
  const canProxy = ['A','AAAA','CNAME'].includes(editor.type);
  const tunnelOptions = dnsTunnelOptions();
  const ttlOptions = [[1,DNS_PHASE2_TEXT.auto],[60,DNS_PHASE2_TEXT.minute1],[300,DNS_PHASE2_TEXT.minute5],[600,DNS_PHASE2_TEXT.minute10],[3600,DNS_PHASE2_TEXT.hour1],[86400,DNS_PHASE2_TEXT.day1]];
  const tunnelPicker = editor.fromTunnel ? `<label>${DNS_PHASE2_TEXT.tunnelPick}<select id="dns-tunnel-source">${tunnelOptions.length ? tunnelOptions.map((tunnel) => `<option value="${esc(tunnel.id)}" ${editor.tunnelId === tunnel.id ? 'selected' : ''}>${esc(dnsTunnelDomain(tunnel))} ${DNS_PHASE2_TEXT.arrow} ${esc(dnsTunnelTarget(tunnel))} ${DNS_PHASE2_TEXT.dot} ${esc(dnsNodeLabel(tunnel.node_id))}</option>`).join('') : `<option value="">${DNS_PHASE2_TEXT.noTunnel}</option>`}</select></label>` : '';
  const tunnelHint = editor.fromTunnel ? `<div class="dns-warning">${DNS_PHASE2_TEXT.quickHint}</div>` : '';
  const title = editor.fromTunnel ? DNS_PHASE2_TEXT.fromTunnel : (editor.id ? DNS_PHASE2_TEXT.editTitle : DNS_PHASE2_TEXT.createTitle);
  const contentField = isCAA ? `<div class="dns-field-grid"><label>Flags<input id="dns-caa-flags" type="number" min="0" max="255" value="${esc(editor.caa.flags)}" required /></label><label>Tag<select id="dns-caa-tag">${['issue','issuewild','iodef'].map((tag) => `<option value="${tag}" ${editor.caa.tag === tag ? 'selected' : ''}>${tag}</option>`).join('')}</select></label></div><label>Value<input id="dns-caa-value" value="${esc(editor.caa.value)}" placeholder="letsencrypt.org" required /></label>` : `<label>${DNS_PHASE2_TEXT.content}<input id="dns-record-content" value="${esc(editor.content)}" placeholder="${editor.type === 'TXT' ? DNS_PHASE2_TEXT.txtPlaceholder : DNS_PHASE2_TEXT.targetPlaceholder}" required /></label>`;
  const zoneStr = dnsZone() || '';
  let shortName = editor.name || '';
  if (zoneStr && shortName.endsWith('.' + zoneStr)) {
    shortName = shortName.slice(0, -(zoneStr.length + 1));
  } else if (shortName === zoneStr) {
    shortName = '@';
  }
  const zoneNameDisplay = zoneStr ? `.${esc(zoneStr)}` : '';
  const nameField = `<label>${DNS_PHASE2_TEXT.name}<div class="dns-name-input-wrapper" style="margin-top:8px;"><input id="dns-record-name" value="${esc(shortName)}" placeholder="例如: www 或 @" required /><span class="dns-name-suffix">${zoneNameDisplay}</span></div></label>`;
  return `<div class="dns-modal-backdrop"><section class="dns-modal" role="dialog" aria-modal="true" aria-labelledby="dns-editor-title"><div class="panel-head"><div><div class="eyebrow">CLOUDFLARE DNS</div><h3 id="dns-editor-title">${title}</h3></div><button class="icon-button" data-dns-action="close-editor" aria-label="${DNS_PHASE2_TEXT.close}">${DNS_PHASE2_TEXT.closeSymbol}</button></div>${editor.managed && !editor.fromTunnel ? `<div class="dns-warning">${DNS_PHASE2_TEXT.managedWarning}</div>` : ''}${tunnelHint}<form id="dns-record-form" class="integration-form">${tunnelPicker}<label>${DNS_PHASE2_TEXT.recordType}<select id="dns-record-type">${DNS_EDITABLE_TYPES.map((type) => `<option value="${type}" ${editor.type === type ? 'selected' : ''}>${type}</option>`).join('')}</select></label>${nameField}${contentField}${isMX ? `<label>${DNS_PHASE2_TEXT.priority}<input id="dns-record-priority" type="number" min="0" max="65535" value="${esc(editor.priority)}" required /></label>` : ''}<div class="dns-field-grid"><label>TTL<select id="dns-record-ttl">${ttlOptions.map(([value,label]) => `<option value="${value}" ${Number(editor.ttl) === value ? 'selected' : ''}>${label}</option>`).join('')}</select></label>${canProxy ? `<label class="checkbox-label"><input id="dns-record-proxied" type="checkbox" ${editor.proxied ? 'checked' : ''} />${DNS_PHASE2_TEXT.enableProxy}</label>` : `<div class="muted dns-proxy-note">${DNS_PHASE2_TEXT.noProxy}</div>`}</div><div class="settings-actions"><button type="submit">${editor.id ? DNS_PHASE2_TEXT.save : DNS_PHASE2_TEXT.create}</button><button type="button" class="secondary" data-dns-action="close-editor">${DNS_PHASE2_TEXT.cancel}</button></div></form></section></div>`;
};

renderDNS = function renderDNSPhase2() {
  const zone = dnsZone();
  const configured = dnsConfigured();
  const records = dnsFilteredRecords();
  const managedCount = records.filter((record) => dnsManagedMeta(record).managed).length;
  const rawTypes = safeArray(STATE.dnsRecords).map((record) => record.type).filter(Boolean).sort();
  const types = ['all', ...new Set(rawTypes)];
  const actions = configured ? `<button data-dns-action="refresh" class="secondary">${STATE.dnsLoading ? DNS_PHASE2_TEXT.loading : DNS_PHASE2_TEXT.refresh}</button><button data-dns-action="from-tunnel" class="secondary">${DNS_PHASE2_TEXT.fromTunnel}</button><button data-dns-action="new">${DNS_PHASE2_TEXT.newRecord}</button>` : `<button class="secondary" data-dns-action="open-settings">${DNS_PHASE2_TEXT.openSettings}</button>`;
  let body = '';
  if (!configured) {
    body = `<div class="dns-setup-card"><div class="dns-setup-icon">CF</div><div><h3>${DNS_PHASE2_TEXT.setupTitle}</h3><p>${DNS_PHASE2_TEXT.setupCopy}</p><ol>${DNS_PHASE2_TEXT.setupSteps.map((step) => `<li>${step}</li>`).join('')}</ol></div></div>`;
  } else if (STATE.dnsLoadError) {
    body = emptyState(DNS_PHASE2_TEXT.loadFail, `${DNS_PHASE2_TEXT.loadFailHint} ${esc(STATE.dnsLoadError)}`);
  } else {
    const groups = dnsGroupedRecords(records);
    const groupHTML = groups.map((group, index) => {
      const groupManaged = group.records.filter((record) => dnsManagedMeta(record).managed).length;
      const rows = group.records.map((record) => {
        const meta = dnsManagedMeta(record);
        const content = dnsRecordContent(record);
        const binding = meta.tunnel ? `<div class="muted">${DNS_PHASE2_TEXT.target}: <span class="mono">${esc(dnsTunnelTarget(meta.tunnel))}</span> ${DNS_PHASE2_TEXT.dot} ${DNS_PHASE2_TEXT.node}: ${esc(dnsNodeLabel(meta.tunnel.node_id))}</div>` : '';
        const editable = DNS_EDITABLE_TYPES.includes(record.type);
        const canProxy = record.proxiable || ['A','AAAA','CNAME'].includes(record.type);
        return `<tr><td><strong>${esc(record.name)}</strong>${meta.managed ? `<small class="managed-tag" title="${esc(meta.title)}">${DNS_PHASE2_TEXT.managed}</small>` : ''}${binding}</td><td>${statusBadge(record.type)}</td><td class="mono dns-content">${esc(content || DNS_PHASE2_TEXT.dash)}</td><td>${canProxy ? (record.proxied ? statusBadge('proxied') : statusBadge('dns only')) : DNS_PHASE2_TEXT.dash}</td><td>${record.ttl === 1 ? DNS_PHASE2_TEXT.auto : `${esc(record.ttl || DNS_PHASE2_TEXT.dash)} ${DNS_PHASE2_TEXT.seconds}`}</td><td>${editable ? `<div class="dns-row-actions"><button class="secondary tiny-btn" data-dns-action="edit" data-dns-id="${esc(record.id)}">${DNS_PHASE2_TEXT.edit}</button><button class="ghost tiny-btn" data-dns-action="delete" data-dns-id="${esc(record.id)}">${DNS_PHASE2_TEXT.delete}</button></div>` : `<span class="muted">${DNS_PHASE2_TEXT.readonly}</span>`}</td></tr>`;
      });
      return `<details class="dns-accordion" data-dns-group-key="${esc(group.key)}" ${dnsGroupOpen(group, index) ? 'open' : ''}><summary><span><strong>${esc(group.key)}</strong><small>${group.records.length} ${DNS_PHASE2_TEXT.records} ${DNS_PHASE2_TEXT.dot} ${groupManaged} ${DNS_PHASE2_TEXT.managed}</small></span><span class="dns-chevron">v</span></summary><div class="dns-group-records">${renderTable([DNS_PHASE2_TEXT.recordName,DNS_PHASE2_TEXT.type,DNS_PHASE2_TEXT.recordContent,DNS_PHASE2_TEXT.proxy,'TTL',DNS_PHASE2_TEXT.operation], rows, DNS_PHASE2_TEXT.emptyTitle, DNS_PHASE2_TEXT.emptyHint)}</div></details>`;
    }).join('');
    body = `<div class="metric-grid compact dns-summary-grid">${metric('DNS ' + DNS_PHASE2_TEXT.records, safeArray(STATE.dnsRecords).length, `${records.length} ${DNS_PHASE2_TEXT.matched}`)}${metric(DNS_PHASE2_TEXT.groups, groups.length)}${metric(DNS_PHASE2_TEXT.managed, managedCount)}${metric(DNS_PHASE2_TEXT.proxied, records.filter((record) => record.proxied).length)}</div><div class="dns-toolbar"><label>${DNS_PHASE2_TEXT.search}<input id="dns-filter" value="${esc(STATE.dnsFilter)}" placeholder="${DNS_PHASE2_TEXT.filterPlaceholder}" /></label><label>${DNS_PHASE2_TEXT.type}<select id="dns-type-filter">${types.map((type) => `<option value="${esc(type)}" ${STATE.dnsTypeFilter === type ? 'selected' : ''}>${type === 'all' ? DNS_PHASE2_TEXT.allTypes : esc(type)}</option>`).join('')}</select></label><span class="muted">Zone: ${esc(zone || DNS_PHASE2_TEXT.dash)} ${DNS_PHASE2_TEXT.dot} ${DNS_PHASE2_TEXT.tokenSafe}</span></div>${groupHTML || emptyState(DNS_PHASE2_TEXT.noRecords, DNS_PHASE2_TEXT.noRecordsHint)}`;
  }
  return pageCard('dns', `${viewHeader('dns', actions)}<div class="panel"><div class="panel-head"><div><h3>${DNS_PHASE2_TEXT.focusTitle}</h3><span class="muted">${DNS_PHASE2_TEXT.focusHint}</span></div>${configured ? statusBadge(STATE.dnsLoadError ? 'error' : 'connected') : statusBadge('not configured')}</div>${body}</div>${renderDNSRecordForm()}${renderDNSDeleteDialog()}`);
};

bindDNSUI = function bindDNSUIPhase2() {
  if (STATE.activePage === 'dns' && !STATE.dnsLoaded && !STATE.dnsLoading) loadDNSRecords();
  document.querySelectorAll('.dns-accordion').forEach((details) => details.addEventListener('toggle', () => {
    if (details.open) {
      STATE.dnsOpenGroup = details.dataset.dnsGroupKey || '';
      document.querySelectorAll('.dns-accordion[open]').forEach((other) => { if (other !== details) other.open = false; });
    } else if (STATE.dnsOpenGroup === details.dataset.dnsGroupKey) STATE.dnsOpenGroup = '';
  }));
  document.querySelectorAll('[data-dns-action]').forEach((button) => button.addEventListener('click', () => {
    const action = button.dataset.dnsAction;
    const record = currentDNSRecord(button.dataset.dnsId);
    if (action === 'refresh') loadDNSRecords(true);
    if (action === 'new') { STATE.dnsEditor = dnsEditorRecord(null); render(); }
    if (action === 'from-tunnel') { STATE.dnsEditor = dnsEditorRecord(null, { fromTunnel: true }); render(); }
    if (action === 'edit' && record) { STATE.dnsEditor = dnsEditorRecord(record); render(); }
    if (action === 'delete' && record) { STATE.dnsDeleteRecord = record; STATE.dnsDeleteName = ''; render(); }
    if (action === 'close-editor') { STATE.dnsEditor = null; render(); }
    if (action === 'close-delete') { STATE.dnsDeleteRecord = null; STATE.dnsDeleteName = ''; render(); }
    if (action === 'open-settings') setDNSPage('settings');
  }));
  $('dns-filter')?.addEventListener('input', (event) => { STATE.dnsFilter = event.target.value; render(); });
  $('dns-type-filter')?.addEventListener('change', (event) => { STATE.dnsTypeFilter = event.target.value; render(); });
  $('dns-tunnel-source')?.addEventListener('change', (event) => { syncDNSCNAMEFromTunnel(event.target.value); render(); });
  $('dns-record-type')?.addEventListener('change', (event) => { STATE.dnsEditor.type = event.target.value; render(); });
  const form = $('dns-record-form');
  if (form) form.addEventListener('submit', (event) => { event.preventDefault(); submitDNSRecord(); });
  $('dns-delete-name')?.addEventListener('input', (event) => {
    STATE.dnsDeleteName = event.target.value;
    const submit = $('dns-delete-form button[type="submit"]');
    if (submit) submit.disabled = STATE.dnsDeleteName !== STATE.dnsDeleteRecord?.name;
  });
  const deleteForm = $('dns-delete-form');
  if (deleteForm) deleteForm.addEventListener('submit', (event) => { event.preventDefault(); submitDNSDelete(); });
};

// Phase 3 Settings Center
const SETTINGS_PHASE3_TEXT = {
  overview: '\u8bbe\u7f6e\u603b\u89c8',
  overviewHint: '\u96c6\u4e2d\u7ba1\u7406\u96c6\u6210\u51ed\u636e\u3001FRPC \u8fd0\u884c\u65f6\u3001\u901a\u7528\u7b56\u7565\u4e0e\u8d26\u6237\u5b89\u5168\u3002',
  refresh: '\u5237\u65b0\u8bbe\u7f6e',
  configured: '\u5df2\u914d\u7f6e',
  notConfigured: '\u672a\u914d\u7f6e',
  saved: '\u51ed\u636e\u5df2\u4fdd\u5b58',
  unsaved: '\u5c1a\u672a\u4fdd\u5b58\u51ed\u636e',
  tokenMask: 'Token \u63a9\u7801',
  credentialRef: '\u51ed\u636e\u6307\u7eb9',
  credentialRevision: '\u51ed\u636e\u7248\u672c',
  updatedAt: '\u66f4\u65b0\u65f6\u95f4',
  verifiedAt: '\u9a8c\u8bc1\u65f6\u95f4',
  lastError: '\u6700\u8fd1\u9519\u8bef',
  audit: '\u5ba1\u8ba1\u65e5\u5fd7',
  saveVerify: '\u4fdd\u5b58\u5e76\u9a8c\u8bc1',
  verifySaved: '\u9a8c\u8bc1\u5df2\u4fdd\u5b58 Token',
  saving: '\u4fdd\u5b58\u4e2d\u2026',
  verifying: '\u9a8c\u8bc1\u4e2d\u2026',
  zone: 'Zone \u540d\u79f0\u6216 Zone ID',
  apiToken: 'API Token',
  keepToken: 'API Token 已安全保存。若需修改请在此粘贴',
  pasteToken: '\u7c98\u8d34\u65b0\u51ed\u636e',
  username: '\u7528\u6237\u540d',
  password: '\u5bc6\u7801',
  baseURL: 'Base URL',
  entrance: '\u5b89\u5168\u5165\u53e3',
  syncNodes: '\u540c\u6b65\u8282\u70b9',
  cloudflareHint: '为保障安全，刷新页面后输入框不会回显 API Token；页面仅显示掩码与凭据状态。',
  chmlfrpHint: '\u4fdd\u5b58\u540e\u4f1a\u89e6\u53d1 chmlfrp \u8fde\u901a\u6027\u6821\u9a8c\uff0c\u5bc6\u7801\u4e0d\u4f1a\u56de\u663e\u3002',
  onepanelHint: '\u4fdd\u5b58\u540e\u4f1a\u6d4b\u8bd5 1Panel API \u8fde\u901a\u6027\uff0cToken \u4e0d\u4f1a\u56de\u663e\u3002',
  runtime: 'FRPC Runtime \u7b56\u7565',
  runtimeHint: '\u63a7\u5236\u672c\u5730 FRPC \u542f\u7528\u3001\u65e5\u5fd7\u7ea7\u522b\u3001\u5065\u5eb7\u68c0\u67e5\u4e0e\u81ea\u52a8\u6062\u590d\u7b56\u7565\u3002',
  enableFRPC: '\u542f\u7528 FRPC \u63a7\u5236',
  logLevel: '\u65e5\u5fd7\u7ea7\u522b',
  healthInterval: '\u5065\u5eb7\u68c0\u67e5\u95f4\u9694',
  restartBackoff: '\u91cd\u542f\u9000\u907f',
  autoRecover: '\u6545\u969c\u6062\u590d',
  switchNode: '\u8282\u70b9\u5207\u6362',
  binarySource: '\u4e8c\u8fdb\u5236\u6765\u6e90',
  binaryVersion: '\u4e8c\u8fdb\u5236\u7248\u672c',
  policies: '\u901a\u7528\u4e0e\u961f\u5217\u7b56\u7565',
  policiesHint: '\u914d\u7f6e\u65e5\u5fd7\u4fdd\u7559\u3001\u5237\u65b0\u9891\u7387\u3001Job \u91cd\u8bd5\u4e0e\u5f52\u6863\u7b56\u7565\u3002',
  defaultLogLines: '\u9ed8\u8ba4\u65e5\u5fd7\u884c\u6570',
  dataRetention: '\u6570\u636e\u4fdd\u7559\u5929\u6570',
  refreshMode: '\u9ed8\u8ba4\u5237\u65b0\u6a21\u5f0f',
  syncPoll: '\u540c\u6b65\u8f6e\u8be2\u95f4\u9694',
  maxAttempts: 'Job \u6700\u5927\u91cd\u8bd5',
  retryBackoff: 'Job \u9000\u907f',
  archiveRetention: '\u5f52\u6863\u4fdd\u7559\u5929\u6570',
  stalledPolicy: '\u5361\u6b7b\u4efb\u52a1\u7b56\u7565',
  saveSettings: '\u4fdd\u5b58\u7b56\u7565',
  account: '\u8d26\u6237\u5b89\u5168\u4e0e\u5371\u9669\u64cd\u4f5c',
  accountHint: '\u5355\u7ba1\u7406\u5458\u6a21\u578b\u4e0b\uff0c\u6539\u5bc6\u3001Token \u540a\u9500\u548c\u7ec8\u7aef\u6062\u590d\u90fd\u5728\u6b64\u96c6\u4e2d\u5c55\u793a\u3002',
  oldPassword: '\u5f53\u524d\u5bc6\u7801',
  newPassword: '\u65b0\u5bc6\u7801',
  confirmPassword: '\u786e\u8ba4\u65b0\u5bc6\u7801',
  changePassword: '\u4fee\u6539\u5bc6\u7801',
  recovery: '\u7ec8\u7aef\u6062\u590d\u547d\u4ee4',
  recoveryHint: '\u4e0d\u63d0\u4f9b\u516c\u5f00\u5fd8\u8bb0\u5bc6\u7801 API\uff1b\u5fd8\u8bb0\u5bc6\u7801\u65f6\u53ea\u80fd\u5728\u670d\u52a1\u5668\u7ec8\u7aef\u6267\u884c\u6062\u590d\u3002',
  showRecovery: '\u67e5\u770b\u6062\u590d\u547d\u4ee4',
  sessions: '\u4f1a\u8bdd\u4e0e API Token',
  revokeHint: '\u540a\u9500\u540e\u5bf9\u5e94 Session \u6216 API Token \u7acb\u5373\u5931\u6548\u3002',
  tokenID: '\u4ee4\u724c ID',
  tokenType: '\u7c7b\u578b',
  status: '\u72b6\u6001',
  lastUsed: '\u6700\u8fd1\u4f7f\u7528',
  expiresAt: '\u8fc7\u671f\u65f6\u95f4',
  operation: '\u64cd\u4f5c',
  revoke: '\u540a\u9500',
  noTokens: '\u6682\u65e0\u6d3b\u8dc3\u4f1a\u8bdd',
  noTokensHint: '\u6210\u529f\u767b\u5f55\u540e\u4f1a\u81ea\u52a8\u521b\u5efa\u4f1a\u8bdd\u8bb0\u5f55\u3002',
  requiredCloudflareZone: '\u8bf7\u586b\u5199 Cloudflare Zone \u540d\u79f0\u6216 Zone ID\u3002',
  requiredCloudflareToken: '\u8bf7\u5148\u8f93\u5165 Cloudflare API Token\u3002',
  requiredChmlfrp: '\u8bf7\u586b\u5199 chmlfrp \u7528\u6237\u540d\u4e0e\u5bc6\u7801\u3002',
  requiredOnepanel: '\u8bf7\u586b\u5199 1Panel Base URL \u4e0e API Token\u3002',
  passwordMismatch: '\u65b0\u5bc6\u7801\u4e24\u6b21\u8f93\u5165\u4e0d\u4e00\u81f4\u3002',
  passwordShort: '\u65b0\u5bc6\u7801\u81f3\u5c11 8 \u4e2a\u5b57\u7b26\u3002',
  savedNotice: '\u8bbe\u7f6e\u5df2\u4fdd\u5b58\u3002',
  saveFailed: '\u4fdd\u5b58\u5931\u8d25',
  chmlfrpSaved: 'chmlfrp \u51ed\u636e\u5df2\u4fdd\u5b58\u5e76\u5c06\u8fdb\u884c\u9a8c\u8bc1\u3002',
  onepanelSaved: '1Panel \u51ed\u636e\u5df2\u4fdd\u5b58\u5e76\u5c06\u8fdb\u884c\u9a8c\u8bc1\u3002',
  runtimeSaved: 'FRPC Runtime \u7b56\u7565\u5df2\u4fdd\u5b58\u3002',
  policySaved: '\u901a\u7528\u4e0e\u961f\u5217\u7b56\u7565\u5df2\u4fdd\u5b58\u3002',
  passwordChanged: '\u5bc6\u7801\u5df2\u4fee\u6539\uff0c\u8bf7\u5982\u6709\u9700\u8981\u91cd\u65b0\u767b\u5f55\u5176\u4ed6\u8bbe\u5907\u3002',
  dash: '\u2014'
};

function settingsCurrent() { return STATE.settings || {}; }
function settingsSafeIntegration(key) { const item = { ...integrationState(key) }; delete item.api_token; delete item.password; return item; }
function settingsBusyAttr() { return STATE.actionBusy ? 'disabled' : ''; }
function settingsFieldValue(value, fallback = '') { return esc(value ?? fallback); }
function settingsNumberValue(value, fallback) { const number = Number(value); return Number.isFinite(number) && number > 0 ? number : fallback; }
function settingsConfigured(item, secretKey) { return Boolean(item.configured || item[secretKey] || item.has_password || item.has_api_token); }
function settingsSecretLabel(item, secretKey) { return settingsConfigured(item, secretKey) ? SETTINGS_PHASE3_TEXT.saved : SETTINGS_PHASE3_TEXT.unsaved; }
function settingsMetaRows(rows) {
  return `<div class="settings-meta-list">${rows.map(([label, value, extraClass = '']) => `<span><small>${label}</small><strong class="${extraClass}">${value}</strong></span>`).join('')}</div>`;
}
function settingsPatchPayload(overrides = {}) {
  const current = settingsCurrent();
  const currentIntegrations = {
    chmlfrp: settingsSafeIntegration('chmlfrp'),
    onepanel: settingsSafeIntegration('onepanel'),
    cloudflare: settingsSafeIntegration('cloudflare'),
  };
  return {
    general: { ...(current.general || {}), ...(overrides.general || {}) },
    sync: { ...(current.sync || {}), ...(overrides.sync || {}) },
    queue: { ...(current.queue || {}), ...(overrides.queue || {}) },
    frpc_runtime: { ...(current.frpc_runtime || {}), ...(overrides.frpc_runtime || {}) },
    integrations: {
      ...currentIntegrations,
      ...(overrides.integrations || {}),
      chmlfrp: { ...currentIntegrations.chmlfrp, ...(overrides.integrations?.chmlfrp || {}) },
      onepanel: { ...currentIntegrations.onepanel, ...(overrides.integrations?.onepanel || {}) },
      cloudflare: { ...currentIntegrations.cloudflare, ...(overrides.integrations?.cloudflare || {}) },
    },
  };
}
async function saveSettingsPatch(overrides, busy, successMessage) {
  STATE.actionBusy = busy; STATE.error = ''; STATE.notice = ''; render();
  try {
    await request('/settings', { method: 'PATCH', body: JSON.stringify(settingsPatchPayload(overrides)) });
    await loadSnapshot();
    STATE.notice = successMessage || SETTINGS_PHASE3_TEXT.savedNotice;
  } catch (err) { STATE.error = `${SETTINGS_PHASE3_TEXT.saveFailed}${SETTINGS_PHASE3_TEXT.dash}${apiError(err)}`; }
  finally { STATE.actionBusy = ''; render(); }
}
function settingsSelect(id, value, options) { return `<select id="${id}">${options.map(([optionValue, label]) => `<option value="${esc(optionValue)}" ${String(value || '') === String(optionValue) ? 'selected' : ''}>${esc(label)}</option>`).join('')}</select>`; }
function settingsCredentialCard(kind, title, subtitle, status, formHTML, metaHTML, extraHTML = '') {
  return `<section class="panel settings-card settings-card-${kind}"><div class="panel-head"><div><div class="eyebrow">${kind.toUpperCase()}</div><h3>${title}</h3><span class="muted">${subtitle}</span></div>${status}</div>${metaHTML}${formHTML}${extraHTML}</section>`;
}
function renderSettingsCloudflareCard() {
  const cloudflare = integrationState('cloudflare');
  const configured = settingsConfigured(cloudflare, 'has_api_token');
  const zone = STATE.tempCfZone !== undefined ? STATE.tempCfZone : (cloudflare.zone_name || cloudflare.identifier || '');
  const savedToken = cloudflare.api_token || '';
  const tempToken = STATE.tempCfToken !== undefined ? STATE.tempCfToken : savedToken;
  const meta = settingsMetaRows([
    [SETTINGS_PHASE3_TEXT.tokenMask, esc(cloudflare.token_mask || settingsSecretLabel(cloudflare, 'has_api_token')), 'credential-pill'],
    [SETTINGS_PHASE3_TEXT.credentialRef, `<code>${esc(cloudflare.credential_ref || SETTINGS_PHASE3_TEXT.dash)}</code>`],
    [SETTINGS_PHASE3_TEXT.credentialRevision, esc(cloudflare.credential_revision || 0)],
    [SETTINGS_PHASE3_TEXT.verifiedAt, esc(fmtTime(cloudflare.last_validated_at || cloudflare.last_verified_at))],
  ]);
  const zoneDisplay = zone ? `已绑定 Zone: <strong>${esc(zone)}</strong>` : '<span class="muted">自动探测 Zone (或在弹窗中选择)</span>';
  const form = `<form id="cloudflare-settings-form" class="integration-form settings-form-grid"><label>${SETTINGS_PHASE3_TEXT.apiToken}<input id="cloudflare-api-token" name="cloudflare-api-token" type="text" value="${esc(tempToken)}" autocomplete="off" placeholder="${configured ? SETTINGS_PHASE3_TEXT.keepToken : SETTINGS_PHASE3_TEXT.pasteToken}" /></label><div style="margin-bottom:16px; font-size:14px;">${zoneDisplay}</div><p class="muted integration-help">${SETTINGS_PHASE3_TEXT.cloudflareHint}</p><div class="settings-actions"><button type="submit" ${settingsBusyAttr()}>${STATE.actionBusy === 'cloudflare-save' ? SETTINGS_PHASE3_TEXT.saving : SETTINGS_PHASE3_TEXT.saveVerify}</button><button type="button" class="secondary" id="cloudflare-verify-btn" ${settingsBusyAttr()}>${STATE.actionBusy === 'cloudflare-verify' ? SETTINGS_PHASE3_TEXT.verifying : SETTINGS_PHASE3_TEXT.verifySaved}</button><button type="button" class="ghost" id="cloudflare-audit-btn">${SETTINGS_PHASE3_TEXT.audit}</button></div></form>${renderCfZoneModal()}`;
  const error = cloudflare.last_error_message || cloudflare.last_error;
  return settingsCredentialCard('cloudflare', 'Cloudflare DNS', SETTINGS_PHASE3_TEXT.cloudflareHint, statusBadge(configured ? 'configured' : 'not configured'), form, meta, error ? `<div class="settings-inline-error">${esc(error)}</div>` : '');
}

function renderCfZoneModal() {
  if (!STATE.cfZoneModalOpen) return '';
  const options = (STATE.cfZones || []).map(z => `<option value="${esc(z.name)}">${esc(z.name)}</option>`).join('');
  return `<div class="dns-modal-backdrop"><section class="dns-modal" role="dialog" aria-modal="true"><div class="panel-head"><div><div class="eyebrow">CLOUDFLARE DNS</div><h3>选择要绑定的 Zone</h3></div><button type="button" class="icon-button" onclick="STATE.cfZoneModalOpen=false; STATE.actionBusy=''; render();">×</button></div><p style="margin-bottom:16px;">该 API Token 关联了多个 Zone，请选择一个进行绑定：</p><form class="integration-form" onsubmit="event.preventDefault(); STATE.cfZoneModalOpen=false; STATE.tempCfZone=document.getElementById('cf-modal-zone-select').value; saveCloudflareSettings();"><label><select id="cf-modal-zone-select">${options}</select></label><div class="settings-actions"><button type="submit" ${STATE.cfZones?.length ? '' : 'disabled'}>确认绑定</button><button type="button" class="secondary" onclick="STATE.cfZoneModalOpen=false; STATE.actionBusy=''; render();">取消</button></div></form></section></div>`;
}
function renderSettingsChmlFrpCard() {
  const item = integrationState('chmlfrp');
  const configured = settingsConfigured(item, 'has_password');
  const meta = settingsMetaRows([
    [SETTINGS_PHASE3_TEXT.username, esc(item.username || SETTINGS_PHASE3_TEXT.dash)],
    [SETTINGS_PHASE3_TEXT.password, esc(settingsSecretLabel(item, 'has_password')), 'credential-pill'],
    [SETTINGS_PHASE3_TEXT.verifiedAt, esc(fmtTime(item.last_validated_at || item.last_verified_at))],
  ]);
  const form = `<form id="chmlfrp-settings-form" class="integration-form settings-form-grid"><label>${SETTINGS_PHASE3_TEXT.username}<input id="chmlfrp-username" value="${esc(item.username || '')}" autocomplete="username" placeholder="chmlfrp" required /></label><label>${SETTINGS_PHASE3_TEXT.password}<input id="chmlfrp-password" type="password" autocomplete="new-password" placeholder="${configured ? SETTINGS_PHASE3_TEXT.keepToken : SETTINGS_PHASE3_TEXT.pasteToken}" /></label><p class="muted integration-help">${SETTINGS_PHASE3_TEXT.chmlfrpHint}</p><div class="settings-actions"><button type="submit" ${settingsBusyAttr()}>${STATE.actionBusy === 'chmlfrp-save' ? SETTINGS_PHASE3_TEXT.saving : SETTINGS_PHASE3_TEXT.saveVerify}</button><button type="button" class="secondary" data-action="nodes-sync">${SETTINGS_PHASE3_TEXT.syncNodes}</button></div></form>`;
  const error = item.last_error_message || item.last_error;
  return settingsCredentialCard('chmlfrp', 'ChmlFrp', SETTINGS_PHASE3_TEXT.chmlfrpHint, statusBadge(configured ? 'configured' : 'not configured'), form, meta, error ? `<div class="settings-inline-error">${esc(error)}</div>` : '');
}
function renderSettingsOnePanelCard() {
  const item = integrationState('onepanel');
  const configured = settingsConfigured(item, 'has_api_token');
  const meta = settingsMetaRows([
    [SETTINGS_PHASE3_TEXT.baseURL, esc(item.base_url || SETTINGS_PHASE3_TEXT.dash)],
    [SETTINGS_PHASE3_TEXT.apiToken, esc(settingsSecretLabel(item, 'has_api_token')), 'credential-pill'],
    [SETTINGS_PHASE3_TEXT.verifiedAt, esc(fmtTime(item.last_validated_at || item.last_verified_at))],
  ]);
  const form = `<form id="onepanel-settings-form" class="integration-form settings-form-grid"><label>${SETTINGS_PHASE3_TEXT.baseURL}<input id="onepanel-base-url" value="${esc(item.base_url || '')}" autocomplete="off" placeholder="https://panel.example.com" required /></label><label>${SETTINGS_PHASE3_TEXT.apiToken}<input id="onepanel-api-token" type="password" autocomplete="off" placeholder="${configured ? SETTINGS_PHASE3_TEXT.keepToken : SETTINGS_PHASE3_TEXT.pasteToken}" /></label><label>${SETTINGS_PHASE3_TEXT.entrance}<input id="onepanel-entrance" value="${esc(item.entrance || '')}" autocomplete="off" placeholder="/entrance" /></label><p class="muted integration-help">${SETTINGS_PHASE3_TEXT.onepanelHint}</p><div class="settings-actions"><button type="submit" ${settingsBusyAttr()}>${STATE.actionBusy === 'onepanel-save' ? SETTINGS_PHASE3_TEXT.saving : SETTINGS_PHASE3_TEXT.saveVerify}</button></div></form>`;
  const error = item.last_error_message || item.last_error;
  return settingsCredentialCard('onepanel', '1Panel', SETTINGS_PHASE3_TEXT.onepanelHint, statusBadge(configured ? 'configured' : 'not configured'), form, meta, error ? `<div class="settings-inline-error">${esc(error)}</div>` : '');
}
function renderSettingsRuntimeCard() {
  const runtime = settingsCurrent().frpc_runtime || {};
  return `<section class="panel settings-card settings-card-runtime"><div class="panel-head"><div><div class="eyebrow">FRPC RUNTIME</div><h3>${SETTINGS_PHASE3_TEXT.runtime}</h3><span class="muted">${SETTINGS_PHASE3_TEXT.runtimeHint}</span></div>${statusBadge(runtime.frpc_enabled ? 'enabled' : 'disabled')}</div><form id="settings-runtime-form" class="integration-form settings-form-grid"><label class="settings-switch"><input id="settings-frpc-enabled" type="checkbox" ${runtime.frpc_enabled ? 'checked' : ''} /><span></span><b>${SETTINGS_PHASE3_TEXT.enableFRPC}</b></label><label>${SETTINGS_PHASE3_TEXT.logLevel}${settingsSelect('settings-frpc-log-level', runtime.frpc_log_level || 'info', [['info','info'],['debug','debug'],['warn','warn'],['error','error']])}</label><label>${SETTINGS_PHASE3_TEXT.healthInterval}<input id="settings-frpc-health" value="${esc(runtime.frpc_healthcheck_interval || '30s')}" /></label><label>${SETTINGS_PHASE3_TEXT.restartBackoff}<input id="settings-frpc-backoff" value="${esc(runtime.frpc_restart_backoff || '30s')}" /></label><label>${SETTINGS_PHASE3_TEXT.autoRecover}${settingsSelect('settings-frpc-recover', runtime.auto_recover_strategy || 'reload_then_restart', [['reload_then_restart','reload_then_restart'],['restart_only','restart_only'],['disabled','disabled']])}</label><label>${SETTINGS_PHASE3_TEXT.switchNode}${settingsSelect('settings-frpc-switch', runtime.switch_node_strategy || 'prefer_healthy_low_load', [['prefer_healthy_low_load','prefer_healthy_low_load'],['manual_only','manual_only'],['disabled','disabled']])}</label><label>${SETTINGS_PHASE3_TEXT.binarySource}<input id="settings-frpc-source" value="${esc(runtime.frpc_binary_source || 'embedded')}" /></label><label>${SETTINGS_PHASE3_TEXT.binaryVersion}<input id="settings-frpc-version" value="${esc(runtime.frpc_binary_version || '0.54.0')}" /></label><div class="settings-actions"><button type="submit" ${settingsBusyAttr()}>${STATE.actionBusy === 'runtime-save' ? SETTINGS_PHASE3_TEXT.saving : SETTINGS_PHASE3_TEXT.saveSettings}</button></div></form></section>`;
}
function renderSettingsPolicyCard() {
  const general = settingsCurrent().general || {};
  const sync = settingsCurrent().sync || {};
  const queue = settingsCurrent().queue || {};
  return `<section class="panel settings-card settings-card-policy"><div class="panel-head"><div><div class="eyebrow">POLICY</div><h3>${SETTINGS_PHASE3_TEXT.policies}</h3><span class="muted">${SETTINGS_PHASE3_TEXT.policiesHint}</span></div></div><form id="settings-policy-form" class="integration-form settings-form-grid"><label>${SETTINGS_PHASE3_TEXT.defaultLogLines}<input id="settings-default-log-lines" type="number" min="10" value="${esc(settingsNumberValue(general.default_log_lines, 100))}" /></label><label>${SETTINGS_PHASE3_TEXT.dataRetention}<input id="settings-data-retention" type="number" min="1" value="${esc(settingsNumberValue(general.data_retention_days, 30))}" /></label><label>${SETTINGS_PHASE3_TEXT.refreshMode}${settingsSelect('settings-refresh-mode', general.default_refresh_mode || 'polling', [['polling','polling'],['sse','sse'],['manual','manual']])}</label><label>${SETTINGS_PHASE3_TEXT.syncPoll}<input id="settings-sync-poll" value="${esc(sync.sync_poll_interval || '10s')}" /></label><label>${SETTINGS_PHASE3_TEXT.healthInterval}<input id="settings-sync-health" value="${esc(sync.healthcheck_interval || '1m')}" /></label><label>${SETTINGS_PHASE3_TEXT.maxAttempts}<input id="settings-max-attempts" type="number" min="1" value="${esc(settingsNumberValue(queue.max_attempts, 5))}" /></label><label>${SETTINGS_PHASE3_TEXT.retryBackoff}<input id="settings-retry-backoff" value="${esc(queue.retry_backoff || '30s')}" /></label><label>${SETTINGS_PHASE3_TEXT.archiveRetention}<input id="settings-archive-retention" type="number" min="1" value="${esc(settingsNumberValue(queue.archive_retention_days, 30))}" /></label><label>${SETTINGS_PHASE3_TEXT.stalledPolicy}${settingsSelect('settings-stalled-policy', queue.stalled_job_policy || 'mark_blocked', [['mark_blocked','mark_blocked'],['requeue','requeue'],['cancel','cancel']])}</label><div class="settings-actions"><button type="submit" ${settingsBusyAttr()}>${STATE.actionBusy === 'policy-save' ? SETTINGS_PHASE3_TEXT.saving : SETTINGS_PHASE3_TEXT.saveSettings}</button></div></form></section>`;
}
function renderSettingsAccountCard() {
  const tokenRows = safeArray(STATE.authTokens).map((token) => `<tr><td class="mono">${esc(shortID(token.id))}</td><td>${esc(token.token_type || 'session')}</td><td>${token.revoked_at ? statusBadge('revoked') : statusBadge('active')}</td><td>${esc(fmtTime(token.last_used_at))}</td><td>${esc(fmtTime(token.expires_at))}</td><td>${token.revoked_at ? SETTINGS_PHASE3_TEXT.dash : actionButton(SETTINGS_PHASE3_TEXT.revoke,'token-revoke',{id:token.id,ghost:true})}</td></tr>`);
  return `<section class="panel settings-card settings-card-account"><div class="panel-head"><div><div class="eyebrow">SECURITY</div><h3>${SETTINGS_PHASE3_TEXT.account}</h3><span class="muted">${SETTINGS_PHASE3_TEXT.accountHint}</span></div></div><form id="settings-password-form" class="integration-form settings-form-grid"><label>${SETTINGS_PHASE3_TEXT.oldPassword}<input id="settings-old-password" type="password" autocomplete="current-password" required /></label><label>${SETTINGS_PHASE3_TEXT.newPassword}<input id="settings-new-password" type="password" autocomplete="new-password" required /></label><label>${SETTINGS_PHASE3_TEXT.confirmPassword}<input id="settings-confirm-password" type="password" autocomplete="new-password" required /></label><div class="settings-actions"><button type="submit" ${settingsBusyAttr()}>${STATE.actionBusy === 'password-change' ? SETTINGS_PHASE3_TEXT.saving : SETTINGS_PHASE3_TEXT.changePassword}</button></div></form><div class="danger-zone"><div><strong>${SETTINGS_PHASE3_TEXT.recovery}</strong><p>${SETTINGS_PHASE3_TEXT.recoveryHint}</p></div><button type="button" class="secondary" id="forgot-password-btn">${SETTINGS_PHASE3_TEXT.showRecovery}</button></div><div class="settings-token-table"><div class="panel-head"><h3>${SETTINGS_PHASE3_TEXT.sessions}</h3><span class="muted">${SETTINGS_PHASE3_TEXT.revokeHint}</span></div>${renderTable([SETTINGS_PHASE3_TEXT.tokenID,SETTINGS_PHASE3_TEXT.tokenType,SETTINGS_PHASE3_TEXT.status,SETTINGS_PHASE3_TEXT.lastUsed,SETTINGS_PHASE3_TEXT.expiresAt,SETTINGS_PHASE3_TEXT.operation], tokenRows, SETTINGS_PHASE3_TEXT.noTokens, SETTINGS_PHASE3_TEXT.noTokensHint)}</div></section>`;
}

renderSettings = function renderSettingsPhase3() {
  const configuredCount = ['cloudflare','chmlfrp','onepanel'].filter((key) => settingsConfigured(integrationState(key), key === 'chmlfrp' ? 'has_password' : 'has_api_token')).length;
  const runtime = settingsCurrent().frpc_runtime || {};
  return pageCard('settings', `${viewHeader('settings', actionButton(SETTINGS_PHASE3_TEXT.refresh,'reload',{secondary:true}))}<div class="settings-overview panel"><div><h3>${SETTINGS_PHASE3_TEXT.overview}</h3><span class="muted">${SETTINGS_PHASE3_TEXT.overviewHint}</span></div><div class="metric-grid compact">${metric(SETTINGS_PHASE3_TEXT.configured, `${configuredCount}/3`)}${metric('FRPC', runtime.frpc_enabled ? 'enabled' : 'disabled')}${metric(SETTINGS_PHASE3_TEXT.sessions, safeArray(STATE.authTokens).filter((token) => !token.revoked_at).length)}${metric(SETTINGS_PHASE3_TEXT.credentialRevision, integrationState('cloudflare').credential_revision || 0)}</div></div><div class="settings-card-grid integrations-grid">${renderSettingsCloudflareCard()}${renderSettingsChmlFrpCard()}${renderSettingsOnePanelCard()}</div><div class="settings-card-grid strategy-grid">${renderSettingsRuntimeCard()}${renderSettingsPolicyCard()}</div>${renderSettingsAccountCard()}`);
};

async function saveChmlFrpSettings() {
  const username = $('chmlfrp-username')?.value.trim() || '';
  const password = $('chmlfrp-password')?.value || '';
  const current = integrationState('chmlfrp');
  if (!username || (!password && !settingsConfigured(current, 'has_password'))) { STATE.error = SETTINGS_PHASE3_TEXT.requiredChmlfrp; render(); return; }
  await saveSettingsPatch({ integrations: { chmlfrp: { ...settingsSafeIntegration('chmlfrp'), username, password } } }, 'chmlfrp-save', SETTINGS_PHASE3_TEXT.chmlfrpSaved);
}
async function saveOnePanelSettings() {
  const baseURL = $('onepanel-base-url')?.value.trim() || '';
  const apiToken = $('onepanel-api-token')?.value || '';
  const entrance = $('onepanel-entrance')?.value.trim() || '';
  const current = integrationState('onepanel');
  if (!baseURL || (!apiToken && !settingsConfigured(current, 'has_api_token'))) { STATE.error = SETTINGS_PHASE3_TEXT.requiredOnepanel; render(); return; }
  await saveSettingsPatch({ integrations: { onepanel: { ...settingsSafeIntegration('onepanel'), base_url: baseURL, entrance, api_token: apiToken } } }, 'onepanel-save', SETTINGS_PHASE3_TEXT.onepanelSaved);
}
async function saveRuntimeSettings() {
  await saveSettingsPatch({ frpc_runtime: { frpc_enabled: Boolean($('settings-frpc-enabled')?.checked), frpc_log_level: $('settings-frpc-log-level')?.value || 'info', frpc_healthcheck_interval: $('settings-frpc-health')?.value.trim() || '30s', frpc_restart_backoff: $('settings-frpc-backoff')?.value.trim() || '30s', auto_recover_strategy: $('settings-frpc-recover')?.value || 'reload_then_restart', switch_node_strategy: $('settings-frpc-switch')?.value || 'prefer_healthy_low_load', frpc_binary_source: $('settings-frpc-source')?.value.trim() || 'embedded', frpc_binary_version: $('settings-frpc-version')?.value.trim() || '0.54.0' } }, 'runtime-save', SETTINGS_PHASE3_TEXT.runtimeSaved);
}
async function savePolicySettings() {
  await saveSettingsPatch({
    general: { default_log_lines: Number($('settings-default-log-lines')?.value || 100), data_retention_days: Number($('settings-data-retention')?.value || 30), default_refresh_mode: $('settings-refresh-mode')?.value || 'polling' },
    sync: { healthcheck_interval: $('settings-sync-health')?.value.trim() || '1m', sync_poll_interval: $('settings-sync-poll')?.value.trim() || '10s', diff_strategy: settingsCurrent().sync?.diff_strategy || 'pause_on_conflict', manual_override_priority: settingsCurrent().sync?.manual_override_priority || 'manual_wins' },
    queue: { max_attempts: Number($('settings-max-attempts')?.value || 5), retry_backoff: $('settings-retry-backoff')?.value.trim() || '30s', stalled_job_policy: $('settings-stalled-policy')?.value || 'mark_blocked', archive_retention_days: Number($('settings-archive-retention')?.value || 30) },
  }, 'policy-save', SETTINGS_PHASE3_TEXT.policySaved);
}
async function changeSettingsPassword() {
  const oldPassword = $('settings-old-password')?.value || '';
  const newPassword = $('settings-new-password')?.value || '';
  const confirmPassword = $('settings-confirm-password')?.value || '';
  if (newPassword !== confirmPassword) { STATE.error = SETTINGS_PHASE3_TEXT.passwordMismatch; render(); return; }
  if (newPassword.length < 8) { STATE.error = SETTINGS_PHASE3_TEXT.passwordShort; render(); return; }
  STATE.actionBusy = 'password-change'; STATE.error = ''; STATE.notice = ''; render();
  try {
    await request('/auth/password/change', { method: 'POST', body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }) });
    STATE.notice = SETTINGS_PHASE3_TEXT.passwordChanged;
  } catch (err) { STATE.error = apiError(err); }
  finally { STATE.actionBusy = ''; render(); }
}
async function loadCloudflareZones() {
  const token = $('cloudflare-api-token')?.value || '';
  STATE.tempCfToken = token;
  STATE.tempCfZone = $('cloudflare-zone')?.value || '';
  STATE.actionBusy = 'cloudflare-load-zones'; STATE.error = ''; STATE.notice = ''; render();
  try {
    const res = await request('/settings/integrations/cloudflare/zones', { method: 'POST', body: JSON.stringify({ token }) });
    STATE.cfZones = res?.data?.zones || [];
    STATE.notice = 'Zones 列表已加载，可在下拉框中选择。';
  } catch (err) { STATE.error = `加载 Zones 失败：${apiError(err)}`; }
  finally { STATE.actionBusy = ''; render(); }
}

function bindSettingsCenterUI() {
  $('chmlfrp-settings-form')?.addEventListener('submit', (event) => { event.preventDefault(); saveChmlFrpSettings(); });
  $('onepanel-settings-form')?.addEventListener('submit', (event) => { event.preventDefault(); saveOnePanelSettings(); });
  $('settings-runtime-form')?.addEventListener('submit', (event) => { event.preventDefault(); saveRuntimeSettings(); });
  $('settings-policy-form')?.addEventListener('submit', (event) => { event.preventDefault(); savePolicySettings(); });
  $('settings-password-form')?.addEventListener('submit', (event) => { event.preventDefault(); changeSettingsPassword(); });
  $('cloudflare-load-zones-btn')?.addEventListener('click', loadCloudflareZones);
}
const renderWithSettingsCenterBase = render;
render = function renderWithSettingsCenter() { renderWithSettingsCenterBase(); bindSettingsCenterUI(); };
