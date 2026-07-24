import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import vm from 'node:vm';

const source = await readFile(new URL('./dist/app.js', import.meta.url), 'utf8');

function createContext() {
  const elements = new Map();
  const context = {
    console,
    setTimeout: (callback) => callback(),
    requestAnimationFrame: (callback) => callback(),
    fetch: async () => { throw new Error('unexpected fetch'); },
    navigator: { clipboard: { writeText: async () => {} } },
    document: {
      addEventListener() {},
      getElementById(id) { return elements.get(id) || null; },
      querySelectorAll() { return []; },
      createElement() { return { setAttribute() {}, select() {}, remove() {}, style: {}, value: '' }; },
      execCommand() { return true; },
      body: { appendChild() {}, innerHTML: '' },
      cookie: '',
    },
  };
  vm.createContext(context);
  vm.runInContext(source, context);
  return context;
}

test('uses the public session probe without exposing or rewriting the session cookie', () => {
  assert.ok(source.includes("request('/auth/session'"));
  assert.ok(!source.includes("request('/auth/me'"));
  assert.ok(!/document\.cookie\s*=\s*.*ashan_frp_session/.test(source));
  assert.doesNotMatch(source, /request\(['"]\/(?:auth\/)?(?:forgot|reset)/i);
});

test('Cloudflare settings never render a saved token', () => {
  const context = createContext();
  vm.runInContext("STATE.settings = { integrations: { cloudflare: { configured: true, identifier: 'example.com', api_token: 'actual-secret' } } };", context);
  const html = vm.runInContext('renderCloudflareSettings()', context);
  assert.match(html, /id="cloudflare-zone"/);
  assert.match(html, /id="cloudflare-api-token"/);
  assert.match(html, /type="text"/);
  assert.doesNotMatch(html, /actual-secret/);
});

test('Cloudflare settings show only the safe credential identity and verification history entry', () => {
  const context = createContext();
  vm.runInContext("STATE.settings = { integrations: { cloudflare: { configured: true, identifier: 'example.com', token_mask: '****ABCD', credential_ref: 'abc123def456', credential_revision: 3, api_token: 'actual-secret' } } };", context);
  const html = vm.runInContext('renderCloudflareSettings()', context);
  assert.match(html, /Token 掩码/);
  assert.match(html, /\*\*\*\*ABCD/);
  assert.match(html, /abc123def456/);
  assert.match(html, /版本 3/);
  assert.match(html, /查看验证记录/);
  assert.doesNotMatch(html, /actual-secret/);
});

test('audit log renders safe correlation fields, filters, and expandable details', () => {
  const context = createContext();
  vm.runInContext("STATE.auditLogs = [{ action: 'cloudflare.credential.verify', outcome: 'failure', duration_ms: 81, request_id: 'req-123', credential_ref: 'abc123def456', error_code: 'CLOUDFLARE_TOKEN_INVALID', detail_json: '{\"provider\":\"cloudflare\"}', created_at: '2026-07-17T00:00:00Z' }];", context);
  const html = vm.runInContext('renderLogs()', context);
  assert.match(html, /安全审计日志/);
  assert.match(html, /查看详情/);
  assert.match(html, /req-123/);
  assert.match(html, /abc123def456/);
  assert.match(html, /CLOUDFLARE_TOKEN_INVALID/);
  assert.match(html, /audit-outcome/);
  assert.doesNotMatch(html, /actual-secret|Bearer\s+[A-Za-z0-9._-]+/);
});



test('phase 4 navigation exposes eight focused entries and hides legacy views', () => {
  const context = createContext();
  const nav = vm.runInContext('renderNav()', context);
  assert.equal((nav.match(/class="nav-item/g) || []).length, 8);
  for (const id of ['control','dashboard','dns','frp','nodes','jobs','logs','settings']) {
    assert.match(nav, new RegExp(`data-page="${id}"`));
  }
  for (const id of ['domains','website','tunnels','websites']) {
    assert.doesNotMatch(nav, new RegExp(`data-page="${id}"`));
  }
  vm.runInContext("STATE.authMe = { login_name: 'admin' }; STATE.settings = { integrations: {} }; STATE.activePage = 'tunnels';", context);
  const shell = vm.runInContext('appShell()', context);
  assert.equal(vm.runInContext('STATE.activePage', context), 'control');
  assert.match(shell, /data-view="control"/);
  assert.doesNotMatch(shell, /data-view="(?:domains|website|tunnels|websites)"/);
  assert.ok(source.includes('renderControlPage()}${renderDashboard()}${renderDNS()}${renderFRP()}${renderNodes()}${renderJobs()}${renderLogs()}${renderSettings()}'));
});

test('settings center renders credential cards, runtime policies, and account controls without secrets', () => {
  const context = createContext();
  vm.runInContext("STATE.settings = { general: { default_log_lines: 120, data_retention_days: 45, default_refresh_mode: 'polling' }, sync: { healthcheck_interval: '1m', sync_poll_interval: '10s' }, queue: { max_attempts: 5, retry_backoff: '30s', stalled_job_policy: 'mark_blocked', archive_retention_days: 30 }, frpc_runtime: { frpc_enabled: true, frpc_log_level: 'debug', frpc_healthcheck_interval: '30s', frpc_restart_backoff: '30s', auto_recover_strategy: 'reload_then_restart', switch_node_strategy: 'prefer_healthy_low_load' }, integrations: { cloudflare: { configured: true, zone_name: 'example.com', token_mask: '****SAFE', credential_ref: 'ref-safe', credential_revision: 2, api_token: 'actual-secret' }, chmlfrp: { username: 'frog', has_password: true, password: 'frog-secret' }, onepanel: { base_url: 'https://panel.example.com', has_api_token: true, api_token: 'panel-secret' } } }; STATE.authTokens = [{ id: 'tok_123456789', token_type: 'session', expires_at: '2026-07-24T00:00:00Z', last_used_at: '2026-07-23T00:00:00Z' }];", context);
  const html = vm.runInContext('renderSettings()', context);
  assert.match(html, /settings-card-cloudflare/);
  assert.match(html, /chmlfrp-settings-form/);
  assert.match(html, /onepanel-settings-form/);
  assert.match(html, /settings-runtime-form/);
  assert.match(html, /settings-policy-form/);
  assert.match(html, /settings-password-form/);
  assert.match(html, /data-action="token-revoke"/);
  assert.match(html, /\*\*\*\*SAFE/);
  assert.match(html, /ref-safe/);
  assert.doesNotMatch(html, /actual-secret|frog-secret|panel-secret/);
  assert.ok(source.includes("request('/settings'"));
  assert.ok(source.includes("request('/auth/password/change'"));
});

test('DNS console displays grouped Cloudflare records, tunnel management tags, and CRUD actions', () => {
  const context = createContext();
  vm.runInContext("STATE.settings = { integrations: { cloudflare: { configured: true, identifier: 'example.com' } } }; STATE.nodes = [{ id: 'node_1', display_name: 'edge-node' }]; STATE.tunnels = [{ id: 'tun_1', node_id: 'node_1', name: 'web', full_domain: 'www.example.com', local_ip: '127.0.0.1', local_port: 8080, cf_record_id: 'rec_1' }]; STATE.dnsRecords = [{ id: 'rec_1', name: 'www.example.com', type: 'CNAME', content: 'edge.example.net', ttl: 1, proxied: true, proxiable: true }]; STATE.dnsLoaded = true;", context);
  const html = vm.runInContext('renderDNS()', context);
  assert.match(html, /dns-accordion/);
  assert.match(html, /example\.com/);
  assert.match(html, /www\.example\.com/);
  assert.match(html, /data-dns-action="from-tunnel"/);
  assert.match(html, /data-dns-action="new"/);
  assert.match(html, /data-dns-action="edit"/);
  assert.match(html, /data-dns-action="delete"/);
  assert.match(html, /managed-tag/);
  assert.match(html, /127\.0\.0\.1:8080/);
  assert.doesNotMatch(html, /api_token|actual-secret/i);
  assert.ok(source.includes("request('/dns/records'"));
});

test('DNS editor supports CAA and requires an exact record name for deletion', () => {
  const context = createContext();
  const formHtml = vm.runInContext("STATE.dnsEditor = dnsEditorRecord({ id: 'rec_1', type: 'CAA', name: 'example.com', ttl: 300, data: { flags: 0, tag: 'issue', value: 'letsencrypt.org' } }); renderDNSRecordForm()", context);
  assert.match(formHtml, /dns-caa-flags/);
  assert.match(formHtml, /dns-caa-tag/);
  assert.match(formHtml, /dns-caa-value/);
  const cnameHtml = vm.runInContext("STATE.tunnels = [{ id: 'tun_1', node_id: 'node_1', full_domain: 'app.example.com', dns_domain_cname: 'edge.example.net', local_ip: '127.0.0.1', local_port: 8080, cf_proxied: true }]; STATE.nodes = [{ id: 'node_1', display_name: 'edge-node' }]; STATE.dnsEditor = dnsEditorRecord(null, { fromTunnel: true }); renderDNSRecordForm()", context);
  assert.match(cnameHtml, /dns-tunnel-source/);
  assert.match(cnameHtml, /app\.example\.com/);
  assert.match(cnameHtml, /edge\.example\.net/);
  const deleteHtml = vm.runInContext("STATE.dnsDeleteRecord = { id: 'rec_1', type: 'A', name: 'example.com', content: '192.0.2.1' }; STATE.dnsDeleteName = ''; renderDNSDeleteDialog()", context);
  assert.match(deleteHtml, /dns-delete-name/);
  assert.match(deleteHtml, /disabled/);
});
