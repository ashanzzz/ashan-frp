
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
  loginUsername: '',
  loginPassword: '',
};

const PAGE_META = {
  dashboard: { title: 'Dashboard', kicker: 'WORKBENCH', subtitle: 'Overview for DNS, FRP, jobs, nodes, tunnels and website management.' },
  dns: { title: 'DNS', kicker: 'DNS', subtitle: 'DNS and sync status.' },
  domains: { title: 'Domains', kicker: 'DOMAINS', subtitle: 'Primary domains and mappings.' },
  frp: { title: 'FRP', kicker: 'FRP', subtitle: 'FRPC runtime, nodes and tunnels.' },
  jobs: { title: 'Jobs', kicker: 'JOBS', subtitle: 'Job list and activity timeline.' },
  nodes: { title: 'Nodes', kicker: 'NODES', subtitle: 'Node health and source info.' },
  tunnels: { title: 'Tunnels', kicker: 'TUNNELS', subtitle: 'Tunnel status and targets.' },
  websites: { title: 'Websites', kicker: 'WEBSITE', subtitle: 'Website mappings and conflicts.' },
  logs: { title: 'Logs', kicker: 'LOGS', subtitle: 'Events, audits and snapshots.' },
  chmlfrp: { title: 'chmlfrp', kicker: 'CHMLFRP', subtitle: 'chmlfrp related status.' },
  website: { title: 'Website Tunnels', kicker: 'WEBSITE TUNNELS', subtitle: 'Website tunnel overview.' },
  settings: { title: 'Settings', kicker: 'SETTINGS', subtitle: 'Runtime configuration and integrations.' },
};

const NAV_ITEMS = [['dashboard','Dashboard'],['dns','DNS'],['domains','Domains'],['frp','FRP'],['website','Website Tunnels'],['jobs','Jobs'],['nodes','Nodes'],['tunnels','Tunnels'],['websites','Websites'],['logs','Logs'],['settings','Settings']];

function $(id) { return document.getElementById(id); }
function esc(v) { return String(v ?? '').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;').replaceAll("'",'&#39;'); }
function fmt(v) { if (v === null || v === undefined || v === '') return '?'; if (v instanceof Date) return v.toLocaleString(); if (typeof v === 'boolean') return v ? 'Yes' : 'No'; if (typeof v === 'number') return Number.isFinite(v) ? String(v) : '?'; if (typeof v === 'object') return JSON.stringify(v, null, 2); return String(v); }
function fmtTime(v) { if (!v) return '?'; const d = new Date(v); return Number.isNaN(d.getTime()) ? String(v) : d.toLocaleString(); }
function safeArray(v) { return Array.isArray(v) ? v : []; }
function apiUrl(path) { return `${STATE.apiBase}${path.startsWith('/') ? path : `/${path}`}`; }
function statusBadge(status) { const s = String(status || '').toLowerCase(); const cls = ['healthy','online','synced','enabled','succeeded','running','active'].includes(s) ? 'good' : ['degraded','pending','queued','retry_wait','blocked'].includes(s) ? 'warn' : ['offline','error','failed','canceled','archived','disabled','conflict'].includes(s) ? 'bad' : ''; return `<span class="badge ${cls}">${esc(status || '?')}</span>`; }

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
    const [version, health, me] = await Promise.all([
      request('/version').catch(() => ({ data: {} })),
      request('/health').catch(() => ({ data: {} })),
      request('/auth/me').catch((err) => (err?.status === 401 ? null : Promise.reject(err))),
    ]);
    STATE.version = version?.data || null;
    STATE.health = health?.data || null;
    STATE.authMe = me?.data || null;
    STATE.sessionMode = STATE.authMe ? 'authenticated' : 'anonymous';

    if (STATE.authMe) {
      const [dashboard, settings, nodes, tunnels, websites, jobs] = await Promise.all([
        request('/dashboard').catch(() => ({ data: {} })),
        request('/settings').catch(() => ({ data: {} })),
        request('/nodes').catch(() => ({ data: { nodes: [] } })),
        request('/tunnels').catch(() => ({ data: { tunnels: [] } })),
        request('/website-mappings').catch(() => ({ data: { website_mappings: [] } })),
        request('/jobs').catch(() => ({ data: { jobs: [] } })),
      ]);
      STATE.dashboard = dashboard?.data || null;
      STATE.settings = settings?.data || null;
      STATE.nodes = safeArray(nodes?.data?.nodes ?? nodes?.data ?? []);
      STATE.tunnels = safeArray(tunnels?.data?.tunnels ?? tunnels?.data ?? []);
      STATE.websites = safeArray(websites?.data?.website_mappings ?? websites?.data ?? []);
      STATE.jobs = safeArray(jobs?.data?.jobs ?? jobs?.data ?? []);
    } else {
      STATE.dashboard = null;
      STATE.settings = null;
      STATE.nodes = [];
      STATE.tunnels = [];
      STATE.websites = [];
      STATE.jobs = [];
    }
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
function viewHeader(id, actions = '') { const meta = PAGE_META[id] || PAGE_META.dashboard; return `<div class="view-head"><div><div class="eyebrow">${esc(meta.kicker)}</div><h2>${esc(meta.title)}</h2><p>${esc(meta.subtitle)}</p></div><div class="view-actions">${actions}</div></div>`; }
function renderSimplePage(id, label) { return pageCard(id, `${viewHeader(id)}<div class="page-card"><div class="placeholder">${esc(label)} page is wired and ready for detail expansion.</div></div>`); }
function renderJobs() { const jobs = STATE.jobs || []; return pageCard('jobs', `${viewHeader('jobs')}<div class="page-card"><div class="table-wrap"><table class="data-table"><thead><tr><th>Name</th><th>Status</th><th>Updated</th></tr></thead><tbody>${jobs.length ? jobs.map((job) => `<tr><td>${esc(job.name || job.id || '?')}</td><td>${statusBadge(job.status)}</td><td>${fmtTime(job.updated_at || job.created_at)}</td></tr>`).join('') : '<tr><td colspan="3" class="empty">No jobs</td></tr>'}</tbody></table></div></div>`); }
function loginPanel() { return `<div class="page-card"><div class="placeholder">Please sign in to load the full operator console.</div><div class="meta-list"><div class="meta-row"><span>Mode</span><span>${esc(STATE.sessionMode === 'anonymous' ? 'anonymous' : 'authenticated')}</span></div><div class="meta-row"><span>API Base</span><span>${esc(STATE.apiBase)}</span></div><div class="meta-row"><span>UI Base</span><span>${esc(STATE.uiBase)}</span></div></div><div class="grid" style="margin-top:12px"><input id="login-username" placeholder="Username" value="${esc(STATE.loginUsername)}" /><input id="login-password" type="password" placeholder="Password" value="${esc(STATE.loginPassword)}" /><button id="login-btn">Sign in</button><button class="secondary" id="reload-btn">Reload status</button></div></div>`; }
function appShell() {
  const auth = STATE.authMe;
  const loginText = auth ? `Logged in as ${esc(auth.display_name || auth.login_name || auth.id || 'unknown')}` : STATE.sessionMode === 'anonymous' ? 'Anonymous / read-only' : 'Login state unknown';
  const rightState = `<div class="toolbar"><span class="badge fresh">Version ${esc(STATE.version?.version || '?')}</span><span class="badge ${STATE.health?.status === 'healthy' ? 'good' : 'warn'}">Health ${esc(STATE.health?.status || '?')}</span><span class="badge ${STATE.loading ? 'warn' : 'good'}">${STATE.loading ? 'Loading?' : 'Ready'}</span></div>`;
  const loggedInLayout = `<div class="layout"><aside class="sidebar"><div class="nav-group"><div class="nav-group-title">Pages</div><div class="nav-list">${renderNav()}</div></div></aside><main class="content"><div class="view-stack">${renderSimplePage('dashboard','Dashboard')} ${renderSimplePage('dns','DNS')} ${renderSimplePage('domains','Domains')} ${renderSimplePage('frp','FRP')} ${renderSimplePage('website','Website tunnels')} ${renderJobs()} ${renderSimplePage('nodes','Nodes')} ${renderSimplePage('tunnels','Tunnels')} ${renderSimplePage('websites','Websites')} ${renderSimplePage('logs','Logs')} ${renderSimplePage('settings','Settings')}</div></main></div>`;
  const anonymousLayout = `<div class="layout"><main class="content"><div class="view-stack">${pageCard('dashboard', `${viewHeader('dashboard')}<div class="page-card">${loginPanel()}</div>`)} ${renderSimplePage('dns','DNS')} ${renderSimplePage('domains','Domains')} ${renderSimplePage('frp','FRP')} ${renderSimplePage('website','Website tunnels')} ${renderSimplePage('jobs','Jobs')} ${renderSimplePage('nodes','Nodes')} ${renderSimplePage('tunnels','Tunnels')} ${renderSimplePage('websites','Websites')} ${renderSimplePage('logs','Logs')} ${renderSimplePage('settings','Settings')}</div></main></div>`;
  const content = auth ? loggedInLayout : anonymousLayout;
  return `<div class="shell"><header class="hero"><div class="hero-left"><div class="eyebrow">Ashan FRP Console</div><h1 class="title">${esc((PAGE_META[STATE.activePage] || PAGE_META.dashboard).title)}</h1><p class="subtitle">${esc((PAGE_META[STATE.activePage] || PAGE_META.dashboard).subtitle)}</p><div class="section-gap login-state ${auth ? 'good' : STATE.sessionMode === 'anonymous' ? 'warn' : 'bad'}"><span class="dot"></span><div class="text"><strong>${esc(loginText)}</strong><span>API Base: ${esc(STATE.apiBase)} ? UI Base: ${esc(STATE.uiBase)}</span></div></div>${STATE.error ? `<div class="error-box" style="display:block">${esc(STATE.error)}</div>` : '<div class="error-box"></div>'}</div>${rightState}</header>${content}</div>`;
}

function render() {
  const root = $(APP_ROOT_ID);
  if (!root) return;
  root.innerHTML = appShell();
  document.querySelectorAll('[data-page]').forEach((btn) => btn.addEventListener('click', () => { STATE.activePage = btn.dataset.page; render(); }));
  const refresh = $('reload-btn'); if (refresh) refresh.addEventListener('click', loadSnapshot);
  const loginBtn = $('login-btn');
  if (loginBtn) loginBtn.addEventListener('click', async () => {
    const username = $('login-username')?.value || '';
    const password = $('login-password')?.value || '';
    STATE.loginUsername = username;
    STATE.loginPassword = password;
    try {
      const res = await request('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) });
      if (res?.data?.auth?.token) document.cookie = `ashan_frp_session=${res.data.auth.token}; path=/`;
      await loadSnapshot();
    } catch (err) {
      STATE.error = err?.message || String(err);
      render();
    }
  });
}
function bootHtml(message) { return `<div class="boot-screen"><div class="boot-card"><div class="eyebrow">Ashan FRP</div><h1>${esc(message || 'Loading?')}</h1><p>Initializing UI and backend data.</p></div></div>`; }
let setupDone = false;
function setup() { const root = $(APP_ROOT_ID); if (!root) { document.body.innerHTML = bootHtml('Missing page container'); return; } if (setupDone) return; setupDone = true; root.innerHTML = bootHtml('Loading?'); render(); loadSnapshot(); }
document.addEventListener('DOMContentLoaded', setup);
