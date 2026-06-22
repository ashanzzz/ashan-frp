(function () {
  const API = '/api/v1';
  const state = {
    version: null,
    health: null,
    nodes: [],
    tunnels: [],
    websites: [],
    jobs: [],
    runtime: null,
    settings: null,
    events: [],
  };

  const refs = {
    errorBox: document.getElementById('errorBox'),
    streamBadge: document.getElementById('streamBadge'),
    refreshBtn: document.getElementById('refreshBtn'),
    docsBtn: document.getElementById('docsBtn'),
    openApiBtn: document.getElementById('openApiBtn'),
    lastRefresh: document.getElementById('lastRefresh'),
    versionValue: document.getElementById('versionValue'),
    versionDesc: document.getElementById('versionDesc'),
    healthValue: document.getElementById('healthValue'),
    healthDesc: document.getElementById('healthDesc'),
    runtimeValue: document.getElementById('runtimeValue'),
    runtimeDesc: document.getElementById('runtimeDesc'),
    eventCount: document.getElementById('eventCount'),
    versionMeta: document.getElementById('versionMeta'),
    healthMeta: document.getElementById('healthMeta'),
    runtimeMeta: document.getElementById('runtimeMeta'),
    nodesBody: document.getElementById('nodesBody'),
    tunnelsBody: document.getElementById('tunnelsBody'),
    websitesBody: document.getElementById('websitesBody'),
    jobsBody: document.getElementById('jobsBody'),
    settingsBody: document.getElementById('settingsBody'),
    eventsBody: document.getElementById('eventsBody'),
    nodeForm: document.getElementById('nodeForm'),
    nodeId: document.getElementById('nodeId'),
    nodeDisplayName: document.getElementById('nodeDisplayName'),
    nodeProvider: document.getElementById('nodeProvider'),
    nodeType: document.getElementById('nodeType'),
    nodeEndpointURL: document.getElementById('nodeEndpointURL'),
    nodeRegion: document.getElementById('nodeRegion'),
    nodeStatus: document.getElementById('nodeStatus'),
    nodeCanonicalName: document.getElementById('nodeCanonicalName'),
    nodeMetadata: document.getElementById('nodeMetadata'),
    nodeSubmitBtn: document.getElementById('nodeSubmitBtn'),
    nodeClearBtn: document.getElementById('nodeClearBtn'),
    chmlfrpUsername: document.getElementById('chmlfrpUsername'),
    chmlfrpPassword: document.getElementById('chmlfrpPassword'),
    onepanelBaseURL: document.getElementById('onepanelBaseURL'),
    onepanelEntrance: document.getElementById('onepanelEntrance'),
    onepanelAPIToken: document.getElementById('onepanelAPIToken'),
    cloudflareAPIToken: document.getElementById('cloudflareAPIToken'),
    cloudflareZoneID: document.getElementById('cloudflareZoneID'),
    integrationsSummary: document.getElementById('integrationsSummary'),
    nodeCountBadge: document.getElementById('nodeCountBadge'),
    tokenNote: document.getElementById('tokenNote'),
  };

  let stream = null;
  let refreshTimer = null;
  let refreshInFlight = false;
  let nodeEditID = '';
  let dirtySettings = false;

  function escapeHTML(value) {
    return String(value ?? '')
      .replaceAll('&', '&amp;')
      .replaceAll('<', '&lt;')
      .replaceAll('>', '&gt;')
      .replaceAll('"', '&quot;')
      .replaceAll("'", '&#39;');
  }

  function formatTime(value) {
    if (!value) return '—';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return escapeHTML(value);
    return date.toLocaleString();
  }

  function setError(message) {
    if (!message) {
      refs.errorBox.style.display = 'none';
      refs.errorBox.textContent = '';
      return;
    }
    refs.errorBox.style.display = 'block';
    refs.errorBox.textContent = message;
  }

  function badgeClass(value) {
    const text = String(value ?? '').toLowerCase();
    if (['healthy', 'running', 'online', 'enabled', 'synced', 'success', 'active'].includes(text)) return 'good';
    if (['degraded', 'pending', 'queued', 'retry_wait', 'stopped', 'warning'].includes(text)) return 'warn';
    if (['down', 'offline', 'failed', 'blocked', 'error', 'banned', 'disabled', 'archived'].includes(text)) return 'bad';
    return '';
  }

  function badgeHTML(value) {
    const text = escapeHTML(value ?? '—');
    const cls = badgeClass(value);
    return `<span class="badge ${cls}">${text}</span>`;
  }

  function renderMeta(target, rows) {
    target.innerHTML = rows.map(([label, value]) => `
      <div class="meta-row">
        <span>${escapeHTML(label)}</span>
        <span class="mono">${value}</span>
      </div>
    `).join('') || '<div class="muted tiny">暂无数据</div>';
  }

  function jsonPretty(value) {
    return JSON.stringify(value, null, 2);
  }

  function renderNodes() {
    refs.nodeCountBadge.textContent = `${state.nodes.length} nodes`;
    refs.nodesBody.innerHTML = state.nodes.length ? state.nodes.map((node) => `
      <tr>
        <td>${escapeHTML(node.display_name ?? node.name ?? node.id ?? '—')}</td>
        <td>${escapeHTML(node.provider ?? '—')}</td>
        <td>${escapeHTML(node.node_type ?? '—')}</td>
        <td>${badgeHTML(node.status)}</td>
        <td>${badgeHTML(node.health_status)}</td>
        <td>${escapeHTML(node.region ?? '—')}</td>
        <td class="mono">${escapeHTML(node.endpoint_url ?? '—')}</td>
        <td>
          <button class="secondary tiny-btn" data-action="edit-node" data-id="${escapeHTML(node.id)}">编辑</button>
          <button class="secondary tiny-btn" data-action="check-node" data-id="${escapeHTML(node.id)}">检查</button>
          <button class="secondary tiny-btn" data-action="archive-node" data-id="${escapeHTML(node.id)}">归档</button>
        </td>
      </tr>
    `).join('') : '<tr><td colspan="8" class="muted">暂无节点</td></tr>';
  }

  function renderTunnels() {
    refs.tunnelsBody.innerHTML = state.tunnels.length ? state.tunnels.map((tunnel) => `
      <tr>
        <td>${escapeHTML(tunnel.name ?? tunnel.id ?? '—')}</td>
        <td class="mono">${escapeHTML(tunnel.node_id ?? '—')}</td>
        <td>${escapeHTML(tunnel.tunnel_type ?? '—')}</td>
        <td>${badgeHTML(tunnel.desired_state)}</td>
        <td>${badgeHTML(tunnel.actual_state)}</td>
        <td class="mono">${escapeHTML((tunnel.local_ip ?? '—') + ':' + (tunnel.local_port ?? '—'))}</td>
        <td class="mono">${escapeHTML(tunnel.remote_port ? String(tunnel.remote_port) : '—')}</td>
        <td class="mono">${escapeHTML(tunnel.state_reason ?? '—')}</td>
      </tr>
    `).join('') : '<tr><td colspan="8" class="muted">暂无隧道</td></tr>';
  }

  function renderWebsites() {
    refs.websitesBody.innerHTML = state.websites.length ? state.websites.map((item) => `
      <tr>
        <td>${escapeHTML(item.primary_domain ?? '—')}</td>
        <td>${escapeHTML(item.source_kind ?? '—')}</td>
        <td class="mono">${escapeHTML(item.node_id ?? '—')}</td>
        <td>${item.https_enabled ? badgeHTML('on') : badgeHTML('off')}</td>
        <td>${badgeHTML(item.status)}</td>
        <td class="mono">${escapeHTML(item.panel_website_id ?? '—')}</td>
        <td class="mono">${escapeHTML(item.runtime_key ?? '—')}</td>
      </tr>
    `).join('') : '<tr><td colspan="7" class="muted">暂无网站映射</td></tr>';
  }

  function renderJobs() {
    refs.jobsBody.innerHTML = state.jobs.length ? state.jobs.map((job) => `
      <tr>
        <td>${escapeHTML(job.title ?? job.id ?? '—')}</td>
        <td class="mono">${escapeHTML(job.kind ?? '—')}</td>
        <td class="mono">${escapeHTML((job.target_type ?? '—') + ':' + (job.target_id ?? '—'))}</td>
        <td>${badgeHTML(job.status)}</td>
        <td class="mono">${escapeHTML(String(job.attempt_count ?? 0) + '/' + String(job.max_attempts ?? 0))}</td>
        <td class="mono">${escapeHTML(formatTime(job.updated_at))}</td>
      </tr>
    `).join('') : '<tr><td colspan="6" class="muted">暂无作业</td></tr>';
  }

  function renderSettings() {
    const s = state.settings || {};
    refs.settingsBody.innerHTML = `
      <div class="settings-grid">
        <div class="settings-card">
          <h3>通用</h3>
          <div class="mini-grid">
            <label>默认日志行数 <input id="setDefaultLogLines" type="number" min="1" value="${escapeHTML(s.general?.default_log_lines ?? 100)}"></label>
            <label>数据保留天数 <input id="setDataRetentionDays" type="number" min="1" value="${escapeHTML(s.general?.data_retention_days ?? 30)}"></label>
            <label>默认刷新模式 <input id="setDefaultRefreshMode" type="text" value="${escapeHTML(s.general?.default_refresh_mode ?? 'polling')}"></label>
          </div>
        </div>
        <div class="settings-card">
          <h3>同步策略</h3>
          <div class="mini-grid">
            <label>健康检查间隔 <input id="setHealthcheckInterval" type="text" value="${escapeHTML(s.sync?.healthcheck_interval ?? '1m')}"></label>
            <label>轮询间隔 <input id="setSyncPollInterval" type="text" value="${escapeHTML(s.sync?.sync_poll_interval ?? '10s')}"></label>
            <label>差异策略 <input id="setDiffStrategy" type="text" value="${escapeHTML(s.sync?.diff_strategy ?? 'pause_on_conflict')}"></label>
            <label>手动优先级 <input id="setManualOverridePriority" type="text" value="${escapeHTML(s.sync?.manual_override_priority ?? 'manual_wins')}"></label>
          </div>
        </div>
        <div class="settings-card">
          <h3>队列</h3>
          <div class="mini-grid">
            <label>最大尝试次数 <input id="setQueueMaxAttempts" type="number" min="1" value="${escapeHTML(s.queue?.max_attempts ?? 5)}"></label>
            <label>重试退避 <input id="setQueueRetryBackoff" type="text" value="${escapeHTML(s.queue?.retry_backoff ?? '30s')}"></label>
            <label>卡住任务策略 <input id="setQueueStalledJobPolicy" type="text" value="${escapeHTML(s.queue?.stalled_job_policy ?? 'mark_blocked')}"></label>
            <label>归档保留天数 <input id="setQueueArchiveRetentionDays" type="number" min="1" value="${escapeHTML(s.queue?.archive_retention_days ?? 30)}"></label>
          </div>
        </div>
        <div class="settings-card">
          <h3>FRPC Runtime</h3>
          <div class="mini-grid">
            <label>启用 <select id="setFrpcEnabled"><option value="true" ${s.frpc_runtime?.frpc_enabled ? 'selected' : ''}>true</option><option value="false" ${!s.frpc_runtime?.frpc_enabled ? 'selected' : ''}>false</option></select></label>
            <label>二进制来源 <input id="setFrpcBinarySource" type="text" value="${escapeHTML(s.frpc_runtime?.frpc_binary_source ?? 'embedded')}"></label>
            <label>二进制版本 <input id="setFrpcBinaryVersion" type="text" value="${escapeHTML(s.frpc_runtime?.frpc_binary_version ?? '0.54.0')}"></label>
            <label>日志级别 <input id="setFrpcLogLevel" type="text" value="${escapeHTML(s.frpc_runtime?.frpc_log_level ?? 'info')}"></label>
            <label>健康检查间隔 <input id="setFrpcHealthcheckInterval" type="text" value="${escapeHTML(s.frpc_runtime?.frpc_healthcheck_interval ?? '30s')}"></label>
            <label>重启退避 <input id="setFrpcRestartBackoff" type="text" value="${escapeHTML(s.frpc_runtime?.frpc_restart_backoff ?? '30s')}"></label>
            <label>自动恢复策略 <input id="setAutoRecoverStrategy" type="text" value="${escapeHTML(s.frpc_runtime?.auto_recover_strategy ?? 'reload_then_restart')}"></label>
            <label>切换节点策略 <input id="setSwitchNodeStrategy" type="text" value="${escapeHTML(s.frpc_runtime?.switch_node_strategy ?? 'prefer_healthy_low_load')}"></label>
          </div>
        </div>
        <div class="settings-card span-2">
          <h3>集成 / Token</h3>
          <div class="mini-grid">
            <label>ChmlFrp 用户名 <input id="setChmlfrpUsername" type="text" value="${escapeHTML(s.integrations?.chmlfrp?.username ?? '')}"></label>
            <label>ChmlFrp 密码 <input id="setChmlfrpPassword" type="password" placeholder="输入后会保存在本地 state 文件"></label>
            <label>1Panel Base URL <input id="setOnepanelBaseURL" type="text" value="${escapeHTML(s.integrations?.onepanel?.base_url ?? '')}"></label>
            <label>1Panel Entrance <input id="setOnepanelEntrance" type="text" value="${escapeHTML(s.integrations?.onepanel?.entrance ?? '')}"></label>
            <label>1Panel API Token <input id="setOnepanelAPIToken" type="password" placeholder="已保存则显示掩码状态"></label>
            <label>Cloudflare API Token <input id="setCloudflareAPIToken" type="password" placeholder="已保存则显示掩码状态"></label>
            <label>Cloudflare Zone ID <input id="setCloudflareZoneID" type="text" value="${escapeHTML(s.integrations?.cloudflare?.zone_id ?? '')}"></label>
          </div>
          <div class="footer-note">Token 不会回显明文；页面只提示是否已配置。当前实现先落本地 state 文件，后续可再切到更安全的密钥存储。</div>
        </div>
      </div>
      <div class="settings-actions">
        <button id="saveSettingsBtnInner" type="button">保存设置</button>
        <button class="secondary" id="resetSettingsBtnInner" type="button">重置页面</button>
      </div>
    `;
    refs.integrationsSummary.textContent = `ChmlFrp: ${s.integrations?.chmlfrp?.has_password ? '已配置' : '未配置'} · 1Panel: ${s.integrations?.onepanel?.has_api_token ? '已配置' : '未配置'} · Cloudflare: ${s.integrations?.cloudflare?.has_api_token ? '已配置' : '未配置'}`;
    refs.tokenNote.textContent = 'Tokens are stored in the local backend state file in this iteration. No plaintext is rendered back into the UI.';
    dirtySettings = false;
    wireSettingsControls();
  }

  function renderEvents() {
    refs.eventCount.textContent = String(state.events.length);
    refs.eventsBody.innerHTML = state.events.length ? state.events.map((evt) => `
      <div class="event-item">
        <div class="head">
          <div class="kind">${escapeHTML(evt.kind ?? 'event')}</div>
          <div class="muted tiny">${escapeHTML(formatTime(evt.created_at))}</div>
        </div>
        <div class="tiny muted mono">channel: ${escapeHTML(evt.channel ?? '—')} · level: ${escapeHTML(evt.level ?? '—')} · cursor: ${escapeHTML(evt.cursor ?? '—')}</div>
        <div style="margin-top: 6px;">${escapeHTML(evt.message ?? '—')}</div>
      </div>
    `).join('') : '<div class="muted tiny">暂无事件</div>';
  }

  function renderSnapshot() {
    const version = state.version ?? {};
    const health = state.health ?? {};
    const runtime = state.runtime ?? {};

    refs.versionValue.textContent = version.version ?? '—';
    refs.versionDesc.textContent = [version.app_name, version.engine].filter(Boolean).join(' · ') || '—';
    refs.healthValue.innerHTML = badgeHTML(health.status ?? 'unknown');
    refs.healthDesc.textContent = [health.nodes, health.tunnels, health.website_mappings, health.jobs, health.events]
      .map((v, i) => ['nodes', 'tunnels', 'websites', 'jobs', 'events'][i] + ': ' + (v ?? 0))
      .join(' · ');
    refs.runtimeValue.innerHTML = badgeHTML(runtime.engine_status ?? 'unknown');
    refs.runtimeDesc.textContent = [runtime.frpc_version, runtime.active_tunnels_count != null ? (runtime.active_tunnels_count + ' active') : null, runtime.last_action]
      .filter(Boolean)
      .join(' · ') || '—';

    renderMeta(refs.versionMeta, [
      ['version', escapeHTML(version.version ?? '—')],
      ['engine', escapeHTML(version.engine ?? '—')],
      ['status', badgeHTML(version.status ?? 'unknown')],
      ['app_name', escapeHTML(version.app_name ?? '—')],
      ['api_base', `<span class="mono">${escapeHTML(version.api_base ?? '—')}</span>`],
      ['ui_base', `<span class="mono">${escapeHTML(version.ui_base ?? '—')}</span>`],
    ]);

    renderMeta(refs.healthMeta, [
      ['status', badgeHTML(health.status ?? 'unknown')],
      ['nodes', escapeHTML(health.nodes ?? 0)],
      ['tunnels', escapeHTML(health.tunnels ?? 0)],
      ['website_mappings', escapeHTML(health.website_mappings ?? 0)],
      ['jobs', escapeHTML(health.jobs ?? 0)],
      ['events', escapeHTML(health.events ?? 0)],
      ['data_file', `<span class="mono">${escapeHTML(health.data_file ?? '—')}</span>`],
    ]);

    renderMeta(refs.runtimeMeta, [
      ['frpc_version', escapeHTML(runtime.frpc_version ?? '—')],
      ['engine_status', badgeHTML(runtime.engine_status ?? 'unknown')],
      ['active_tunnels_count', escapeHTML(runtime.active_tunnels_count ?? 0)],
      ['active_tunnel_ids', `<span class="mono">${escapeHTML(Array.isArray(runtime.active_tunnel_ids) ? runtime.active_tunnel_ids.join(', ') || '—' : '—')}</span>`],
      ['last_action', escapeHTML(runtime.last_action ?? '—')],
      ['updated_at', `<span class="mono">${escapeHTML(formatTime(runtime.updated_at))}</span>`],
    ]);

    refs.lastRefresh.textContent = 'updated ' + new Date().toLocaleTimeString();
  }

  function applyData(data) {
    state.version = data.version;
    state.health = data.health;
    state.nodes = data.nodes;
    state.tunnels = data.tunnels;
    state.websites = data.websites;
    state.jobs = data.jobs;
    state.runtime = data.runtime;
    if (!dirtySettings || !state.settings) {
      state.settings = data.settings;
    }
    renderSnapshot();
    renderNodes();
    renderTunnels();
    renderWebsites();
    renderJobs();
    if (!dirtySettings) {
      renderSettings();
    } else {
      refs.tokenNote.textContent = '设置正在编辑中，自动刷新已暂缓，保存后会恢复同步。';
    }
    renderEvents();
  }

  async function request(path, options = {}) {
    const response = await fetch(API + path, {
      headers: { Accept: 'application/json', ...(options.headers || {}) },
      ...options,
    });
    const text = await response.text();
    let payload = null;
    try {
      payload = text ? JSON.parse(text) : null;
    } catch (error) {
      throw new Error(text || ('HTTP ' + response.status));
    }
    if (!response.ok) {
      throw new Error((payload && payload.error && payload.error.message) || ('HTTP ' + response.status));
    }
    return payload ? payload.data : null;
  }

  async function refresh(silent) {
    if (refreshInFlight) return;
    refreshInFlight = true;
    setError('');
    if (!silent) {
      refs.refreshBtn.disabled = true;
      refs.refreshBtn.textContent = 'Refreshing…';
    }
    try {
      const results = await Promise.allSettled([
        request('/version'),
        request('/health'),
        request('/nodes'),
        request('/tunnels'),
        request('/website-mappings'),
        request('/jobs'),
        request('/frpc/runtime'),
        request('/settings'),
      ]);

      const [version, health, nodes, tunnels, websites, jobs, runtime, settings] = results;
      const next = {
        version: version.status === 'fulfilled' ? version.value : null,
        health: health.status === 'fulfilled' ? health.value : null,
        nodes: nodes.status === 'fulfilled' ? (nodes.value.nodes || []) : [],
        tunnels: tunnels.status === 'fulfilled' ? (tunnels.value.tunnels || []) : [],
        websites: websites.status === 'fulfilled' ? (websites.value.website_mappings || []) : [],
        jobs: jobs.status === 'fulfilled' ? (jobs.value.jobs || []) : [],
        runtime: runtime.status === 'fulfilled' ? runtime.value : null,
        settings: settings.status === 'fulfilled' ? settings.value : null,
      };

      applyData(next);
      const failures = results.filter((item) => item.status === 'rejected');
      if (failures.length > 0) {
        setError(failures.map((item) => item.reason && item.reason.message ? item.reason.message : String(item.reason)).join(' · '));
      }
    } catch (error) {
      setError(error instanceof Error ? error.message : String(error));
    } finally {
      refreshInFlight = false;
      if (!silent) {
        refs.refreshBtn.disabled = false;
        refs.refreshBtn.textContent = 'Refresh';
      }
    }
  }

  function queueRefresh() {
    if (dirtySettings) return;
    if (refreshTimer) clearTimeout(refreshTimer);
    refreshTimer = setTimeout(() => refresh(true), 300);
  }

  function connectStream() {
    if (stream) stream.close();
    state.events = [];
    renderEvents();
    stream = new EventSource(API + '/events/stream');
    refs.streamBadge.textContent = 'SSE: connecting';
    refs.streamBadge.className = 'badge warn';

    stream.onopen = function () {
      refs.streamBadge.textContent = 'SSE: connected';
      refs.streamBadge.className = 'badge good';
    };

    stream.onerror = function () {
      refs.streamBadge.textContent = 'SSE: reconnecting';
      refs.streamBadge.className = 'badge warn';
    };

    stream.onmessage = function (event) {
      try {
        const parsed = JSON.parse(event.data);
        state.events.unshift(parsed);
        state.events = state.events.slice(0, 20);
        renderEvents();
        queueRefresh();
      } catch (error) {
        console.warn('failed to parse SSE payload', error);
      }
    };
  }

  function getNodeFormData() {
    const metadataText = refs.nodeMetadata.value.trim();
    let metadata = undefined;
    if (metadataText) {
      metadata = JSON.parse(metadataText);
    }
    return {
      display_name: refs.nodeDisplayName.value.trim(),
      provider: refs.nodeProvider.value.trim(),
      node_type: refs.nodeType.value.trim(),
      endpoint_url: refs.nodeEndpointURL.value.trim(),
      region: refs.nodeRegion.value.trim(),
      status: refs.nodeStatus.value.trim(),
      canonical_name: refs.nodeCanonicalName.value.trim(),
      metadata,
    };
  }

  function resetNodeForm() {
    nodeEditID = '';
    refs.nodeForm.reset();
    refs.nodeId.value = '';
    refs.nodeStatus.value = 'active';
    refs.nodeType.value = 'frp_node';
    refs.nodeProvider.value = 'chmlfrp';
    refs.nodeSubmitBtn.textContent = '新增节点';
  }

  function fillNodeForm(node) {
    nodeEditID = node.id || '';
    refs.nodeId.value = node.id || '';
    refs.nodeDisplayName.value = node.display_name || '';
    refs.nodeProvider.value = node.provider || 'chmlfrp';
    refs.nodeType.value = node.node_type || 'frp_node';
    refs.nodeEndpointURL.value = node.endpoint_url || '';
    refs.nodeRegion.value = node.region || '';
    refs.nodeStatus.value = node.status || 'active';
    refs.nodeCanonicalName.value = node.canonical_name || '';
    refs.nodeMetadata.value = node.metadata ? jsonPretty(node.metadata) : '';
    refs.nodeSubmitBtn.textContent = '保存节点';
  }

  async function submitNodeForm(event) {
    event.preventDefault();
    let payload;
    try {
      payload = getNodeFormData();
    } catch (error) {
      setError('节点 metadata JSON 解析失败');
      return;
    }
    if (!payload.display_name || !payload.provider || !payload.node_type) {
      setError('display_name / provider / node_type 不能为空');
      return;
    }
    await request(nodeEditID ? `/nodes/${encodeURIComponent(nodeEditID)}` : '/nodes', {
      method: nodeEditID ? 'PATCH' : 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    resetNodeForm();
    await refresh(false);
  }

  function updateSettingsDirty() {
    dirtySettings = true;
    refs.tokenNote.textContent = '设置已修改，尚未保存。自动刷新将暂缓，保存后会继续同步。';
  }

  function wireSettingsControls() {
    const ids = [
      'setDefaultLogLines','setDataRetentionDays','setDefaultRefreshMode',
      'setHealthcheckInterval','setSyncPollInterval','setDiffStrategy','setManualOverridePriority',
      'setQueueMaxAttempts','setQueueRetryBackoff','setQueueStalledJobPolicy','setQueueArchiveRetentionDays',
      'setFrpcEnabled','setFrpcBinarySource','setFrpcBinaryVersion','setFrpcLogLevel','setFrpcHealthcheckInterval',
      'setFrpcRestartBackoff','setAutoRecoverStrategy','setSwitchNodeStrategy',
      'setChmlfrpUsername','setChmlfrpPassword','setOnepanelBaseURL','setOnepanelEntrance','setOnepanelAPIToken','setCloudflareAPIToken','setCloudflareZoneID',
      'saveSettingsBtnInner','resetSettingsBtnInner',
    ];
    ids.forEach((id) => {
      const el = document.getElementById(id);
      if (!el) return;
      if (el.dataset.bound === '1') return;
      el.dataset.bound = '1';
      if (el.tagName === 'INPUT' || el.tagName === 'SELECT') {
        el.addEventListener('input', updateSettingsDirty);
        el.addEventListener('change', updateSettingsDirty);
      }
      if (id === 'saveSettingsBtnInner') {
        el.addEventListener('click', saveSettings);
      }
      if (id === 'resetSettingsBtnInner') {
        el.addEventListener('click', () => renderSettings());
      }
    });
  }

  function collectSettingsPayload() {
    const s = state.settings || {};
    const boolFromSelect = (id) => document.getElementById(id)?.value === 'true';
    return {
      general: {
        default_log_lines: Number(document.getElementById('setDefaultLogLines').value || 100),
        data_retention_days: Number(document.getElementById('setDataRetentionDays').value || 30),
        default_refresh_mode: document.getElementById('setDefaultRefreshMode').value.trim() || 'polling',
      },
      sync: {
        healthcheck_interval: document.getElementById('setHealthcheckInterval').value.trim() || '1m',
        sync_poll_interval: document.getElementById('setSyncPollInterval').value.trim() || '10s',
        diff_strategy: document.getElementById('setDiffStrategy').value.trim() || 'pause_on_conflict',
        manual_override_priority: document.getElementById('setManualOverridePriority').value.trim() || 'manual_wins',
      },
      queue: {
        max_attempts: Number(document.getElementById('setQueueMaxAttempts').value || 5),
        retry_backoff: document.getElementById('setQueueRetryBackoff').value.trim() || '30s',
        stalled_job_policy: document.getElementById('setQueueStalledJobPolicy').value.trim() || 'mark_blocked',
        archive_retention_days: Number(document.getElementById('setQueueArchiveRetentionDays').value || 30),
      },
      frpc_runtime: {
        frpc_enabled: boolFromSelect('setFrpcEnabled'),
        frpc_binary_source: document.getElementById('setFrpcBinarySource').value.trim() || 'embedded',
        frpc_binary_version: document.getElementById('setFrpcBinaryVersion').value.trim() || '0.54.0',
        frpc_log_level: document.getElementById('setFrpcLogLevel').value.trim() || 'info',
        frpc_healthcheck_interval: document.getElementById('setFrpcHealthcheckInterval').value.trim() || '30s',
        frpc_restart_backoff: document.getElementById('setFrpcRestartBackoff').value.trim() || '30s',
        auto_recover_strategy: document.getElementById('setAutoRecoverStrategy').value.trim() || 'reload_then_restart',
        switch_node_strategy: document.getElementById('setSwitchNodeStrategy').value.trim() || 'prefer_healthy_low_load',
      },
      integrations: {
        chmlfrp: {
          username: document.getElementById('setChmlfrpUsername').value.trim() || s.integrations?.chmlfrp?.username || '',
          password: document.getElementById('setChmlfrpPassword').value || undefined,
        },
        onepanel: {
          base_url: document.getElementById('setOnepanelBaseURL').value.trim() || '',
          entrance: document.getElementById('setOnepanelEntrance').value.trim() || '',
          api_token: document.getElementById('setOnepanelAPIToken').value || undefined,
        },
        cloudflare: {
          api_token: document.getElementById('setCloudflareAPIToken').value || undefined,
          zone_id: document.getElementById('setCloudflareZoneID').value.trim() || '',
        },
      },
    };
  }

  async function saveSettings() {
    const payload = collectSettingsPayload();
    await request('/settings', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    dirtySettings = false;
    setError('');
    await refresh(false);
  }

  function bindGlobalActions() {
    refs.refreshBtn.addEventListener('click', () => refresh(false));
    refs.docsBtn.addEventListener('click', () => { window.location.href = '/api/docs'; });
    refs.openApiBtn.addEventListener('click', () => { window.location.href = '/api/openapi.json'; });
    refs.nodeForm.addEventListener('submit', submitNodeForm);
    refs.nodeClearBtn.addEventListener('click', resetNodeForm);
    refs.settingsBody.addEventListener('input', updateSettingsDirty);
    refs.settingsBody.addEventListener('change', updateSettingsDirty);
    refs.nodesBody.addEventListener('click', async (event) => {
      const btn = event.target.closest('button[data-action]');
      if (!btn) return;
      const id = btn.dataset.id;
      const action = btn.dataset.action;
      const node = state.nodes.find((item) => item.id === id);
      if (action === 'edit-node' && node) fillNodeForm(node);
      if (action === 'check-node') {
        await request(`/nodes/${encodeURIComponent(id)}/actions/check`, { method: 'POST' });
        await refresh(false);
      }
      if (action === 'archive-node') {
        await request(`/nodes/${encodeURIComponent(id)}/actions/archive`, { method: 'POST' });
        if (nodeEditID === id) resetNodeForm();
        await refresh(false);
      }
    });
  }

  connectStream();
  bindGlobalActions();
  resetNodeForm();
  refresh(false);
})();
