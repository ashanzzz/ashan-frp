const APP_ROOT_ID = 'app';
const API_PREFIX = '/api/v1';
const STATE = {
  apiBase: API_PREFIX,
  uiBase: '/ui',
  version: null,
  health: null,
  dashboard: null,
  settings: null,
  nodes: [],
  tunnels: [],
  websites: [],
  jobs: [],
  authMe: null,
  activePage: 'dashboard',
  activeNodeId: '',
  activeTunnelId: '',
  activeWebsiteId: '',
  activeJobId: '',
  error: '',
  loading: false,
  lastLoadedAt: null,
  sessionMode: 'unknown',
};

const PAGE_META = {
  dashboard: {
    title: '运营总览',
    kicker: 'WORKBENCH',
    subtitle: '从这里进入 DNS 管理、域名、FRP 管理、chmlfrp 和网站隧道管理。首屏只保留真正需要的导航与状态，具体工作流在各页面展开。',
    cta: '刷新总览',
  },
  dns: {
    title: 'DNS 管理',
    kicker: 'DNS',
    subtitle: '查看 Cloudflare 相关连接状态、域名解析骨架和当前绑定的隧道映射。',
    cta: '刷新 DNS 面板',
  },
  domains: {
    title: '域名',
    kicker: 'DOMAINS',
    subtitle: '集中查看主域名、子域名、解析目标与站点映射，保持域名与网站/隧道的对应关系清晰。',
    cta: '刷新域名面板',
  },
  frp: {
    title: 'FRP 管理',
    kicker: 'FRP',
    subtitle: '查看 FRPC 运行态、节点、隧道和最近作业状态，作为后续管理操作的工作中心。',
    cta: '刷新 FRP 面板',
  },
  chmlfrp: {
    title: 'chmlfrp',
    kicker: 'CHMLFRP',
    subtitle: '展示 chmlfrp 凭据、节点和绑定隧道的状态骨架，后续可在此补充专用操作入口。',
    cta: '刷新 chmlfrp 面板',
  },
  website: {
    title: '网站隧道管理',
    kicker: 'WEBSITE TUNNELS',
    subtitle: '汇总网站映射、隧道绑定、协议、目标地址和同步状态，为网站接入与切换提供入口。',
    cta: '刷新网站隧道面板',
  },
  settings: {
    title: '系统设置',
    kicker: 'SETTINGS',
    subtitle: '查看一般设置、同步策略、队列和 FRPC 运行配置，并展示云端集成状态。',
    cta: '刷新设置',
  },
};

const NAV_SECTIONS = [
  {
    title: '主页面',
    items: [
      { id: 'dashboard', title: '运营总览', hint: '首页 / 状态 / 导航' },
      { id: 'dns', title: 'DNS 管理', hint: '解析 / Cloudflare' },
      { id: 'domains', title: '域名', hint: '主域名 / 绑定' },
      { id: 'frp', title: 'FRP 管理', hint: '运行态 / 节点 / 隧道' },
      { id: 'website', title: '网站隧道管理', hint: '站点映射 / 同步' },
      { id: 'settings', title: '系统设置', hint: '策略 / 集成 / 运行' },
    ],
  },
];

function $(id) { return document.getElementById(id); }
function esc(value) {
  return String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}
function fmt(value) {
  if (value === null || value === undefined || value === '') return '—';
  if (value instanceof Date) return value.toLocaleString();
  if (typeof value === 'boolean') return value ? '是' : '否';
  if (typeof value === 'number') return Number.isFinite(value) ? String(value) : '—';
  if (typeof value === 'object') return JSON.stringify(value, null, 2);
  return String(value);
}
function fmtTime(value) {
  if (!value) return '—';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return String(value);
  return d.toLocaleString();
}
function truthyBadge(value, yes = '已配置', no = '未配置') {
  return `<span class="badge ${value ? 'good' : 'warn'}">${value ? yes : no}</span>`;
}
function statusBadge(status) {
  const s = String(status || '').toLowerCase();
  if (['healthy', 'online', 'synced', 'enabled', 'succeeded', 'running', 'active'].includes(s)) {
    return `<span class="badge good">${esc(status)}</span>`;
  }
  if (['degraded', 'pending', 'queued', 'retry_wait', 'blocked'].includes(s)) {
    return `<span class="badge warn">${esc(status)}</span>`;
  }
  if (['offline', 'error', 'failed', 'canceled', 'archived', 'disabled', 'conflict'].includes(s)) {
    return `<span class="badge bad">${esc(status)}</span>`;
  }
  return `<span class="badge">${esc(status || '—')}</span>`;
}
function shortJSON(value, fallback = '—') {
  if (value === null || value === undefined || value === '') return fallback;
  if (typeof value === 'string') return value;
  try {
    const text = JSON.stringify(value);
    return text.length > 120 ? `${text.slice(0, 117)}…` : text;
  } catch {
    return String(value);
  }
}
function domainListFromMapping(row) {
  const list = [];
  if (row.primary_domain) list.push(row.primary_domain);
  if (Array.isArray(row.domains)) list.push(...row.domains);
  return [...new Set(list.filter(Boolean))];
}
function fullDomainForTunnel(tunnel) {
  return tunnel.full_domain || [tunnel.subdomain, tunnel.project_name].filter(Boolean).join('.') || '—';
}
function tunnelSummary(tunnel) {
  const bits = [];
  if (tunnel.protocol) bits.push(tunnel.protocol.toUpperCase());
  if (tunnel.local_ip || tunnel.local_port) bits.push(`${tunnel.local_ip || '—'}:${tunnel.local_port ?? '—'}`);
  if (tunnel.remote_port) bits.push(`remote ${tunnel.remote_port}`);
  if (tunnel.tunnel_type) bits.push(tunnel.tunnel_type);
  return bits.join(' · ') || '—';
}
function nodeSummary(node) {
  return [node.provider, node.node_type, node.region, node.endpoint_url].filter(Boolean).join(' · ') || '—';
}
function websiteSummary(row) {
  return [row.source_kind, row.proxy_target, row.certificate_mode].filter(Boolean).join(' · ') || '—';
}
function safeArray(value) { return Array.isArray(value) ? value : []; }

function formToObject(form) {
  const data = new FormData(form);
  const out = {};
  for (const [key, value] of data.entries()) {
    if (key in out) {
      if (!Array.isArray(out[key])) out[key] = [out[key]];
      out[key].push(value);
    } else {
      out[key] = value;
    }
  }
  return out;
}

function uiBase() {
  const base = STATE.uiBase || '/ui';
  return base.endsWith('/') ? base.slice(0, -1) : base;
}
function apiUrl(path) {
  const base = STATE.apiBase || API_PREFIX;
  return `${base}${path.startsWith('/') ? path : `/${path}`}`;
}
function uiUrl(path = '') {
  const base = uiBase();
  const suffix = path ? (path.startsWith('/') ? path : `/${path}`) : '';
  return `${base}${suffix}`;
}

async function request(path, options = {}) {
  const response = await fetch(apiUrl(path), {
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
    ...options,
  });
  const text = await response.text();
  let parsed = null;
  try { parsed = text ? JSON.parse(text) : null; } catch { parsed = text; }
  if (!response.ok) {
    const message = (parsed && parsed.error && parsed.error.message) || (typeof parsed === 'string' ? parsed : `HTTP ${response.status}`);
    const error = new Error(message);
    error.status = response.status;
    error.payload = parsed;
    throw error;
  }
  return parsed;
}

async function loadSnapshot() {
  STATE.loading = true;
  STATE.error = '';
  render();
  try {
    const [version, health, dashboard, settings, nodes, tunnels, websites, jobs] = await Promise.all([
      request('/version').catch(() => ({ data: {} })),
      request('/health').catch(() => ({ data: {} })),
      request('/dashboard').catch(() => ({ data: {} })),
      request('/settings').catch(() => ({ data: {} })),
      request('/nodes').catch(() => ({ data: { nodes: [] } })),
      request('/tunnels').catch(() => ({ data: { tunnels: [] } })),
      request('/website-mappings').catch(() => ({ data: { website_mappings: [] } })),
      request('/jobs').catch(() => ({ data: { jobs: [] } })),
    ]);
    STATE.version = version?.data || null;
    STATE.health = health?.data || null;
    STATE.dashboard = dashboard?.data || null;
    STATE.settings = settings?.data || null;
    STATE.nodes = safeArray(nodes?.data?.nodes ?? nodes?.data ?? []);
    STATE.tunnels = safeArray(tunnels?.data?.tunnels ?? tunnels?.data ?? []);
    STATE.websites = safeArray(websites?.data?.website_mappings ?? websites?.data ?? []);
    STATE.jobs = safeArray(jobs?.data?.jobs ?? jobs?.data ?? []);
    STATE.lastLoadedAt = new Date();
    const me = await request('/auth/me').catch((err) => {
      if (err && err.status === 401) return null;
      throw err;
    });
    STATE.authMe = me?.data || null;
    STATE.sessionMode = STATE.authMe ? 'authenticated' : 'anonymous';
    STATE.error = '';
  } catch (err) {
    STATE.error = err?.message || String(err);
  } finally {
    STATE.loading = false;
    render();
  }
}

function appShell() {
  const version = STATE.version || {};
  const health = STATE.health || {};
  const auth = STATE.authMe;
  const loginStateClass = auth ? 'good' : (STATE.sessionMode === 'anonymous' ? 'warn' : 'bad');
  const loginText = auth
    ? `已登录 · ${auth.display_name || auth.login_name || auth.id || 'unknown'}`
    : STATE.sessionMode === 'anonymous'
      ? '未登录 / 需要登录后才能访问大多数资源页面'
      : '登录状态未知';

  return `
    <div class="shell">
      <header class="hero">
        <div class="hero-left">
          <div class="eyebrow">Ashan FRP 运营台</div>
          <h1 class="title">${esc((PAGE_META[STATE.activePage] || PAGE_META.dashboard).title)}</h1>
          <p class="subtitle">${esc((PAGE_META[STATE.activePage] || PAGE_META.dashboard).subtitle)}</p>
          <div class="section-gap login-state ${loginStateClass}">
            <span class="dot"></span>
            <div class="text">
              <strong>${esc(loginText)}</strong>
              <span>API Base: <span class="mono">${esc(version.api_base || STATE.apiBase)}</span> · UI Base: <span class="mono">${esc(version.ui_base || uiBase())}</span></span>
            </div>
          </div>
          ${STATE.error ? `<div id="error-box" class="error-box" style="display:block">${esc(STATE.error)}</div>` : '<div id="error-box" class="error-box"></div>'}
        </div>
        <div class="toolbar">
          <span class="badge fresh">版本 ${esc(version.version || '—')}</span>
          <span class="badge ${health.status === 'healthy' ? 'good' : 'warn'}">健康 ${esc(health.status || '—')}</span>
          <span class="badge">隧道 ${esc(health.tunnels ?? '—')}</span>
          <span class="badge">作业 ${esc(health.jobs ?? '—')}</span>
          <span class="badge ${STATE.loading ? 'warn' : 'good'}">${STATE.loading ? '加载中…' : '已就绪'}</span>
          <button class="secondary" id="refresh-btn">刷新</button>
        </div>
      </header>

      <div class="layout">
        <aside class="sidebar">
          ${NAV_SECTIONS.map(renderNavSection).join('')}
          <div class="panel sidebar-note">
            <strong>当前页面骨架</strong>
            <div class="footer-note">先把导航和页面分区做完整，后续再按页面补操作按钮与表单。域名、DNS、FRP、chmlfrp、网站隧道都已经拆成独立入口。</div>
          </div>
        </aside>

        <main class="content">
          <div class="view-stack">
            ${renderDashboardPage()}
            ${renderDnsPage()}
            ${renderDomainsPage()}
            ${renderFrpPage()}
            ${renderWebsitePage()}
            ${renderSettingsPage()}
          </div>
        </main>
      </div>
    </div>
  `;
}

function renderNavSection(section) {
  return `
    <nav class="nav-group">
      <div class="nav-group-title">${esc(section.title)}</div>
      <div class="nav-list">
        ${section.items.map((item) => `
          <button class="nav-item ${STATE.activePage === item.id ? 'active' : ''}" data-page="${esc(item.id)}">
            <span>
              <strong>${esc(item.title)}</strong><br />
              <small>${esc(item.hint)}</small>
            </span>
            <span>${STATE.activePage === item.id ? '●' : '○'}</span>
          </button>
        `).join('')}
      </div>
    </nav>
  `;
}

function pageCard(id, body) {
  const active = STATE.activePage === id ? 'active' : '';
  return `<section class="view ${active}" data-view="${esc(id)}">${body}</section>`;
}

function viewHeader(idOrMeta, maybeMeta, maybeActions = '') {
  const meta = maybeMeta && typeof maybeMeta === 'object' ? maybeMeta : idOrMeta;
  const actions = maybeMeta && typeof maybeMeta === 'object' ? maybeActions : (maybeMeta || '');
  return `
    <div class="view-head">
      <div>
        <div class="eyebrow">${esc(meta.kicker)}</div>
        <h2>${esc(meta.title)}</h2>
        <p>${esc(meta.subtitle)}</p>
      </div>
      <div class="view-actions">
        ${actions}
      </div>
    </div>
  `;
}

function renderDashboardPage() {
  const version = STATE.version || {};
  const health = STATE.health || {};
  const dashboard = STATE.dashboard || {};
  const settings = STATE.settings || {};
  const tunnels = STATE.tunnels.length ? STATE.tunnels : safeArray(dashboard.tunnels);
  const jobs = STATE.jobs.length ? STATE.jobs : safeArray(dashboard.jobs);
  const nodes = STATE.nodes.length;
  const websites = STATE.websites.length;
  const latestAudit = safeArray(dashboard.recent_audit_logs || []).slice(0, 5);

  return pageCard('dashboard', `
    ${viewHeader('dashboard', PAGE_META.dashboard, `<button class="secondary" id="refresh-dashboard">${esc(PAGE_META.dashboard.cta)}</button>`)}
    <div class="page-card">
      <div class="kpi-grid">
        <div class="kpi"><div class="kpi-label">健康状态</div><div class="kpi-value">${esc(health.status || '—')}</div><div class="kpi-desc">API / UI 基础状态。${health.status === 'healthy' ? '系统当前看起来可用。' : '如有异常，可继续点开页面查看细分模块。'}</div></div>
        <div class="kpi"><div class="kpi-label">节点 / 隧道 / 网站</div><div class="kpi-value">${nodes} / ${tunnels.length} / ${websites}</div><div class="kpi-desc">入口已经拆分成独立页面，方便逐项管理与核对。</div></div>
        <div class="kpi"><div class="kpi-label">作业队列</div><div class="kpi-value">${jobs.length}</div><div class="kpi-desc">展示 queued / running / failed 等状态，便于判断同步与执行结果。</div></div>
      </div>
    </div>
    <div class="workbench-grid">
      <div class="panel span-2">
        <h2>当前总览 <small>版本、基础路径、设置快照</small></h2>
        <div class="grid">
          <div class="detail-card">
            <div class="detail-kicker">Version</div>
            <h3>${esc(version.app_name || 'Ashan FRP')}</h3>
            <div class="meta-list">
              <div class="meta-row"><span>版本</span><span class="mono">${esc(version.version || '—')}</span></div>
              <div class="meta-row"><span>引擎</span><span>${esc(version.engine || '—')}</span></div>
              <div class="meta-row"><span>API Base</span><span class="mono">${esc(version.api_base || STATE.apiBase)}</span></div>
              <div class="meta-row"><span>UI Base</span><span class="mono">${esc(version.ui_base || uiBase())}</span></div>
            </div>
          </div>
          <div class="detail-card">
            <div class="detail-kicker">Settings</div>
            <h3>运行策略</h3>
            <div class="meta-list">
              <div class="meta-row"><span>刷新模式</span><span>${esc(settings.general?.default_refresh_mode || '—')}</span></div>
              <div class="meta-row"><span>健康检查</span><span>${esc(settings.sync?.healthcheck_interval || '—')}</span></div>
              <div class="meta-row"><span>队列重试</span><span>${esc(settings.queue?.retry_backoff || '—')}</span></div>
              <div class="meta-row"><span>FRPC 版本</span><span class="mono">${esc(settings.frpc_runtime?.frpc_binary_version || '—')}</span></div>
            </div>
          </div>
        </div>
      </div>
      <div class="panel">
        <h2>登录与集成 <small>真实状态</small></h2>
        <div class="meta-list">
          <div class="meta-row"><span>登录状态</span><span>${STATE.authMe ? '已登录' : '未登录'}</span></div>
          <div class="meta-row"><span>ChmlFrp</span><span>${renderCredentialMini(settings.integrations?.chmlfrp)}</span></div>
          <div class="meta-row"><span>OnePanel</span><span>${renderCredentialMini(settings.integrations?.onepanel)}</span></div>
          <div class="meta-row"><span>Cloudflare</span><span>${renderCredentialMini(settings.integrations?.cloudflare)}</span></div>
        </div>
        <div class="footer-note">这里仅显示可从后端确认的状态，不伪造“已连接”。</div>
      </div>
      <div class="panel span-3">
        <h2>最近审计 / 作业 <small>便于检查同步链路</small></h2>
        <div class="rack">
          <div>
            <h3>最近作业</h3>
            <div class="table-wrap">${renderJobsTable(jobs.slice(0, 6))}</div>
          </div>
          <div>
            <h3>最近审计</h3>
            <div class="event-log">
              ${latestAudit.length ? latestAudit.map((log) => `
                <div class="event-item">
                  <div class="head"><span class="kind">${esc(log.action || log.kind || 'event')}</span><span class="muted">${esc(fmtTime(log.created_at))}</span></div>
                  <div class="tiny muted">${esc(shortJSON(log.details || log.payload || log.message || '—'))}</div>
                </div>
              `).join('') : '<div class="placeholder">暂无审计记录。</div>'}
            </div>
          </div>
        </div>
      </div>
      <div class="panel span-3">
        <h2>待办入口 <small>你要的五个主页面已经拆开</small></h2>
        <div class="grid">
          ${[
            ['dns', 'DNS 管理', 'Cloudflare 解析、状态和域名映射入口'],
            ['domains', '域名', '主域名、别名、站点绑定'],
            ['frp', 'FRP 管理', '运行态、节点、隧道、作业'],
            ['chmlfrp', 'chmlfrp', '第三方集成和绑定状态'],
            ['website', '网站隧道管理', '映射、协议、目标和同步状态'],
          ].map(([id, title, desc]) => `
            <div class="detail-card">
              <div class="detail-kicker">${esc(id)}</div>
              <h3>${esc(title)}</h3>
              <p class="muted">${esc(desc)}</p>
              <div class="settings-actions"><button class="secondary" data-page="${esc(id)}">打开页面</button></div>
            </div>
          `).join('')}
        </div>
      </div>
    </div>
  `);
}

function renderDnsPage() {
  const settings = STATE.settings || {};
  const cloudflare = settings.integrations?.cloudflare || {};
  const tunnels = STATE.tunnels;
  const websites = STATE.websites;
  return pageCard('dns', `
    ${viewHeader('dns', PAGE_META.dns, `<button class="secondary" data-refresh="dns">${esc(PAGE_META.dns.cta)}</button>`) }
    <div class="grid">
      <div class="panel">
        <h2>Cloudflare 连接状态 <small>真实凭据状态</small></h2>
        <div class="meta-list">
          <div class="meta-row"><span>Zone / 域名</span><span class="mono">${esc(cloudflare.zone_name || '—')}</span></div>
          <div class="meta-row"><span>API Token</span><span>${truthyBadge(!!cloudflare.has_api_token)}</span></div>
          <div class="meta-row"><span>最后验证</span><span>${esc(fmtTime(cloudflare.last_validated_at))}</span></div>
          <div class="meta-row"><span>最后错误</span><span class="muted">${esc(cloudflare.last_error_message || '—')}</span></div>
        </div>
      </div>
      <div class="panel">
        <h2>DNS / 解析骨架 <small>与隧道关联</small></h2>
        <div class="meta-list">
          <div class="meta-row"><span>绑定隧道数量</span><span>${esc(tunnels.length)}</span></div>
          <div class="meta-row"><span>网站映射数量</span><span>${esc(websites.length)}</span></div>
          <div class="meta-row"><span>解析状态</span><span>${statusBadge(settings.integrations?.cloudflare?.configured ? 'configured' : 'pending')}</span></div>
          <div class="meta-row"><span>页面用途</span><span>只展示真实可见的 DNS 工作流入口</span></div>
        </div>
      </div>
      <div class="panel span-3">
        <h2>相关映射 <small>域名 / 隧道 / 站点</small></h2>
        <div class="table-wrap">${renderDnsTable(websites.slice(0, 8), tunnels.slice(0, 8))}</div>
      </div>
    </div>
  `);
}

function renderDomainsPage() {
  const websites = STATE.websites;
  const domains = websites.flatMap((w) => domainListFromMapping(w));
  const uniqueDomains = [...new Set(domains.filter(Boolean))].sort();
  const current = websites.find((w) => w.id === STATE.activeWebsiteId) || websites[0] || {};
  return pageCard('domains', `
    ${viewHeader(PAGE_META.domains, `<button class="secondary" data-refresh="domains">${esc(PAGE_META.domains.cta)}</button><button class="secondary" data-action="new-website">新建映射</button>`) }
    <div class="grid">
      <div class="panel">
        <h2>域名概览 <small>主域名 / 别名 / 数量</small></h2>
        <div class="stats">
          <div class="stat"><div class="label">唯一域名</div><div class="value">${uniqueDomains.length}</div><div class="desc">根据网站映射中的 primary_domain 与 domains 汇总。</div></div>
          <div class="stat"><div class="label">网站映射</div><div class="value">${websites.length}</div><div class="desc">每个映射都可以带一个主域名和多个别名。</div></div>
          <div class="stat"><div class="label">当前高亮</div><div class="value">${current.id ? '1' : '0'}</div><div class="desc">用于编辑区默认载入的记录。</div></div>
        </div>
      </div>
      <div class="panel">
        <h2>网站映射编辑 <small>可直接修改网站配置</small></h2>
        <form id="website-form" class="settings-form">
          <div class="mini-grid">
            <label class="wide">映射 ID<input name="id" value="${esc(current.id || '')}" placeholder="web_xxx" /></label>
            <label>来源类型<input name="source_kind" value="${esc(current.source_kind || 'manual')}" placeholder="manual / chmlfrp / tunnel" /></label>
            <label>Node ID<input name="node_id" value="${esc(current.node_id || '')}" placeholder="node_xxx" /></label>
            <label>Tunnel ID<input name="tunnel_id" value="${esc(current.tunnel_id || '')}" placeholder="tunnel_xxx" /></label>
            <label>主域名<input name="primary_domain" value="${esc(current.primary_domain || '')}" placeholder="example.com" /></label>
            <label>站点别名<input name="website_alias" value="${esc(current.website_alias || '')}" placeholder="site alias" /></label>
            <label class="wide">域名别名（逗号分隔）<input name="domains" value="${esc((current.domains || []).join(', '))}" placeholder="www.example.com, m.example.com" /></label>
            <label>代理目标<input name="proxy_target" value="${esc(current.proxy_target || '')}" placeholder="http://127.0.0.1:8080" /></label>
            <label>证书模式<input name="certificate_mode" value="${esc(current.certificate_mode || '')}" placeholder="auto / custom / none" /></label>
            <label>SSL 证书引用<input name="ssl_certificate_ref" value="${esc(current.ssl_certificate_ref || '')}" placeholder="cert_xxx" /></label>
            <label><span>HTTPS</span><select name="https_enabled"><option value="true" ${current.https_enabled ? 'selected' : ''}>启用</option><option value="false" ${!current.https_enabled ? 'selected' : ''}>关闭</option></select></label>
            <label><span>代理</span><select name="proxy_enabled"><option value="true" ${current.proxy_enabled ? 'selected' : ''}>启用</option><option value="false" ${!current.proxy_enabled ? 'selected' : ''}>关闭</option></select></label>
            <label><span>缓存</span><select name="cache_enabled"><option value="true" ${current.cache_enabled ? 'selected' : ''}>启用</option><option value="false" ${!current.cache_enabled ? 'selected' : ''}>关闭</option></select></label>
            <label class="wide">冲突策略<input name="conflict_strategy" value="${esc(current.conflict_strategy || '')}" placeholder="manual_wins / pause_on_conflict / ..." /></label>
            <label class="wide">HTTP 配置 JSON<textarea name="http_config" placeholder='{"headers":{}}'>${esc(current.http_config ? JSON.stringify(current.http_config, null, 2) : '')}</textarea></label>
          </div>
          <div class="settings-actions">
            <button type="submit">保存网站映射</button>
            <button type="button" class="secondary" data-action="clear-website-form">清空</button>
          </div>
          <div class="footer-note">保存会调用 <span class="mono">POST /api/v1/website-mappings</span> 或 <span class="mono">PATCH /api/v1/website-mappings/:id</span>。</div>
        </form>
      </div>
      <div class="panel span-3">
        <h2>域名列表 <small>从网站映射聚合</small></h2>
        <div class="table-wrap">${renderDomainTable(uniqueDomains, websites)}</div>
      </div>
    </div>
  `);
}

function renderFrpPage() {
  const settings = STATE.settings || {};
  const runtime = settings.frpc_runtime || {};
  const tunnels = STATE.tunnels;
  const nodes = STATE.nodes;
  const jobs = STATE.jobs;
  return pageCard('frp', `
    ${viewHeader('frp', PAGE_META.frp, `<button class="secondary" data-refresh="frp">${esc(PAGE_META.frp.cta)}</button>`) }
    <div class="grid">
      <div class="panel">
        <h2>FRPC 运行态 <small>配置与生命周期</small></h2>
        <div class="meta-list">
          <div class="meta-row"><span>启用</span><span>${truthyBadge(!!runtime.frpc_enabled, '启用', '未启用')}</span></div>
          <div class="meta-row"><span>Binary Source</span><span class="mono">${esc(runtime.frpc_binary_source || '—')}</span></div>
          <div class="meta-row"><span>Binary Version</span><span class="mono">${esc(runtime.frpc_binary_version || '—')}</span></div>
          <div class="meta-row"><span>Log Level</span><span class="mono">${esc(runtime.frpc_log_level || '—')}</span></div>
          <div class="meta-row"><span>Healthcheck</span><span class="mono">${esc(runtime.frpc_healthcheck_interval || '—')}</span></div>
          <div class="meta-row"><span>恢复策略</span><span>${esc(runtime.auto_recover_strategy || '—')}</span></div>
        </div>
      </div>
      <div class="panel">
        <h2>节点概览 <small>可切换 FRP 目标</small></h2>
        <div class="stats">
          <div class="stat"><div class="label">节点数</div><div class="value">${nodes.length}</div><div class="desc">来自 /nodes 的真实列表。</div></div>
          <div class="stat"><div class="label">当前隧道</div><div class="value">${tunnels.length}</div><div class="desc">来自 /tunnels 的真实列表。</div></div>
          <div class="stat"><div class="label">作业</div><div class="value">${jobs.length}</div><div class="desc">展示同步、应用、修复等作业。</div></div>
        </div>
      </div>
      <div class="panel span-3">
        <h2>FRP 管理工作台 <small>节点 / 隧道 / 作业</small></h2>
        <div class="rack">
          <div>
            <h3>节点</h3>
            <div class="table-wrap">${renderNodeTable(nodes)}</div>
          </div>
          <div>
            <h3>隧道</h3>
            <div class="table-wrap">${renderTunnelTable(tunnels)}</div>
          </div>
        </div>
        <div class="section-gap table-wrap">${renderJobsTable(jobs.slice(0, 10))}</div>
      </div>
      <div class="panel span-3">
        <h2>chmlfrp 集成区 <small>并入 FRP 页面，不再单独拆页</small></h2>
        <div class="grid">
          <div class="detail-card">
            <div class="detail-kicker">凭据状态</div>
            <div class="meta-list">
              <div class="meta-row"><span>用户名</span><span class="mono">${esc(settings.integrations?.chmlfrp?.username || '—')}</span></div>
              <div class="meta-row"><span>密码</span><span>${truthyBadge(!!settings.integrations?.chmlfrp?.has_password, '已保存', '未保存')}</span></div>
              <div class="meta-row"><span>最后验证</span><span>${esc(fmtTime(settings.integrations?.chmlfrp?.last_validated_at))}</span></div>
              <div class="meta-row"><span>最后错误</span><span class="muted">${esc(settings.integrations?.chmlfrp?.last_error_message || '—')}</span></div>
            </div>
          </div>
          <div class="detail-card">
            <div class="detail-kicker">相关节点 / 隧道</div>
            <div class="meta-list">
              <div class="meta-row"><span>相关节点</span><span>${nodes.filter((n) => (n.provider || '').toLowerCase().includes('chml') || (n.node_type || '').toLowerCase().includes('chml')).length}</span></div>
              <div class="meta-row"><span>相关隧道</span><span>${tunnels.filter((t) => (t.chmlfrp_node || '').length || (t.chmlfrp_tunnel_id || '').length).length}</span></div>
              <div class="meta-row"><span>页面入口</span><span class="mono">frp</span></div>
            </div>
          </div>
        </div>
      </div>
    </div>
  `);
}

function renderChmlfrpPage() {
  return '';
}

function renderWebsitePage() {
  const websites = STATE.websites;
  const tunnels = STATE.tunnels;
  return pageCard('website', `
    ${viewHeader('website', PAGE_META.website, `<button class="secondary" data-refresh="website">${esc(PAGE_META.website.cta)}</button>`) }
    <div class="grid">
      <div class="panel">
        <h2>网站映射概览 <small>域名 / 目标 / 状态</small></h2>
        <div class="stats">
          <div class="stat"><div class="label">映射数量</div><div class="value">${websites.length}</div><div class="desc">/website-mappings 的真实记录。</div></div>
          <div class="stat"><div class="label">绑定隧道</div><div class="value">${tunnels.length}</div><div class="desc">作为网站流量入口的基础隧道。</div></div>
          <div class="stat"><div class="label">同步状态</div><div class="value">${websites.filter((w) => w.status === 'synced').length}</div><div class="desc">仅根据后端真实状态统计。</div></div>
        </div>
      </div>
      <div class="panel">
        <h2>站点字段骨架 <small>面向后续编辑</small></h2>
        <div class="meta-list">
          <div class="meta-row"><span>主域名</span><span class="mono">primary_domain</span></div>
          <div class="meta-row"><span>别名</span><span class="mono">domains[]</span></div>
          <div class="meta-row"><span>代理目标</span><span class="mono">proxy_target</span></div>
          <div class="meta-row"><span>证书</span><span class="mono">certificate_mode / ssl_certificate_ref</span></div>
        </div>
      </div>
      <div class="panel span-3">
        <h2>网站映射列表 <small>当前可见骨架</small></h2>
        <div class="table-wrap">${renderWebsiteTable(websites)}</div>
      </div>
    </div>
  `);
}

function renderSettingsPage() {
  const settings = STATE.settings || {};
  const integrations = settings.integrations || {};
  return pageCard('settings', `
    ${viewHeader('settings', PAGE_META.settings, `<button class="secondary" data-refresh="settings">${esc(PAGE_META.settings.cta)}</button>`) }
    <div class="settings-grid">
      <div class="settings-card span-2">
        <h3>系统设置编辑</h3>
        <form id="settings-form" class="settings-form">
          <div class="mini-grid">
            <label>默认日志行数<input name="general.default_log_lines" type="number" value="${esc(settings.general?.default_log_lines ?? 100)}" /></label>
            <label>数据保留天数<input name="general.data_retention_days" type="number" value="${esc(settings.general?.data_retention_days ?? 30)}" /></label>
            <label>默认刷新模式<input name="general.default_refresh_mode" value="${esc(settings.general?.default_refresh_mode || 'polling')}" /></label>
            <label>健康检查<input name="sync.healthcheck_interval" value="${esc(settings.sync?.healthcheck_interval || '1m')}" /></label>
            <label>轮询间隔<input name="sync.sync_poll_interval" value="${esc(settings.sync?.sync_poll_interval || '10s')}" /></label>
            <label>冲突策略<input name="sync.diff_strategy" value="${esc(settings.sync?.diff_strategy || 'pause_on_conflict')}" /></label>
            <label>最大重试<input name="queue.max_attempts" type="number" value="${esc(settings.queue?.max_attempts ?? 5)}" /></label>
            <label>重试退避<input name="queue.retry_backoff" value="${esc(settings.queue?.retry_backoff || '30s')}" /></label>
            <label>积压策略<input name="queue.stalled_job_policy" value="${esc(settings.queue?.stalled_job_policy || 'mark_blocked')}" /></label>
            <label>FRPC 启用<select name="frpc_runtime.frpc_enabled"><option value="true" ${settings.frpc_runtime?.frpc_enabled ? 'selected' : ''}>启用</option><option value="false" ${!settings.frpc_runtime?.frpc_enabled ? 'selected' : ''}>关闭</option></select></label>
            <label>FRPC 版本<input name="frpc_runtime.frpc_binary_version" value="${esc(settings.frpc_runtime?.frpc_binary_version || '0.54.0')}" /></label>
            <label>FRPC 日志级别<input name="frpc_runtime.frpc_log_level" value="${esc(settings.frpc_runtime?.frpc_log_level || 'info')}" /></label>
            <label>FRPC 健康检查<input name="frpc_runtime.frpc_healthcheck_interval" value="${esc(settings.frpc_runtime?.frpc_healthcheck_interval || '30s')}" /></label>
            <label>FRPC 重启退避<input name="frpc_runtime.frpc_restart_backoff" value="${esc(settings.frpc_runtime?.frpc_restart_backoff || '30s')}" /></label>
            <label>自动恢复<input name="frpc_runtime.auto_recover_strategy" value="${esc(settings.frpc_runtime?.auto_recover_strategy || 'reload_then_restart')}" /></label>
            <label>切换节点策略<input name="frpc_runtime.switch_node_strategy" value="${esc(settings.frpc_runtime?.switch_node_strategy || 'prefer_healthy_low_load')}" /></label>
          </div>
          <div class="settings-actions">
            <button type="submit">保存设置</button>
            <button type="button" class="secondary" data-action="reload-settings-form">重置为当前值</button>
          </div>
        </form>
      </div>
      <div class="settings-card span-2">
        <h3>Integrations 编辑</h3>
        <form id="integrations-form" class="settings-form">
          <div class="mini-grid">
            <label>ChmlFrp 用户名<input name="integrations.chmlfrp.username" value="${esc(integrations.chmlfrp?.username || '')}" /></label>
            <label>ChmlFrp 密码<input name="integrations.chmlfrp.password" type="password" value="${esc(integrations.chmlfrp?.password || '')}" /></label>
            <label>OnePanel Base URL<input name="integrations.onepanel.base_url" value="${esc(integrations.onepanel?.base_url || '')}" /></label>
            <label>OnePanel Entrance<input name="integrations.onepanel.entrance" value="${esc(integrations.onepanel?.entrance || '')}" /></label>
            <label>OnePanel Token<input name="integrations.onepanel.api_token" type="password" value="${esc(integrations.onepanel?.api_token || '')}" /></label>
            <label>Cloudflare Zone / 域名<input name="integrations.cloudflare.zone_name" value="${esc(integrations.cloudflare?.zone_name || '')}" /></label>
            <label>Cloudflare Token<input name="integrations.cloudflare.api_token" type="password" value="${esc(integrations.cloudflare?.api_token || '')}" /></label>
          </div>
          <div class="settings-actions">
            <button type="submit">保存集成</button>
            <button type="button" class="secondary" data-action="reload-integrations-form">重置为当前值</button>
          </div>
        </form>
      </div>
      <div class="settings-card">
        <h3>当前状态</h3>
        <div class="meta-list">
          <div class="meta-row"><span>ChmlFrp</span><span class="mono">${esc(integrations.chmlfrp?.username || '—')}</span></div>
          <div class="meta-row"><span>OnePanel</span><span class="mono">${esc(integrations.onepanel?.base_url || '—')} ${esc(integrations.onepanel?.entrance || '')}</span></div>
          <div class="meta-row"><span>Cloudflare</span><span class="mono">${esc(integrations.cloudflare?.zone_name || '—')}</span></div>
        </div>
      </div>
    </div>
  `);
}

function renderCredentialMini(cred) {
  if (!cred) return '—';
  const label = cred.configured ? '已配置' : '未配置';
  const badge = cred.configured ? 'good' : 'warn';
  const details = [cred.identifier, cred.mask_hint].filter(Boolean).join(' · ');
  return `<span class="badge ${badge}">${esc(label)}${details ? ` · ${esc(details)}` : ''}</span>`;
}

function renderJobsTable(rows) {
  const data = safeArray(rows);
  if (!data.length) return '<div class="placeholder">暂无作业。</div>';
  return `
    <table>
      <thead>
        <tr>
          <th>标题</th><th>类型</th><th>目标</th><th>状态</th><th>通道</th><th>创建时间</th>
        </tr>
      </thead>
      <tbody>
        ${data.map((row) => `
          <tr${STATE.activeJobId === row.id ? ' class="selected-row"' : ''}>
            <td><strong>${esc(row.title || row.kind || row.id)}</strong><div class="muted tiny mono">${esc(row.id || '—')}</div></td>
            <td>${esc(row.kind || '—')}</td>
            <td>${esc([row.target_type, row.target_id].filter(Boolean).join(' / ') || '—')}</td>
            <td>${statusBadge(row.status)}</td>
            <td>${esc(row.channel || '—')}</td>
            <td>${esc(fmtTime(row.created_at))}</td>
          </tr>
        `).join('')}
      </tbody>
    </table>
  `;
}

function renderNodeTable(rows) {
  const data = safeArray(rows);
  if (!data.length) return '<div class="placeholder">暂无节点。</div>';
  return `
    <table>
      <thead>
        <tr>
          <th>名称</th><th>Provider</th><th>类型</th><th>区域</th><th>状态</th><th>健康</th>
        </tr>
      </thead>
      <tbody>
        ${data.map((row) => `
          <tr${STATE.activeNodeId === row.id ? ' class="selected-row"' : ''}>
            <td><strong>${esc(row.display_name || row.canonical_name || row.id)}</strong><div class="muted tiny mono">${esc(row.id || '—')}</div></td>
            <td>${esc(row.provider || '—')}</td>
            <td>${esc(row.node_type || '—')}</td>
            <td>${esc(row.region || '—')}</td>
            <td>${statusBadge(row.status)}</td>
            <td>${statusBadge(row.health_status)}</td>
          </tr>
        `).join('')}
      </tbody>
    </table>
  `;
}

function renderTunnelTable(rows) {
  const data = safeArray(rows);
  if (!data.length) return '<div class="placeholder">暂无隧道。</div>';
  return `
    <table>
      <thead>
        <tr>
          <th>名称</th><th>全域名</th><th>节点</th><th>协议 / 端口</th><th>目标</th><th>状态</th>
        </tr>
      </thead>
      <tbody>
        ${data.map((row) => `
          <tr${STATE.activeTunnelId === row.id ? ' class="selected-row"' : ''}>
            <td><strong>${esc(row.name || row.id)}</strong><div class="muted tiny mono">${esc(row.id || '—')}</div></td>
            <td>${esc(fullDomainForTunnel(row))}</td>
            <td>${esc(row.node_id || '—')}</td>
            <td>${esc([row.protocol || '—', row.local_port ? row.local_port : '—'].join(' / '))}<div class="muted tiny">${esc(tunnelSummary(row))}</div></td>
            <td>${esc([row.local_ip, row.remote_port ? row.remote_port : ''].filter(Boolean).join(':') || '—')}</td>
            <td>${statusBadge(row.actual_state || row.desired_state || '—')}</td>
          </tr>
        `).join('')}
      </tbody>
    </table>
  `;
}

function renderWebsiteTable(rows) {
  const data = safeArray(rows);
  if (!data.length) return '<div class="placeholder">暂无网站映射。</div>';
  return `
    <table>
      <thead>
        <tr>
          <th>主域名</th><th>别名</th><th>来源</th><th>节点 / 隧道</th><th>代理</th><th>状态</th><th>更新时间</th>
        </tr>
      </thead>
      <tbody>
        ${data.map((row) => `
          <tr${STATE.activeWebsiteId === row.id ? ' class="selected-row"' : ''}>
            <td><strong>${esc(row.primary_domain || '—')}</strong><div class="muted tiny mono">${esc(row.id || '—')}</div></td>
            <td>${esc(safeArray(row.domains).join(', ') || '—')}</td>
            <td>${esc(row.source_kind || '—')}</td>
            <td>${esc([row.node_id, row.tunnel_id].filter(Boolean).join(' / ') || '—')}</td>
            <td>${esc(row.proxy_target || '—')}</td>
            <td>${statusBadge(row.status)}</td>
            <td>${esc(fmtTime(row.updated_at || row.last_synced_at || row.created_at))}</td>
          </tr>
        `).join('')}
      </tbody>
    </table>
  `;
}

function renderDnsTable(websites, tunnels) {
  const data = safeArray(websites);
  if (!data.length) return '<div class="placeholder">暂无 DNS 相关映射。</div>';
  return `
    <table>
      <thead>
        <tr>
          <th>主域名</th><th>关联域名</th><th>隧道</th><th>代理</th><th>HTTPS</th><th>状态</th>
        </tr>
      </thead>
      <tbody>
        ${data.map((row) => {
          const tunnel = tunnels.find((t) => t.id === row.tunnel_id);
          return `
            <tr>
              <td><strong>${esc(row.primary_domain || '—')}</strong></td>
              <td>${esc(safeArray(row.domains).join(', ') || '—')}</td>
              <td>${esc(tunnel ? tunnel.name : row.tunnel_id || '—')}</td>
              <td>${esc(row.proxy_target || '—')}</td>
              <td>${truthyBadge(!!row.https_enabled, '启用', '关闭')}</td>
              <td>${statusBadge(row.status)}</td>
            </tr>
          `;
        }).join('')}
      </tbody>
    </table>
  `;
}

function renderDomainTable(domains, websites) {
  if (!domains.length) return '<div class="placeholder">暂无域名。</div>';
  return `
    <table>
      <thead>
        <tr>
          <th>域名</th><th>关联站点</th><th>来源</th><th>证书</th><th>代理</th><th>状态</th>
        </tr>
      </thead>
      <tbody>
        ${domains.map((domain) => {
          const matched = websites.filter((w) => [w.primary_domain, ...(Array.isArray(w.domains) ? w.domains : [])].includes(domain));
          const first = matched[0];
          return `
            <tr>
              <td><strong>${esc(domain)}</strong></td>
              <td>${esc(matched.length ? matched.map((m) => m.website_alias || m.id).join(', ') : '—')}</td>
              <td>${esc(first?.source_kind || '—')}</td>
              <td>${esc(first?.certificate_mode || '—')}</td>
              <td>${truthyBadge(!!first?.proxy_enabled, '启用', '关闭')}</td>
              <td>${statusBadge(first?.status || '—')}</td>
            </tr>
          `;
        }).join('')}
      </tbody>
    </table>
  `;
}

function render() {
  const root = $(APP_ROOT_ID);
  if (!root) return;
  root.innerHTML = appShell();
  wireEvents();
}

function wireEvents() {
  document.querySelectorAll('[data-page]').forEach((btn) => {
    btn.addEventListener('click', () => setPage(btn.dataset.page));
  });
  const refreshBtn = $('refresh-btn');
  if (refreshBtn) refreshBtn.addEventListener('click', loadSnapshot);
  document.querySelectorAll('[data-refresh]').forEach((btn) => {
    btn.addEventListener('click', loadSnapshot);
  });

  const settingsForm = $('settings-form');
  if (settingsForm) {
    settingsForm.addEventListener('submit', async (event) => {
      event.preventDefault();
      const raw = formToObject(event.currentTarget);
      const payload = {
        general: {
          default_log_lines: Number(raw['general.default_log_lines'] || 0),
          data_retention_days: Number(raw['general.data_retention_days'] || 0),
          default_refresh_mode: String(raw['general.default_refresh_mode'] || ''),
        },
        sync: {
          healthcheck_interval: String(raw['sync.healthcheck_interval'] || ''),
          sync_poll_interval: String(raw['sync.sync_poll_interval'] || ''),
          diff_strategy: String(raw['sync.diff_strategy'] || ''),
        },
        queue: {
          max_attempts: Number(raw['queue.max_attempts'] || 0),
          retry_backoff: String(raw['queue.retry_backoff'] || ''),
          stalled_job_policy: String(raw['queue.stalled_job_policy'] || ''),
        },
        frpc_runtime: {
          frpc_enabled: String(raw['frpc_runtime.frpc_enabled']) === 'true',
          frpc_binary_version: String(raw['frpc_runtime.frpc_binary_version'] || ''),
          frpc_log_level: String(raw['frpc_runtime.frpc_log_level'] || ''),
          frpc_healthcheck_interval: String(raw['frpc_runtime.frpc_healthcheck_interval'] || ''),
          frpc_restart_backoff: String(raw['frpc_runtime.frpc_restart_backoff'] || ''),
          auto_recover_strategy: String(raw['frpc_runtime.auto_recover_strategy'] || ''),
          switch_node_strategy: String(raw['frpc_runtime.switch_node_strategy'] || ''),
        },
        integrations: {
          chmlfrp: {
            username: String(raw['integrations.chmlfrp.username'] || ''),
            ...(raw['integrations.chmlfrp.password'] ? { password: String(raw['integrations.chmlfrp.password']) } : {}),
          },
          onepanel: {
            base_url: String(raw['integrations.onepanel.base_url'] || ''),
            entrance: String(raw['integrations.onepanel.entrance'] || ''),
            ...(raw['integrations.onepanel.api_token'] ? { api_token: String(raw['integrations.onepanel.api_token']) } : {}),
          },
          cloudflare: {
            zone_name: String(raw['integrations.cloudflare.zone_name'] || ''),
            ...(raw['integrations.cloudflare.api_token'] ? { api_token: String(raw['integrations.cloudflare.api_token']) } : {}),
          },
        },
      };
      await request('/settings', { method: 'PATCH', body: JSON.stringify(payload) });
      await loadSnapshot();
    });
  }

  const websiteForm = $('website-form');
  if (websiteForm) {
    websiteForm.addEventListener('submit', async (event) => {
      event.preventDefault();
      const raw = formToObject(event.currentTarget);
      const domains = String(raw.domains || '').split(',').map((v) => v.trim()).filter(Boolean);
      let httpConfig = {};
      if (raw.http_config) {
        try {
          httpConfig = JSON.parse(String(raw.http_config));
        } catch (err) {
          alert(`HTTP 配置 JSON 无效：${err.message}`);
          return;
        }
      }
      const payload = {
        source_kind: String(raw.source_kind || 'manual'),
        node_id: String(raw.node_id || ''),
        tunnel_id: String(raw.tunnel_id || ''),
        source_external_id: String(raw.source_external_id || ''),
        website_alias: String(raw.website_alias || ''),
        primary_domain: String(raw.primary_domain || ''),
        domains,
        https_enabled: String(raw.https_enabled) === 'true',
        certificate_mode: String(raw.certificate_mode || ''),
        ssl_certificate_ref: String(raw.ssl_certificate_ref || ''),
        proxy_enabled: String(raw.proxy_enabled) === 'true',
        cache_enabled: String(raw.cache_enabled) === 'true',
        proxy_target: String(raw.proxy_target || ''),
        http_config: httpConfig,
        conflict_strategy: String(raw.conflict_strategy || ''),
      };
      const id = String(raw.id || '').trim();
      const method = id ? 'PATCH' : 'POST';
      const path = id ? `/website-mappings/${encodeURIComponent(id)}` : '/website-mappings';
      await request(path, { method, body: JSON.stringify(payload) });
      await loadSnapshot();
    });
  }

  document.querySelectorAll('[data-action="reload-settings-form"]').forEach((btn) => btn.addEventListener('click', () => render()));
  document.querySelectorAll('[data-action="reload-integrations-form"]').forEach((btn) => btn.addEventListener('click', () => render()));
  document.querySelectorAll('[data-action="clear-website-form"]').forEach((btn) => btn.addEventListener('click', () => {
    const form = $('website-form');
    if (form) form.reset();
  }));
}

function setPage(page) {
  if (!page || !PAGE_META[page]) return;
  if (location.hash.replace('#', '') !== page) {
    location.hash = page;
    return;
  }
  STATE.activePage = page;
  render();
  const view = document.querySelector(`[data-view="${page}"]`);
  if (view) view.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

function bindRoutes() {
  window.addEventListener('hashchange', () => {
    const page = location.hash.replace('#', '').trim();
    if (PAGE_META[page]) {
      STATE.activePage = page;
      render();
    }
  });
}

function initPageFromHash() {
  const hash = location.hash.replace('#', '').trim();
  if (PAGE_META[hash]) STATE.activePage = hash;
}

function setup() {
  const root = $(APP_ROOT_ID);
  if (!root) {
    document.body.innerHTML = '<div class="boot-screen"><div class="boot-card"><h1>页面容器缺失</h1><p>找不到 #app，请检查 index.html 是否已加载。</p></div></div>';
    return;
  }
  initPageFromHash();
  bindRoutes();
  render();
  loadSnapshot();
}

document.addEventListener('DOMContentLoaded', setup);
