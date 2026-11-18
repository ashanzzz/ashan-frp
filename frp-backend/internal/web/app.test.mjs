import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const source = await readFile(new URL('./dist/app.js', import.meta.url), 'utf8');
const component = await readFile(new URL('./src/App.vue', import.meta.url), 'utf8');
const styles = await readFile(new URL('./src/index.css', import.meta.url), 'utf8');
const distIndex = await readFile(new URL('./dist/index.html', import.meta.url), 'utf8');

test('uses the public session probe without exposing or rewriting the session cookie', () => {
  assert.ok(source.includes('/api/v1'));
  assert.ok(!/document\.cookie\s*=\s*.*ashan_frp_session/.test(source));
  assert.doesNotMatch(source, /request\(['"]\/(?:auth\/)?(?:forgot|reset)/i);
});

test('Vue 3 production bundle exposes mount target and reactive components', () => {
  assert.ok(source.length > 1000);
  assert.ok(source.includes('mount') || source.includes('#app'));
});

test('phase 4 navigation exposes focused entries for control, tunnels, nodes, frp, dns', () => {
  for (const pageName of ['总控制台', '穿透规则', '网络节点', '守护', 'Cloudflare DNS']) {
    assert.ok(source.includes(pageName));
  }
});

test('Nodes view exposes Node IP and Region attributes', () => {
  assert.ok(source.includes('物理地区与线路') || source.includes('Region'));
  assert.ok(source.includes('节点 IP') || source.includes('real_ip'));
});

test('settings keeps provider secrets visibly plaintext by product decision', () => {
  assert.match(component, /v-model="settingsForm\.cfApiToken"\s+type="text"/);
  assert.match(component, /v-model="settingsForm\.chmlfrpToken"\s+type="text"/);
  assert.match(component, /默认直接显示完整密钥/);
});

test('Cloudflare setup uses detect-and-save flow with email and zone selection', () => {
  assert.match(component, /settings\/integrations\/cloudflare\/configure/);
  assert.match(component, /cfAccountEmail/);
  assert.match(component, /cfSelectedZoneId/);
  assert.match(component, /检测并保存|保存所选域名/);
});

test('interactive surfaces do not use expensive backdrop blur', () => {
  assert.doesNotMatch(component, /backdrop-blur/);
  assert.doesNotMatch(styles, /backdrop-filter/);
});

test('embedded static asset URLs carry reproducible cache-busting hashes', () => {
  assert.match(distIndex, /\/ui\/app\.js\?v=[a-f0-9]{12}/);
  assert.match(distIndex, /\/ui\/styles\.css\?v=[a-f0-9]{12}/);
});

test('admin login gate provides login modal and session validation', () => {
  assert.ok(source.includes('/auth/session'));
  assert.ok(source.includes('/auth/login'));
  assert.ok(source.includes('/auth/logout'));
  assert.match(component, /管理员登录/);
  assert.match(component, /v-model="loginForm\.username"/);
  assert.match(component, /v-model="loginForm\.password"/);
});

test('DNS view renders Cloudflare record table and claim actions', () => {
  assert.ok(source.includes('/dns/records'));
  assert.match(component, /Cloudflare DNS 解析管理/);
  assert.match(component, /claimDNSRecord/);
  assert.match(component, /unclaimDNSRecord/);
  assert.match(component, /deleteDNSRecord/);
  assert.match(component, /原生记录保护纪律/);
});

test('frontend strictly restricts DNS deletion to records managed by ashan-frp', () => {
  // Verify component logic only treats ashan-frp prefix as managed
  assert.match(component, /c\.startsWith\(['"]ashan-frp managed:['"]\)/);
  assert.match(component, /1panel.*managed:\s*false/i);
  assert.match(component, /v-if="getRecordOriginLabel\(rec\.comment\)\.managed"\s+@click="deleteDNSRecord\(rec\)"/);
  // Verify compiled bundle enforces managed flag before rendering delete button
  assert.match(source, /ashan-frp managed:/);
  assert.match(source, /\.managed/);
});

test('tunnels view supports explicit provisioning and auto-provisioning action', () => {
  assert.ok(source.includes('/provision'));
  assert.match(component, /provisionTunnel/);
  assert.match(component, /⚡ 部署/);
});

test('FRPC view exposes runtime logs and config inspection', () => {
  assert.ok(source.includes('/frpc/logs'));
  assert.ok(source.includes('/frpc/config'));
  assert.match(component, /fetchFrpcLogs/);
  assert.match(component, /fetchFrpcConfig/);
  assert.match(component, /frpc\.toml/);
});

test('Control Center renders failover watchdog banner and controls', () => {
  assert.ok(source.includes('/failover/status'));
  assert.match(component, /Weekly Optimizer/);
  assert.match(component, /toggleAutoFailover/);
  assert.match(component, /triggerManualFailover/);
  assert.match(component, /triggerWeeklyOptimize/);
  assert.match(component, /10分钟/);
});

test('Control Center renders full-chain matrix table with 1Panel, SSL, ChmlFrp and Cloudflare columns', () => {
  assert.ok(source.includes('/matrix'));
  assert.match(component, /syncMatrix/);
  assert.match(component, /1Panel 反代与目标/);
  assert.match(component, /SSL 证书安全/);
  assert.match(component, /全链路一键同步/);
});

