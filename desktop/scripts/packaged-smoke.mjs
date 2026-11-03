import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as pause } from 'node:timers/promises';

const desktop = fileURLToPath(new URL('..', import.meta.url));
const profile = mkdtempSync(path.join(tmpdir(), 'inftask-packaged-smoke-'));
const evidence = process.argv[2] ? path.resolve(process.argv[2]) : profile;
mkdirSync(evidence, { recursive: true });
const env = { ...process.env };
delete env.ELECTRON_RUN_AS_NODE;
const child = spawn(path.join(desktop, 'dist/mac-arm64/InfTask.app/Contents/MacOS/InfTask'),
  ['--user-data-dir=' + profile, '--remote-debugging-address=127.0.0.1', '--remote-debugging-port=0'],
  { env, stdio: ['ignore', 'pipe', 'pipe'] });
let stderr = '';
let exited = false;
let startError;
child.stderr.on('data', (chunk) => { stderr += chunk; });
child.on('error', (error) => { startError = error; });
child.on('exit', () => { exited = true; });
const exit = new Promise((resolve) => child.on('close', resolve));
let socket;
const pending = new Map();
let id = 0;
async function wait(label, fn) {
  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    if (startError) throw startError;
    if (exited) throw new Error('Packaged app exited while waiting for ' + label);
    const value = await fn();
    if (value) return value;
    await pause(100);
  }
  throw new Error('Timed out waiting for ' + label);
}
function call(method, params = {}) {
  return new Promise((resolve, reject) => {
    const request = ++id;
    const timeout = setTimeout(() => { pending.delete(request); reject(new Error('Timed out waiting for ' + method)); }, 15_000);
    pending.set(request, { resolve, reject, timeout });
    socket.send(JSON.stringify({ id: request, method, params }));
  });
}
try {
  const port = await wait('packaged DevTools endpoint', () => stderr.match(/DevTools listening on ws:\/\/127\.0\.0\.1:(\d+)/)?.[1]);
  const target = await wait('packaged cloud UC page', async () => {
    const targets = await (await fetch('http://127.0.0.1:' + port + '/json/list',
      { signal: AbortSignal.timeout(5000) })).json();
    return targets.find((item) => item.type === 'page' && item.url.startsWith('https://www.tokensinfinity.com/'));
  });
  socket = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((resolve, reject) => {
    const timeout = setTimeout(() => reject(new Error('Timed out connecting to packaged DevTools')), 10_000);
    socket.addEventListener('open', () => { clearTimeout(timeout); resolve(); }, { once: true });
    socket.addEventListener('error', (error) => { clearTimeout(timeout); reject(error); }, { once: true });
  });
  socket.addEventListener('message', ({ data }) => {
    const message = JSON.parse(data);
    const request = pending.get(message.id);
    if (!request) return;
    clearTimeout(request.timeout);
    pending.delete(message.id);
    if (message.error) request.reject(new Error(message.error.message));
    else request.resolve(message.result);
  });
  await call('Runtime.enable');
  await wait('packaged login form rendering', async () => {
    const result = await call('Runtime.evaluate', { expression: "document.querySelector('input[type=password]') !== null", returnByValue: true });
    return result.result.value;
  });
  const result = await call('Runtime.evaluate', { expression: '[typeof require, typeof process]', returnByValue: true });
  assert.deepEqual(result.result.value, ['undefined', 'undefined']);
  await call('Runtime.evaluate', { expression: 'new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))', awaitPromise: true });
  const screenshot = await call('Page.captureScreenshot', { format: 'png' });
  writeFileSync(path.join(evidence, 'packaged-cloud-login.png'), Buffer.from(screenshot.data, 'base64'));
  console.log('PASS: packaged arm64 .app starts, reaches live UC login and exposes no Node globals; no credentials submitted.');
} finally {
  socket?.close();
  for (const request of pending.values()) clearTimeout(request.timeout);
  if (!exited) {
    child.kill('SIGTERM');
    const kill = setTimeout(() => child.kill('SIGKILL'), 5000);
    await exit;
    clearTimeout(kill);
  }
  rmSync(profile, { recursive: true });
}
