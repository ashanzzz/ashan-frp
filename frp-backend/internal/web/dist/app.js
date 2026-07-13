
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
  error: '',
  loading: false,
  lastLoadedAt: null,
  sessionMode: 'unknown',
};

const PAGE_META = {
  dashboard: { title: '????', kicker: 'WORKBENCH', subtitle: '????? DNS?FRP???????????????' },
  dns: { title: 'DNS ??', kicker: 'DNS', subtitle: '?? DNS ??????????' },
  domains: { title: '??', kicker: 'DOMAINS', subtitle: '???????????????' },
  frp: { title: 'FRP ??', kicker: 'FRP', subtitle: '?? FRPC ??????????' },
  jobs: { title: '????', kicker: 'JOBS', subtitle: '?????????????' },
  nodes: { title: '????', kicker: 'NODES', subtitle: '?????????????' },
  tunnels: { title: '????', kicker: 'TUNNELS', subtitle: '??????????????' },
  websites: { title: '????', kicker: 'WEBSITE', subtitle: '????????????' },
  logs: { title: '????', kicker: 'LOGS', subtitle: '???????????' },
  chmlfrp: { title: 'chmlfrp', kicker: 'CHMLFRP', subtitle: '?? chmlfrp ?????' },
  website: { title: '??????', kicker: 'WEBSITE TUNNELS', subtitle: '?????????????' },
  settings: { title: '????', kicker: 'SETTINGS', subtitle: '????????????' },
};

const NAV_ITEMS = [
  ['dashboard', '????'], ['dns', 'DNS ??'], ['domains', '??'], ['frp', 'FRP ??'],
  ['website', '??????'], ['jobs', '????'], ['nodes', '????'], ['tunnels', '????'],
  ['websites', '????'], ['logs', '????'], ['settings', '????'],
];

function $(id) { return document.getElementById(id); }
function esc(value) { return String(value ?? '').replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;').replaceAll("'", '&#39;'); }
function fmt(value) { if (value === null || value === undefined || value === '') return '?'; if (value instanceof Date) return value.toLocaleString(); if (typeof value === 'boolean') return value ? '?' : '?'; if (typeof value === 'number') return Number.isFinite(value) ? String(value) : '?'; if (typeof value === 'object') return JSON.stringify(value, null, 2); return String(value); }
function fmtTime(value) { if (!value) return '?'; const d = new Date(value); return Number.isNaN(d.getTime()) ? String(value) : d.toLocaleString(); }
function statusBadge(status) { const s = String(status || '').toLowerCase(); const cls = ['healthy','online','synced','enabled','succeeded','running','active'].includes(s) ? 'good' : ['degraded','pending','queued','retry_wait','blocked'].includes(s) ? 'warn' : ['offline','error','failed','canceled','archived','disabled','conflict'].includes(s) ? 'bad' : ''; return `<span class="badge ${cls}">${esc(status || '?')}</span>`; }
function truthyBadge(value, yes = '???', no = '???') { return `<span class="badge ${value ? 'good' : 'warn'}">${value ? yes : no}</span>`; }
function safeArray(value) { return Array.isArray(value) ? value : []; }
function apiUrl(path) { return `${STATE.apiBase}${path.startsWith('/') ? path : `/${path}`}`; }

async function request(path, options = {}) {
  const response = await fetch(apiUrl(path), { credentials: 'same-origin', headers: { 'Content-Type': 'application/json', ...(options.headers || {}) }, ...options });
  const text = await response.text();
  let parsed = null;
  try { parsed = text ? JSON.parse(text) : null; } catch { parsed = text; }
  if (!response.ok) {
    const message = parsed?.error?.message || (typeof parsed === 'string' ? parsed : `HTTP ${response.status}`);
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
    const me = await request('/auth/me').catch((err) => err?.status === 401 ? null : Promise.reject(err));
    STATE.authMe = me?.data || null;
    STATE.sessionMode = STATE.authMe ? 'authenticated' : 'anonymous';
    STATE.lastLoadedAt = new Date();
  } catch (err) {
    STATE.error = err?.message || String(err);
  } finally {
    STATE.loading = false;
    render();
  }
}

function renderNav() {
  return NAV_ITEMS.map(([id, title]) => `<button class="nav-item ${STATE.activePage === id ? 'active' : ''}" data-page="${id}">${esc(title)}</button>`).join('');
}

function pageCard(id, body) { return `<section class="view ${STATE.activePage === id ? 'active' : ''}" data-view="${id}">${body}</section>`; }

function viewHeader(id, actions = '') {
  const meta = PAGE_META[id] || PAGE_META.dashboard;
  return `<div class="view-head"><div><div class="eyebrow">${esc(meta.kicker)}</div><h2>${esc(meta.title)}</h2><p>${esc(meta.subtitle)}</p></div><div class="view-actions">${actions}</div></div>`;
}

function renderDashboard() {
  const version = STATE.version || {};
  const health = STATE.health || {};
  const dashboard = STATE.dashboard || {};
  return pageCard('dashboard', `
    ${viewHeader('dashboard', '<button class="secondary" id="refresh-btn">??</button>')}
    <div class="page-card">
      <div class="kpi-grid">
        <div class="kpi"><div class="kpi-label">????</div><div class="kpi-value">${esc(health.status || '?')}</div></div>
        <div class="kpi"><div class="kpi-label">?? / ?? / ??</div><div class="kpi-value">${(STATE.nodes || []).length} / ${(STATE.tunnels || []).length} / ${(STATE.websites || []).length}</div></div>
        <div class="kpi"><div class="kpi-label">???</div><div class="kpi-value">${(STATE.jobs || []).length}</div></div>
      </div>
    </div>
    <div class="page-grid">
      <div class="panel"><h3>??</h3><div class="meta-list"><div class="meta-row"><span>??</span><span>${esc(version.version || '?')}</span></div><div class="meta-row"><span>API Base</span><span>${esc(version.api_base || STATE.apiBase)}</span></div><div class="meta-row"><span>UI Base</span><span>${esc(version.ui_base || STATE.uiBase)}</span></div></div></div>
      <div class="panel"><h3>????</h3><div class="meta-list"><div class="meta-row"><span>FRP</span><span>${esc(health.tunnels ?? '?')}</span></div><div class="meta-row"><span>??</span><span>${esc(health.jobs ?? '?')}</span></div><div class="meta-row"><span>????</span><span>${fmtTime(STATE.lastLoadedAt)}</span></div></div></div>
      <div class="panel span-2"><h3>???? / ??</h3><div class="placeholder">???????????????????????????</div></div>
    </div>
  `);
}

function renderSimplePage(id, label) { return pageCard(id, `${viewHeader(id)}<div class="page-card"><div class="placeholder">${esc(label)}??????????????????</div></div>`); }

function renderJobs() {
  const jobs = STATE.jobs || [];
  return pageCard('jobs', `
    ${viewHeader('jobs', '<button class="secondary" id="refresh-btn">??</button>')}
    <div class="page-card">
      <div class="table-wrap">
        <table class="data-table"><thead><tr><th>??</th><th>??</th><th>????</th></tr></thead><tbody>${jobs.length ? jobs.map((job) => `<tr><td>${esc(job.name || job.id || '?')}</td><td>${statusBadge(job.status)}</td><td>${fmtTime(job.updated_at || job.created_at)}</td></tr>`).join('') : '<tr><td colspan="3" class="empty">????</td></tr>'}</tbody></table>
      </div>
    </div>
  `);
}

function appShell() {
  const auth = STATE.authMe;
  const loginText = auth ? `??? ? ${esc(auth.display_name || auth.login_name || auth.id || 'unknown')}` : STATE.sessionMode === 'anonymous' ? '??? / ????' : '??????';
  return `
    <div class="shell">
      <header class="hero">
        <div class="hero-left">
          <div class="eyebrow">Ashan FRP ???</div>
          <h1 class="title">${esc((PAGE_META[STATE.activePage] || PAGE_META.dashboard).title)}</h1>
          <p class="subtitle">${esc((PAGE_META[STATE.activePage] || PAGE_META.dashboard).subtitle)}</p>
          <div class="section-gap login-state ${auth ? 'good' : STATE.sessionMode === 'anonymous' ? 'warn' : 'bad'}"><span class="dot"></span><div class="text"><strong>${esc(loginText)}</strong><span>API Base: ${esc(STATE.apiBase)} ? UI Base: ${esc(STATE.uiBase)}</span></div></div>
          ${STATE.error ? `<div class="error-box" style="display:block">${esc(STATE.error)}</div>` : '<div class="error-box"></div>'}
        </div>
        <div class="toolbar">
          <span class="badge fresh">?? ${esc(STATE.version?.version || '?')}</span>
          <span class="badge ${STATE.health?.status === 'healthy' ? 'good' : 'warn'}">?? ${esc(STATE.health?.status || '?')}</span>
          <span class="badge ${STATE.loading ? 'warn' : 'good'}">${STATE.loading ? '????' : '???'}</span>
        </div>
      </header>
      <div class="layout">
        <aside class="sidebar"><div class="nav-group"><div class="nav-group-title">???</div><div class="nav-list">${renderNav()}</div></div></aside>
        <main class="content"><div class="view-stack">${renderDashboard()}${renderSimplePage('dns', 'DNS')}${renderSimplePage('domains', '??')}${renderSimplePage('frp', 'FRP')}${renderSimplePage('website', '????')}${renderJobs()}${renderSimplePage('nodes', '??')}${renderSimplePage('tunnels', '??')}${renderSimplePage('websites', '????')}${renderSimplePage('logs', '??')}${renderSimplePage('settings', '??')}</div></main>
      </div>
    </div>
  `;
}

function render() {
  const root = $(APP_ROOT_ID);
  if (!root) return;
  root.innerHTML = appShell();
  document.querySelectorAll('[data-page]').forEach((btn) => btn.addEventListener('click', () => { STATE.activePage = btn.dataset.page; render(); }));
  const refresh = $('refresh-btn');
  if (refresh) refresh.addEventListener('click', loadSnapshot);
}

function bootHtml(message) { return `<div class="boot-screen"><div class="boot-card"><div class="eyebrow">Ashan FRP</div><h1>${esc(message || '????')}</h1><p>?????????????</p></div></div>`; }

let setupDone = false;
function setup() {
  const root = $(APP_ROOT_ID);
  if (!root) { document.body.innerHTML = bootHtml('??????'); return; }
  if (setupDone) return;
  setupDone = true;
  root.innerHTML = bootHtml('?????');
  render();
  loadSnapshot();
}

document.addEventListener('DOMContentLoaded', setup);
