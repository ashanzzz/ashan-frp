const APP_ROOT_ID = 'app';
const API_PREFIX = '/api/v1';
const STATE = {
  apiBase: API_PREFIX, uiBase: '/ui', version: null, health: null, dashboard: null, settings: null,
  nodes: [], tunnels: [], websites: [], jobs: [], auditLogs: [], authTokens: [], frpcRuntime: null,
  selectedJob: null, selectedJobEvents: [], authMe: null, activePage: 'dashboard', error: '', notice: '',
  loading: false, actionBusy: '', lastLoadedAt: null, sessionMode: 'unknown', loginUsername: '', loginPassword: '',
  recoveryOpen: false, recoveryCopyStatus: '',
};

const PAGE_META = {
  dashboard: { title: '运营总览', kicker: 'WORKBENCH', subtitle: '集中查看服务健康度、资源状态和待处理任务。' },
  dns: { title: 'DNS 记录', kicker: 'DNS', subtitle: '由隧道和网站映射产生的域名解析状态。' },
  domains: { title: '域名', kicker: 'DOMAINS', subtitle: '统一查看已接入域名、HTTPS 和映射关系。' },
  frp: { title: 'FRP 运行时', kicker: 'FRP', subtitle: '管理 FRPC 进程，并核对节点和隧道运行状态。' },
  website: { title: '网站隧道', kicker: 'WEBSITE TUNNELS', subtitle: 'HTTP/HTTPS 隧道与网站映射的交付视图。' },
  jobs: { title: '任务中心', kicker: 'JOBS', subtitle: '查看异步任务执行状态和事件时间线。' },
  nodes: { title: '节点', kicker: 'NODES', subtitle: '查看节点来源、区域、连接与健康状态。' },
  tunnels: { title: '隧道', kicker: 'TUNNELS', subtitle: '管理已配置的转发规则与上线状态。' },
  websites: { title: '网站映射', kicker: 'WEBSITE', subtitle: '查看域名到站点代理的映射与同步状态。' },
  logs: { title: '操作日志', kicker: 'LOGS', subtitle: '审计用户与系统的重要变更记录。' },
  settings: { title: '系统设置', kicker: 'SETTINGS', subtitle: '核对运行配置、集成凭据与登录会话。' },
};
const NAV_ITEMS = [['dashboard','总览'],['dns','DNS'],['domains','域名'],['frp','FRP'],['website','网站隧道'],['jobs','任务'],['nodes','节点'],['tunnels','隧道'],['websites','网站映射'],['logs','日志'],['settings','设置']];
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
  if (!response.ok) { const error = new Error(parsed?.error?.message || (typeof parsed === 'string' ? parsed : `HTTP ${response.status}`)); error.status = response.status; throw error; }
  return parsed;
}

async function loadSnapshot() {
  STATE.loading = true; STATE.error = ''; render();
  try {
    const [version, health, me] = await Promise.all([
      request('/version').catch(() => ({ data: {} })), request('/health').catch(() => ({ data: {} })),
      request('/auth/me').catch((err) => err?.status === 401 ? null : Promise.reject(err)),
    ]);
    STATE.version = version?.data || null; STATE.health = health?.data || null; STATE.authMe = me?.data || null; STATE.sessionMode = STATE.authMe ? 'authenticated' : 'anonymous';
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

function renderSettings() {
  const runtime = STATE.settings?.frpc_runtime || {}; const integrations = [['chmlfrp','chmlfrp'], ['onepanel','OnePanel'], ['cloudflare','Cloudflare']].map(([key,label]) => { const item = integrationState(key); return `<tr><td><strong>${label}</strong></td><td>${item.configured || item.has_password || item.has_api_token ? statusBadge('configured') : statusBadge('not configured')}</td><td>${esc(item.identifier || item.username || item.base_url || item.zone_name || '—')}</td><td>${esc(fmtTime(item.updated_at || item.last_verified))}</td><td>${item.last_error ? `<span class="error-text">${esc(item.last_error)}</span>` : '—'}</td></tr>`; });
  const tokenRows = STATE.authTokens.map((token) => `<tr><td class="mono">${esc(shortID(token.id))}</td><td>${esc(token.token_type || 'session')}</td><td>${token.revoked_at ? statusBadge('revoked') : statusBadge('active')}</td><td>${esc(fmtTime(token.last_used_at))}</td><td>${esc(fmtTime(token.expires_at))}</td><td>${token.revoked_at ? '—' : actionButton('吊销','token-revoke',{id:token.id,ghost:true})}</td></tr>`);
  return pageCard('settings', `${viewHeader('settings', actionButton('刷新设置','reload',{secondary:true}))}<div class="split-grid"><div class="panel"><div class="panel-head"><h3>运行配置</h3>${statusBadge(runtime.frpc_enabled ? 'enabled' : 'disabled')}</div>${infoRows([['日志级别', esc(runtime.frpc_log_level || '—')], ['自动恢复', esc(runtime.auto_recover_strategy || '—')], ['健康检查间隔', esc(runtime.frpc_healthcheck_interval || '—')], ['同步轮询间隔', esc(STATE.settings?.sync?.sync_poll_interval || '—')]])}</div><div class="panel"><div class="panel-head"><h3>安全提示</h3></div><p class="muted">修改密码需要验证旧密码。忘记管理员密码时，请在服务器终端运行恢复命令；恢复会吊销所有旧会话。</p><button class="secondary" id="forgot-password-btn">忘记密码？查看恢复命令</button></div></div><div class="panel"><div class="panel-head"><h3>集成状态</h3></div>${renderTable(['集成','状态','标识','更新时间','最近错误'], integrations, '暂无集成信息', '保存集成配置后会显示凭据状态，不会显示密钥内容。')}</div><div class="panel"><div class="panel-head"><h3>登录会话</h3><span class="muted">吊销会使对应 Session 或 API Token 立即失效。</span></div>${renderTable(['令牌 ID','类型','状态','最近使用','过期时间','操作'], tokenRows, '暂无活跃会话', '成功登录后会自动创建会话记录。')}</div>`);
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
async function submitLogin() { const username = $('login-username')?.value.trim() || ''; const password = $('login-password')?.value || ''; STATE.loginUsername = username; if (!username || !password) { STATE.error = '请输入管理员用户名和密码。'; render(); return; } try { const response = await request('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) }); if (response?.data?.auth?.token) document.cookie = `ashan_frp_session=${response.data.auth.token}; path=/`; STATE.loginPassword = ''; await loadSnapshot(); } catch (err) { STATE.loginPassword = ''; STATE.error = apiError(err); render(); } }
function appShell() { const auth = STATE.authMe; const activeMeta = PAGE_META[STATE.activePage] || PAGE_META.dashboard; const body = auth ? `<div class="layout"><aside class="sidebar"><div class="nav-group"><div class="nav-group-title">运营模块</div><div class="nav-list">${renderNav()}</div></div></aside><main class="content"><div class="view-stack">${renderDashboard()}${renderDNS()}${renderDomains()}${renderFRP()}${renderWebsiteTunnels()}${renderJobs()}${renderNodes()}${renderTunnels()}${renderWebsites()}${renderLogs()}${renderSettings()}</div></main></div>` : `<div class="layout anonymous-layout"><main class="content"><div class="view-stack">${pageCard('dashboard', `${viewHeader('dashboard')}<div class="panel">${loginPanel()}</div>`)}</div></main></div>`; return `<div class="shell"><header class="hero"><div class="hero-left"><div class="eyebrow">Ashan FRP Console</div><h1 class="title">${esc(activeMeta.title)}</h1><p class="subtitle">${esc(activeMeta.subtitle)}</p><div class="section-gap login-state ${auth ? 'good' : 'warn'}"><span class="dot"></span><div class="text"><strong>${auth ? `已登录：${esc(auth.display_name || auth.login_name || auth.id)}` : '尚未登录'}</strong><span>API：${esc(STATE.apiBase)} · UI：${esc(STATE.uiBase)}</span></div></div>${STATE.error ? `<div class="error-box" style="display:block">${esc(STATE.error)}</div>` : ''}${STATE.notice ? `<div class="notice-box">${esc(STATE.notice)}</div>` : ''}</div><div class="toolbar"><span class="badge fresh">版本 ${esc(STATE.version?.version || '—')}</span><span class="badge ${STATE.health?.status === 'healthy' ? 'good' : 'warn'}">服务 ${esc(STATE.health?.status || '—')}</span><button class="secondary" data-action="reload">刷新</button></div></header>${body}</div>${recoveryDialog()}`; }
function render() { const root = $(APP_ROOT_ID); if (!root) return; root.innerHTML = appShell(); document.querySelectorAll('[data-page]').forEach((button) => button.addEventListener('click', () => { STATE.activePage = button.dataset.page; render(); })); document.querySelectorAll('[data-job-id]').forEach((row) => row.addEventListener('click', () => loadSelectedJob(row.dataset.jobId))); document.querySelectorAll('[data-action]').forEach((button) => button.addEventListener('click', () => runAction(button.dataset.action, button.dataset.id))); const login = $('login-form'); if (login) login.addEventListener('submit', (event) => { event.preventDefault(); submitLogin(); }); $('forgot-password-btn')?.addEventListener('click', openRecoveryDialog); $('recovery-close-btn')?.addEventListener('click', closeRecoveryDialog); $('recovery-confirm-btn')?.addEventListener('click', closeRecoveryDialog); $('recovery-backdrop')?.addEventListener('click', (event) => { if (event.target.id === 'recovery-backdrop') closeRecoveryDialog(); }); document.querySelectorAll('[data-copy-recovery]').forEach((button) => button.addEventListener('click', () => copyRecoveryCommand(button.dataset.copyRecovery))); }
function bootHtml(message) { return `<div class="boot-screen"><div class="boot-card"><div class="boot-kicker">Ashan FRP</div><h1>${esc(message)}</h1><p>正在加载运营数据与服务状态。</p></div></div>`; }
let setupDone = false; function setup() { const root = $(APP_ROOT_ID); if (!root) { document.body.innerHTML = bootHtml('缺少页面容器'); return; } if (setupDone) return; setupDone = true; document.addEventListener('keydown', (event) => { if (event.key === 'Escape' && STATE.recoveryOpen) closeRecoveryDialog(); }); root.innerHTML = bootHtml('运营台加载中…'); loadSnapshot(); }
document.addEventListener('DOMContentLoaded', setup);
