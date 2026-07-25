import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const source = await readFile(new URL('./dist/app.js', import.meta.url), 'utf8');

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
