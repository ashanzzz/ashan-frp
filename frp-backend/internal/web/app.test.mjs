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
  assert.match(html, /type="password"/);
  assert.doesNotMatch(html, /actual-secret/);
});

test('DNS console displays Cloudflare records, managed status, and CRUD actions', () => {
  const context = createContext();
  vm.runInContext("STATE.settings = { integrations: { cloudflare: { configured: true, identifier: 'example.com' } } }; STATE.dnsRecords = [{ id: 'rec_1', name: 'www.example.com', type: 'CNAME', content: 'edge.example.net', ttl: 1, comment: 'ashan-frp managed: tunnel tun_1' }]; STATE.dnsLoaded = true;", context);
  const html = vm.runInContext('renderDNS()', context);
  assert.match(html, /www\.example\.com/);
  assert.match(html, /data-dns-action="new"/);
  assert.match(html, /data-dns-action="edit"/);
  assert.match(html, /data-dns-action="delete"/);
  assert.match(html, /managed-tag/);
  assert.doesNotMatch(html, /api_token|actual-secret/i);
  assert.ok(source.includes("request('/dns/records'"));
});

test('DNS editor supports CAA and requires an exact record name for deletion', () => {
  const context = createContext();
  const formHtml = vm.runInContext("STATE.dnsEditor = dnsEditorRecord({ id: 'rec_1', type: 'CAA', name: 'example.com', ttl: 300, data: { flags: 0, tag: 'issue', value: 'letsencrypt.org' } }); renderDNSRecordForm()", context);
  assert.match(formHtml, /dns-caa-flags/);
  assert.match(formHtml, /dns-caa-tag/);
  assert.match(formHtml, /dns-caa-value/);
  const deleteHtml = vm.runInContext("STATE.dnsDeleteRecord = { id: 'rec_1', type: 'A', name: 'example.com', content: '192.0.2.1' }; STATE.dnsDeleteName = ''; renderDNSDeleteDialog()", context);
  assert.match(deleteHtml, /dns-delete-name/);
  assert.match(deleteHtml, /disabled/);
});
