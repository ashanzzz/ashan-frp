
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
  selectedJob: null,
  selectedJobEvents: [],
  authMe: null,
  activePage: 'dashboard',
  error: '',
  loading: false,
  lastLoadedAt: null,
  sessionMode: 'unknown',
  loginUsername: '',
  loginPassword: '',
  recoveryOpen: false,
  recoveryCopyStatus: '',
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
const RECOVERY_COMMANDS = {
  local: './ashan-frp admin list\n./ashan-frp admin reset-password --username admin',
  docker: 'docker compose exec ashan-frp /app/ashan-frp admin list\ndocker compose exec ashan-frp /app/ashan-frp admin reset-password --username admin',
};

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
function renderJobs() {
  const jobs = STATE.jobs || [];
  const selectedJob = STATE.selectedJob;
  const selectedEvents = STATE.selectedJobEvents || [];
  const jobTable = `<div class="panel"><h2>Jobs <small>${jobs.length} items</small></h2><div class="table-wrap">${jobs.length ? `<table><thead><tr><th>ID</th><th>Status</th><th>Target</th><th>Progress</th><th>Updated</th></tr></thead><tbody>${jobs.map((job) => `<tr class="${selectedJob?.id === job.id ? 'selected-row' : ''}" data-job-id="${esc(job.id)}"><td class="mono">${esc(job.id)}</td><td>${statusBadge(job.status)}</td><td>${esc(job.target_type || '-')} ${esc(job.target_id || '-')}</td><td>${esc(job.progress || 0)}%</td><td>${esc(fmtTime(job.updated_at || job.created_at))}</td></tr>`).join('')}</tbody></table>` : '<div class="placeholder">No jobs yet.</div>'}</div></div>`;
  const jobDetail = selectedJob ? `<div class="detail-card"><div class="detail-head"><div><div class="detail-kicker">JOB DETAIL</div><h3>${esc(selectedJob.name || selectedJob.id)}</h3><div class="muted tiny">${esc(selectedJob.target_type || '-')} ? ${esc(selectedJob.target_id || '-')}</div></div><div>${statusBadge(selectedJob.status)}</div></div><div class="meta-list"><div class="meta-row"><span>ID</span><span class="mono">${esc(selectedJob.id)}</span></div><div class="meta-row"><span>Progress</span><span>${esc(selectedJob.progress || 0)}%</span></div><div class="meta-row"><span>Attempt</span><span>${esc(selectedJob.attempt_count || 0)} / ${esc(selectedJob.max_attempts || 0)}</span></div><div class="meta-row"><span>Created</span><span>${esc(fmtTime(selectedJob.created_at))}</span></div><div class="meta-row"><span>Updated</span><span>${esc(fmtTime(selectedJob.updated_at))}</span></div><div class="meta-row"><span>Result</span><span class="mono">${esc(selectedJob.result_json || '-')}</span></div><div class="meta-row"><span>Error</span><span class="mono">${esc(selectedJob.error_message || '-')}</span></div></div></div>` : '<div class="placeholder">Select a job to inspect details and timeline.</div>';
  const timeline = `<div class="detail-card small-box"><div class="detail-head"><div><div class="detail-kicker">TIMELINE</div><h3>Job events</h3></div><button class="secondary tiny-btn" id="job-refresh-btn">Refresh</button></div><div class="event-log">${selectedEvents.length ? selectedEvents.map((evt) => `<div class="event-item"><div class="head"><span class="kind">${esc(evt.kind || evt.event_type || 'event')}</span><span class="muted mono">${esc(fmtTime(evt.created_at))}</span></div><div class="tiny muted">${esc(evt.message || evt.level || '')}</div><div class="tiny mono">${esc(JSON.stringify(evt.payload || evt.payload_json || {}, null, 2))}</div></div>`).join('') : '<div class="placeholder">No events yet.</div>'}</div></div>`;
  return pageCard('jobs', `<div class="grid"><div class="panel">${jobTable}</div><div class="split-grid"><div class="panel">${jobDetail}</div><div class="panel">${timeline}</div></div></div>`);
}

function renderSimplePage(id, label) { return pageCard(id, `${viewHeader(id)}<div class="page-card"><div class="placeholder">${esc(label)} page is wired and ready for detail expansion.</div></div>`); }
function renderJobs() {
  const jobs = STATE.jobs || [];
  const selectedJob = STATE.selectedJob;
  const selectedEvents = STATE.selectedJobEvents || [];
  const jobTable = `<div class="panel"><h2>Jobs <small>${jobs.length} items</small></h2><div class="table-wrap">${jobs.length ? `<table><thead><tr><th>ID</th><th>Status</th><th>Target</th><th>Progress</th><th>Updated</th></tr></thead><tbody>${jobs.map((job) => `<tr class="${selectedJob?.id === job.id ? 'selected-row' : ''}" data-job-id="${esc(job.id)}"><td class="mono">${esc(job.id)}</td><td>${statusBadge(job.status)}</td><td>${esc(job.target_type || '-')} ${esc(job.target_id || '-')}</td><td>${esc(job.progress || 0)}%</td><td>${esc(fmtTime(job.updated_at || job.created_at))}</td></tr>`).join('')}</tbody></table>` : '<div class="placeholder">No jobs yet.</div>'}</div></div>`;
  const jobDetail = selectedJob ? `<div class="detail-card"><div class="detail-head"><div><div class="detail-kicker">JOB DETAIL</div><h3>${esc(selectedJob.name || selectedJob.id)}</h3><div class="muted tiny">${esc(selectedJob.target_type || '-')} ? ${esc(selectedJob.target_id || '-')}</div></div><div>${statusBadge(selectedJob.status)}</div></div><div class="meta-list"><div class="meta-row"><span>ID</span><span class="mono">${esc(selectedJob.id)}</span></div><div class="meta-row"><span>Progress</span><span>${esc(selectedJob.progress || 0)}%</span></div><div class="meta-row"><span>Attempt</span><span>${esc(selectedJob.attempt_count || 0)} / ${esc(selectedJob.max_attempts || 0)}</span></div><div class="meta-row"><span>Created</span><span>${esc(fmtTime(selectedJob.created_at))}</span></div><div class="meta-row"><span>Updated</span><span>${esc(fmtTime(selectedJob.updated_at))}</span></div><div class="meta-row"><span>Result</span><span class="mono">${esc(selectedJob.result_json || '-')}</span></div><div class="meta-row"><span>Error</span><span class="mono">${esc(selectedJob.error_message || '-')}</span></div></div></div>` : '<div class="placeholder">Select a job to inspect details and timeline.</div>';
  const timeline = `<div class="detail-card small-box"><div class="detail-head"><div><div class="detail-kicker">TIMELINE</div><h3>Job events</h3></div><button class="secondary tiny-btn" id="job-refresh-btn">Refresh</button></div><div class="event-log">${selectedEvents.length ? selectedEvents.map((evt) => `<div class="event-item"><div class="head"><span class="kind">${esc(evt.kind || evt.event_type || 'event')}</span><span class="muted mono">${esc(fmtTime(evt.created_at))}</span></div><div class="tiny muted">${esc(evt.message || evt.level || '')}</div><div class="tiny mono">${esc(JSON.stringify(evt.payload || evt.payload_json || {}, null, 2))}</div></div>`).join('') : '<div class="placeholder">No events yet.</div>'}</div></div>`;
  return pageCard('jobs', `<div class="grid"><div class="panel">${jobTable}</div><div class="split-grid"><div class="panel">${jobDetail}</div><div class="panel">${timeline}</div></div></div>`);
}

function renderSimplePage(id, label) { return pageCard(id, `${viewHeader(id)}<div class="page-card"><div class="placeholder">${esc(label)} page is wired and ready for detail expansion.</div></div>`); }
function renderJobs() {
  const jobs = STATE.jobs || [];
  const selectedJob = STATE.selectedJob;
  const selectedEvents = STATE.selectedJobEvents || [];
  const jobTable = `<div class="panel"><h2>Jobs <small>${jobs.length} items</small></h2><div class="table-wrap">${jobs.length ? `<table><thead><tr><th>ID</th><th>Status</th><th>Target</th><th>Progress</th><th>Updated</th></tr></thead><tbody>${jobs.map((job) => `<tr class="${selectedJob?.id === job.id ? 'selected-row' : ''}" data-job-id="${esc(job.id)}"><td class="mono">${esc(job.id)}</td><td>${statusBadge(job.status)}</td><td>${esc(job.target_type || '-')} ${esc(job.target_id || '-')}</td><td>${esc(job.progress || 0)}%</td><td>${esc(fmtTime(job.updated_at || job.created_at))}</td></tr>`).join('')}</tbody></table>` : '<div class="placeholder">No jobs yet.</div>'}</div></div>`;
  const jobDetail = selectedJob ? `<div class="detail-card"><div class="detail-head"><div><div class="detail-kicker">JOB DETAIL</div><h3>${esc(selectedJob.name || selectedJob.id)}</h3><div class="muted tiny">${esc(selectedJob.target_type || '-')} ? ${esc(selectedJob.target_id || '-')}</div></div><div>${statusBadge(selectedJob.status)}</div></div><div class="meta-list"><div class="meta-row"><span>ID</span><span class="mono">${esc(selectedJob.id)}</span></div><div class="meta-row"><span>Progress</span><span>${esc(selectedJob.progress || 0)}%</span></div><div class="meta-row"><span>Attempt</span><span>${esc(selectedJob.attempt_count || 0)} / ${esc(selectedJob.max_attempts || 0)}</span></div><div class="meta-row"><span>Created</span><span>${esc(fmtTime(selectedJob.created_at))}</span></div><div class="meta-row"><span>Updated</span><span>${esc(fmtTime(selectedJob.updated_at))}</span></div><div class="meta-row"><span>Result</span><span class="mono">${esc(selectedJob.result_json || '-')}</span></div><div class="meta-row"><span>Error</span><span class="mono">${esc(selectedJob.error_message || '-')}</span></div></div></div>` : '<div class="placeholder">Select a job to inspect details and timeline.</div>';
  const timeline = `<div class="detail-card small-box"><div class="detail-head"><div><div class="detail-kicker">TIMELINE</div><h3>Job events</h3></div><button class="secondary tiny-btn" id="job-refresh-btn">Refresh</button></div><div class="event-log">${selectedEvents.length ? selectedEvents.map((evt) => `<div class="event-item"><div class="head"><span class="kind">${esc(evt.kind || evt.event_type || 'event')}</span><span class="muted mono">${esc(fmtTime(evt.created_at))}</span></div><div class="tiny muted">${esc(evt.message || evt.level || '')}</div><div class="tiny mono">${esc(JSON.stringify(evt.payload || evt.payload_json || {}, null, 2))}</div></div>`).join('') : '<div class="placeholder">No events yet.</div>'}</div></div>`;
  return pageCard('jobs', `<div class="grid"><div class="panel">${jobTable}</div><div class="split-grid"><div class="panel">${jobDetail}</div><div class="panel">${timeline}</div></div></div>`);
}
function loginPanel() {
  return `<div class="login-card"><div class="login-intro"><div class="eyebrow">安全登录</div><h3>进入 Ashan FRP 运营台</h3><p>使用管理员账号登录后，系统会按当前账号角色加载可访问的数据和操作入口。</p></div><form id="login-form" class="login-form"><label for="login-username"><span>用户名</span><input id="login-username" name="username" autocomplete="username" placeholder="请输入管理员用户名" value="${esc(STATE.loginUsername)}" /></label><label for="login-password"><span>密码</span><input id="login-password" name="password" type="password" autocomplete="current-password" placeholder="请输入密码" value="${esc(STATE.loginPassword)}" /></label><div class="login-actions"><button id="login-btn" type="submit">登录运营台</button><button class="secondary" type="button" id="reload-btn">刷新服务状态</button></div><div class="login-help"><button class="link-button" type="button" id="forgot-password-btn">忘记密码？</button><span>需要服务器或容器终端权限才能恢复</span></div></form></div>`;
}

function recoveryDialog() {
  if (!STATE.recoveryOpen) return '';
  const status = STATE.recoveryCopyStatus ? `<span class="copy-status" role="status" aria-live="polite">${esc(STATE.recoveryCopyStatus)}</span>` : '';
  return `<div class="recovery-backdrop" id="recovery-backdrop"><section class="recovery-dialog" id="recovery-dialog" role="dialog" aria-modal="true" aria-labelledby="recovery-title" tabindex="-1"><div class="recovery-head"><div><div class="eyebrow">管理员凭据恢复</div><h2 id="recovery-title">忘记用户名或密码</h2></div><button class="icon-button" type="button" id="recovery-close-btn" aria-label="关闭密码恢复说明">×</button></div><div class="recovery-notice"><strong>系统不提供网页密码重置。</strong><span>这是为了避免产生可被外部利用的未认证重置入口。恢复操作只能由拥有服务器或容器终端权限的管理员执行。</span></div><ol class="recovery-steps"><li>先列出管理员账号，确认当前登录用户名。</li><li>执行密码重置命令；如需同时改名，可增加 <code>--new-username 新用户名</code>。</li><li>重置完成后，所有旧 Session 和 API Token 会立即失效。</li></ol><div class="command-group"><div class="command-head"><strong>直接运行二进制</strong><button class="secondary tiny-btn" type="button" data-copy-recovery="local">复制命令</button></div><pre><code>${esc(RECOVERY_COMMANDS.local)}</code></pre></div><div class="command-group"><div class="command-head"><strong>Docker Compose</strong><button class="secondary tiny-btn" type="button" data-copy-recovery="docker">复制命令</button></div><pre><code>${esc(RECOVERY_COMMANDS.docker)}</code></pre></div><div class="recovery-foot">${status}<button type="button" id="recovery-confirm-btn">我知道了</button></div></section></div>`;
}

function focusSoon(id) {
  const schedule = typeof requestAnimationFrame === 'function' ? requestAnimationFrame : (callback) => setTimeout(callback, 0);
  schedule(() => $(id)?.focus());
}

function openRecoveryDialog() {
  STATE.recoveryOpen = true;
  STATE.recoveryCopyStatus = '';
  render();
  focusSoon('recovery-dialog');
}

function closeRecoveryDialog() {
  STATE.recoveryOpen = false;
  STATE.recoveryCopyStatus = '';
  render();
  focusSoon('forgot-password-btn');
}

async function copyRecoveryCommand(kind) {
  const command = RECOVERY_COMMANDS[kind];
  if (!command) return;
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(command);
    } else {
      const textarea = document.createElement('textarea');
      textarea.value = command;
      textarea.setAttribute('readonly', '');
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      textarea.remove();
    }
    STATE.recoveryCopyStatus = '恢复命令已复制';
  } catch {
    STATE.recoveryCopyStatus = '复制失败，请手动选择命令';
  }
  render();
  focusSoon('recovery-dialog');
}

async function submitLogin() {
  const username = $('login-username')?.value.trim() || '';
  const password = $('login-password')?.value || '';
  STATE.loginUsername = username;
  STATE.loginPassword = password;
  if (!username || !password) {
    STATE.error = '请输入用户名和密码';
    render();
    focusSoon(username ? 'login-password' : 'login-username');
    return;
  }
  try {
    const res = await request('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) });
    if (res?.data?.auth?.token) document.cookie = `ashan_frp_session=${res.data.auth.token}; path=/`;
    STATE.loginPassword = '';
    await loadSnapshot();
  } catch (err) {
    STATE.error = err?.message || String(err);
    STATE.loginPassword = '';
    render();
    focusSoon('login-password');
  }
}

function appShell() {
  const auth = STATE.authMe;
  const loginText = auth ? `已登录：${esc(auth.display_name || auth.login_name || auth.id || '未知账号')}` : STATE.sessionMode === 'anonymous' ? '尚未登录' : '正在确认登录状态';
  const rightState = `<div class="toolbar"><span class="badge fresh">版本 ${esc(STATE.version?.version || '?')}</span><span class="badge ${STATE.health?.status === 'healthy' ? 'good' : 'warn'}">服务 ${esc(STATE.health?.status || '?')}</span><span class="badge ${STATE.loading ? 'warn' : 'good'}">${STATE.loading ? '加载中' : '就绪'}</span></div>`;
  const loggedInLayout = `<div class="layout"><aside class="sidebar"><div class="nav-group"><div class="nav-group-title">Pages</div><div class="nav-list">${renderNav()}</div></div></aside><main class="content"><div class="view-stack">${renderSimplePage('dashboard','Dashboard')} ${renderSimplePage('dns','DNS')} ${renderSimplePage('domains','Domains')} ${renderSimplePage('frp','FRP')} ${renderSimplePage('website','Website tunnels')} ${renderJobs()} ${renderSimplePage('nodes','Nodes')} ${renderSimplePage('tunnels','Tunnels')} ${renderSimplePage('websites','Websites')} ${renderSimplePage('logs','Logs')} ${renderSimplePage('settings','Settings')}</div></main></div>`;
  const anonymousLayout = `<div class="layout anonymous-layout"><main class="content"><div class="view-stack">${pageCard('dashboard', `${viewHeader('dashboard')}<div class="page-card">${loginPanel()}</div>`)} ${renderSimplePage('dns','DNS')} ${renderSimplePage('domains','Domains')} ${renderSimplePage('frp','FRP')} ${renderSimplePage('website','Website tunnels')} ${renderSimplePage('jobs','Jobs')} ${renderSimplePage('nodes','Nodes')} ${renderSimplePage('tunnels','Tunnels')} ${renderSimplePage('websites','Websites')} ${renderSimplePage('logs','Logs')} ${renderSimplePage('settings','Settings')}</div></main></div>`;
  const content = auth ? loggedInLayout : anonymousLayout;
  return `<div class="shell"><header class="hero"><div class="hero-left"><div class="eyebrow">Ashan FRP Console</div><h1 class="title">${esc((PAGE_META[STATE.activePage] || PAGE_META.dashboard).title)}</h1><p class="subtitle">${esc((PAGE_META[STATE.activePage] || PAGE_META.dashboard).subtitle)}</p><div class="section-gap login-state ${auth ? 'good' : STATE.sessionMode === 'anonymous' ? 'warn' : 'bad'}"><span class="dot"></span><div class="text"><strong>${esc(loginText)}</strong><span>API：${esc(STATE.apiBase)} · UI：${esc(STATE.uiBase)}</span></div></div>${STATE.error ? `<div class="error-box" style="display:block">${esc(STATE.error)}</div>` : '<div class="error-box"></div>'}</div>${rightState}</header>${content}</div>${recoveryDialog()}`;
}

async function loadSelectedJob(jobId) {
  if (!jobId) return;
  try {
    const res = await request(`/jobs/${encodeURIComponent(jobId)}`);
    STATE.selectedJob = res?.data?.job || STATE.selectedJob || null;
    STATE.selectedJobEvents = res?.data?.events || [];
  } catch (err) {
    STATE.error = STATE.error || `Job detail load failed: ${err?.message || err}`;
    STATE.selectedJobEvents = [];
  }
  render();
}

function render() {
  const root = $(APP_ROOT_ID);
  if (!root) return;
  root.innerHTML = appShell();
  document.querySelectorAll('[data-page]').forEach((btn) => btn.addEventListener('click', () => { STATE.activePage = btn.dataset.page; render(); }));
  document.querySelectorAll('[data-job-id]').forEach((row) => row.addEventListener('click', () => loadSelectedJob(row.dataset.jobId)));
  const refresh = $('reload-btn'); if (refresh) refresh.addEventListener('click', loadSnapshot);
  const jobRefresh = $('job-refresh-btn'); if (jobRefresh && STATE.selectedJob?.id) jobRefresh.addEventListener('click', () => loadSelectedJob(STATE.selectedJob.id));
  const loginForm = $('login-form'); if (loginForm) loginForm.addEventListener('submit', (event) => { event.preventDefault(); submitLogin(); });
  const forgotPassword = $('forgot-password-btn'); if (forgotPassword) forgotPassword.addEventListener('click', openRecoveryDialog);
  const recoveryClose = $('recovery-close-btn'); if (recoveryClose) recoveryClose.addEventListener('click', closeRecoveryDialog);
  const recoveryConfirm = $('recovery-confirm-btn'); if (recoveryConfirm) recoveryConfirm.addEventListener('click', closeRecoveryDialog);
  const recoveryBackdrop = $('recovery-backdrop'); if (recoveryBackdrop) recoveryBackdrop.addEventListener('click', (event) => { if (event.target === recoveryBackdrop) closeRecoveryDialog(); });
  document.querySelectorAll('[data-copy-recovery]').forEach((button) => button.addEventListener('click', () => copyRecoveryCommand(button.dataset.copyRecovery)));
}
function bootHtml(message) { return `<div class="boot-screen"><div class="boot-card"><div class="eyebrow">Ashan FRP</div><h1>${esc(message || 'Loading?')}</h1><p>Initializing UI and backend data.</p></div></div>`; }
let setupDone = false;
function setup() { const root = $(APP_ROOT_ID); if (!root) { document.body.innerHTML = bootHtml('Missing page container'); return; } if (setupDone) return; setupDone = true; document.addEventListener('keydown', (event) => { if (event.key === 'Escape' && STATE.recoveryOpen) closeRecoveryDialog(); }); root.innerHTML = bootHtml('Loading?'); render(); loadSnapshot(); }
document.addEventListener('DOMContentLoaded', setup);
