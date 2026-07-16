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
  assert.match(html, /id="login-form"/);
  assert.match(html, /忘记密码？/);
  assert.match(html, /type="submit"/);
  assert.doesNotMatch(html, /href="[^"]*(?:forgot|reset)/i);
});

test('recovery dialog explains terminal-only reset and both command forms', () => {
  const context = createContext();
  const html = vm.runInContext('STATE.recoveryOpen = true; recoveryDialog()', context);
  assert.match(html, /系统不提供网页密码重置/);
  assert.match(html, /\.\/ashan-frp admin list/);
  assert.match(html, /docker compose exec ashan-frp \/app\/ashan-frp admin reset-password/);
  assert.match(html, /--new-username/);
  assert.match(html, /所有旧 Session 和 API Token 会立即失效/);
});

test('copy action writes only the selected CLI command', async () => {
  const context = createContext();
  await vm.runInContext('copyRecoveryCommand("docker")', context);
  assert.equal(context.copied.length, 1);
  assert.match(context.copied[0], /^docker compose exec/);
  assert.doesNotMatch(context.copied[0], /password\s+[^-\s]/i);
  assert.equal(vm.runInContext('STATE.recoveryCopyStatus', context), '恢复命令已复制');
});

test('recovery UI never calls a public password reset endpoint', () => {
  assert.doesNotMatch(source, /request\(['"]\/(?:auth\/)?(?:forgot|reset)[^'"]*['"]/i);
  assert.match(source, /event\.key === 'Escape'/);
  assert.match(source, /loginForm\.addEventListener\('submit'/);
});
