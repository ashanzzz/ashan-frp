import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { cloudflareConfigureFailureMessage } from './src/provider-errors.js';

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

test('Cloudflare 配置前的系统会话 401 不会被冒充为 Cloudflare 上游失败', () => {
  assert.equal(
    cloudflareConfigureFailureMessage(401, { code: 'UNAUTHORIZED', message: 'Authentication required' }),
    '当前登录会话已失效；该请求未发送到 Cloudflare。请重新登录后再试。',
  );
});

test('settings renders a re-login path and explicit current ChmlFrp account and plaintext token fields', () => {
  assert.match(component, /\/auth\/session/);
  assert.match(component, /当前账户/);
  assert.match(component, /当前 Token（明文）/);
  assert.match(component, /重新登录/);
});

test('only the ChmlFrp credential action resubmits its plaintext token for verification', () => {
  assert.match(component, /saveSettings\(true\)/);
  assert.match(component, /password: saveChmlFrpCredential \? settingsForm\.value\.chmlfrpToken : ''/);
});
