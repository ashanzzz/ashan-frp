import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import vm from 'node:vm';

const source = await readFile(new URL('./dist/app.js', import.meta.url), 'utf8');

function createContext() {
  const copied = [];
  const elements = new Map();
  const context = {
    console,
    setTimeout: (callback) => callback(),
    requestAnimationFrame: (callback) => callback(),
    fetch: async () => { throw new Error('unexpected fetch'); },
    navigator: { clipboard: { writeText: async (value) => { copied.push(value); } } },
    document: {
      addEventListener() {},
      getElementById(id) { return elements.get(id) || null; },
      querySelectorAll() { return []; },
      createElement() { return { setAttribute() {}, select() {}, remove() {}, style: {}, value: '' }; },
      execCommand() { return true; },
      body: { appendChild() {}, innerHTML: '' },
      cookie: '',
    },
    copied,
    elements,
  };
  vm.createContext(context);
  vm.runInContext(source, context);
  return context;
}

test('login panel is localized and exposes the recovery entry', () => {
  const context = createContext();
  const html = vm.runInContext('loginPanel()', context);
  assert.match(html, /进入 Ashan FRP 运营台/);
  assert.match(html, /唯一管理员账号/);
  assert.match(html, /id="login-form"/);
  assert.match(html, /忘记密码？/);
  assert.match(html, /type="submit"/);
  assert.doesNotMatch(html, /href="[^"]*(?:forgot|reset)/i);
});

test('recovery dialog explains single-admin terminal reset and both command forms', () => {
  const context = createContext();
  const html = vm.runInContext('STATE.recoveryOpen = true; recoveryDialog()', context);
  assert.match(html, /系统不提供网页密码重置/);
  assert.match(html, /无法查看当前密码/);
  assert.match(html, /Ashan FRP 只允许一个管理员/);
  assert.match(html, /\.\/ashan-frp admin reset-password/);
  assert.match(html, /docker compose exec -it ashan-frp \/app\/ashan-frp admin reset-password/);
  assert.doesNotMatch(html, /admin list|--username|--new-username/);
  assert.match(html, /所有旧 Session 和 API Token 会立即失效/);
});

test('copy action writes only the selected CLI command', async () => {
  const context = createContext();
  await vm.runInContext('copyRecoveryCommand("docker")', context);
  assert.equal(context.copied.length, 1);
  assert.match(context.copied[0], /^docker compose exec/);
  assert.doesNotMatch(context.copied[0], /password\s+[^-\s]/i);
  assert.equal(vm.runInContext('STATE.recoveryCopyStatus', context), '恢复命令已复制。');
});

test('recovery UI never calls a public password reset endpoint', () => {
  assert.doesNotMatch(source, /request\(['"]\/(?:auth\/)?(?:forgot|reset)[^'"]*['"]/i);
  assert.match(source, /event\.key === 'Escape'/);
  assert.match(source, /login\.addEventListener\('submit'/);
});

test('authenticated console renders operational pages instead of placeholders', () => {
  const context = createContext();
  vm.runInContext(`
    STATE.authMe = { id: 'acc_admin', login_name: 'admin' };
    STATE.nodes = [{ id: 'node_1', display_name: '上海节点', provider: 'chmlfrp', health_status: 'healthy', status: 'online', region: '上海' }];
    STATE.tunnels = [{ id: 'tun_1', name: '网站入口', protocol: 'https', full_domain: 'example.com', local_ip: '127.0.0.1', local_port: 3000, desired_state: 'enabled', actual_state: 'running', dns_domain_cname: 'edge.example.net' }];
    STATE.websites = [{ id: 'web_1', primary_domain: 'example.com', domains: ['www.example.com'], https_enabled: true, proxy_enabled: true, status: 'synced', tunnel_id: 'tun_1' }];
    STATE.jobs = [{ id: 'job_1', title: '同步隧道', status: 'succeeded', target_type: 'tunnel', updated_at: '2026-07-16T00:00:00Z' }];
    STATE.auditLogs = [{ action: 'tunnel.provision', account_name: 'admin', resource_id: 'tun_1', created_at: '2026-07-16T00:00:00Z' }];
    STATE.frpcRuntime = { status: 'running', health_status: 'healthy' };
    STATE.settings = { frpc_runtime: { frpc_enabled: true, frpc_log_level: 'info' }, sync: { sync_poll_interval: '10s' }, integrations: {} };
  `, context);
  const html = vm.runInContext('appShell()', context);
  assert.doesNotMatch(html, /wired and ready for detail expansion/i);
  for (const text of ['运营总览', 'DNS 记录', 'FRP 运行时', '网站隧道', '任务中心', '系统设置']) assert.match(html, new RegExp(text));
  assert.match(html, /启动 FRPC/);
  assert.match(html, /同步节点/);
  assert.match(html, /部署/);
  assert.match(html, /example\.com/);
});

test('operations use only implemented authenticated endpoints', () => {
  for (const endpoint of ['/frpc/start', '/frpc/stop', '/frpc/restart', '/nodes/sync', '/tunnels/', '/website-mappings/', '/auth/tokens/']) assert.ok(source.includes(endpoint), endpoint);
  assert.doesNotMatch(source, /(?:\/forgot-password|\/password\/reset|\/auth\/reset)/i);
});
test('Cloudflare settings form accepts a replacement token without exposing the stored value', () => {
  const context = createContext();
  vm.runInContext(`STATE.settings = { integrations: { cloudflare: { configured: true, identifier: 'example.com', api_token: 'actual-secret' } } };`, context);
  const html = vm.runInContext('renderCloudflareSettings()', context);
  assert.match(html, /id="cloudflare-zone"/);
  assert.match(html, /id="cloudflare-api-token"/);
  assert.match(html, /type="password"/);
  assert.match(html, /留空则保留当前 Token/);
  assert.match(html, /验证已保存 Token/);
  assert.doesNotMatch(html, /actual-secret/);
  assert.match(source, /\/settings\/integrations\/cloudflare\/verify/);
});
