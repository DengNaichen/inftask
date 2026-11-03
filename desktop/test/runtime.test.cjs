const assert = require('node:assert/strict');
const { EventEmitter } = require('node:events');
const { test } = require('node:test');
const { startDesktop } = require('../runtime.cjs');
const { CLOUD_URL, AUTH_ORIGIN, WEB_PREFERENCES, navigationPolicy } = require('../policy.cjs');

function harness(lock = true) {
  const windows = [];
  const external = [];
  const errors = [];
  const app = new EventEmitter();
  app.setName = (name) => { app.name = name; };
  app.enableSandbox = () => { app.sandboxed = true; };
  app.requestSingleInstanceLock = () => lock;
  app.quit = () => { app.quitting = true; };
  app.whenReady = () => Promise.resolve();
  const cloudSession = new EventEmitter();
  cloudSession.setPermissionRequestHandler = (handler) => { cloudSession.request = handler; };
  cloudSession.setPermissionCheckHandler = (handler) => { cloudSession.check = handler; };
  cloudSession.setDevicePermissionHandler = (handler) => { cloudSession.device = handler; };
  cloudSession.webRequest = { onBeforeRequest: (handler) => { cloudSession.filter = handler; } };
  class BrowserWindow extends EventEmitter {
    constructor(options) {
      super();
      this.options = options;
      this.webContents = new EventEmitter();
      this.webContents.setWindowOpenHandler = (handler) => { this.popup = handler; };
      this.webContents.getURL = () => this.url;
      this.webContents.reload = () => { this.reloaded = true; };
      windows.push(this);
    }
    loadURL(url) { this.url = url; return Promise.resolve(); }
    loadFile(file) { this.file = file; this.url = new URL(`file://${file}`).href; return Promise.resolve(); }
    isMinimized() { return Boolean(this.minimized); }
    restore() { this.minimized = false; }
    show() { this.shown = true; }
    focus() { this.focused = true; }
  }
  const Menu = {
    buildFromTemplate: (template) => template,
    setApplicationMenu: (template) => { Menu.template = template; },
  };
  const electron = { app, BrowserWindow, Menu,
    session: { fromPartition: (partition) => { cloudSession.partition = partition; return cloudSession; } },
    shell: { openExternal: async (url) => { external.push(url); } },
    dialog: { showErrorBox: (...args) => errors.push(args) },
  };
  return { electron, app, windows, external, errors, cloudSession, Menu };
}

async function ready() { await new Promise(setImmediate); }
function event(url, isMainFrame = true) {
  return { url, isMainFrame, prevented: false, preventDefault() { this.prevented = true; } };
}

test('navigation uses exact HTTPS origins, never suffix matching', () => {
  const classify = navigationPolicy();
  for (const url of [CLOUD_URL, `${CLOUD_URL}default/issue/AL-1`, `${AUTH_ORIGIN}/user_center/v1/oauth2/authorize?state=fake`]) {
    assert.equal(classify(url), 'internal');
  }
  for (const url of ['https://github.com/DengNaichen/inftask', 'http://harnessagent.tokensinfinity.com/',
    'https://harnessagent.tokensinfinity.com.evil.test/', 'https://www.tokensinfinity.com:444/']) {
    assert.equal(classify(url), 'external');
  }
  for (const url of ['not a URL', 'javascript:alert(1)', 'file:///etc/passwd', 'data:text/html,bad',
    'inftask://anything', 'mailto:user@example.test', 'https://user:secret@github.com/']) {
    assert.equal(classify(url), 'blocked');
  }
});

test('boots one sandboxed window with the persistent cloud session and no preload', async () => {
  const h = harness();
  startDesktop(h.electron);
  await ready();
  assert.equal(h.app.name, 'InfTask');
  assert.equal(h.app.sandboxed, true);
  assert.equal(h.windows.length, 1);
  const window = h.windows[0];
  assert.equal(window.url, CLOUD_URL);
  assert.equal(h.cloudSession.partition, 'persist:inftask-cloud');
  assert.deepEqual(window.options.webPreferences, { ...WEB_PREFERENCES, session: h.cloudSession });
  assert.equal(window.options.webPreferences.preload, undefined);
  assert.equal(window.options.webPreferences.nodeIntegration, false);
  assert.equal(window.options.webPreferences.contextIsolation, true);
  assert.equal(window.options.webPreferences.sandbox, true);
  assert.equal(window.options.webPreferences.webSecurity, true);
});

test('second process quits without creating a window', async () => {
  const h = harness(false);
  startDesktop(h.electron);
  await ready();
  assert.equal(h.app.quitting, true);
  assert.equal(h.windows.length, 0);
});

test('Dock activation and second-instance focus or recreate the main window', async () => {
  const h = harness();
  startDesktop(h.electron);
  await ready();
  h.windows[0].minimized = true;
  h.app.emit('second-instance');
  assert.equal(h.windows[0].minimized, false);
  assert.equal(h.windows[0].focused, true);
  h.windows[0].emit('closed');
  h.app.emit('activate');
  assert.equal(h.windows.length, 2);
  assert.equal(h.windows[1].url, CLOUD_URL);
});

test('external main-frame links go to the browser; external frames and redirects do not', async () => {
  const h = harness();
  startDesktop(h.electron);
  await ready();
  const contents = h.windows[0].webContents;
  const link = event('https://github.com/DengNaichen/inftask');
  contents.emit('will-frame-navigate', link);
  assert.equal(link.prevented, true);
  assert.deepEqual(h.external, [link.url]);
  const frame = event('https://evil.test/', false);
  const redirect = event('https://evil.test/redirect');
  contents.emit('will-frame-navigate', frame);
  contents.emit('will-redirect', redirect);
  assert.equal(frame.prevented, true);
  assert.equal(redirect.prevented, true);
  assert.equal(h.external.length, 1);
  const auth = event(`${AUTH_ORIGIN}/login`);
  contents.emit('will-frame-navigate', auth);
  contents.emit('will-redirect', auth);
  assert.equal(auth.prevented, false);
  const file = event('file:///etc/passwd');
  contents.emit('will-frame-navigate', file);
  assert.equal(file.prevented, true);
  assert.equal(h.external.length, 1);
});

test('external and custom-protocol popups cannot create Electron windows', async () => {
  const h = harness();
  startDesktop(h.electron);
  await ready();
  assert.deepEqual(h.windows[0].popup({ url: 'https://github.com/' }), { action: 'deny' });
  assert.deepEqual(h.windows[0].popup({ url: 'file:///etc/passwd' }), { action: 'deny' });
  assert.deepEqual(h.windows[0].popup({ url: 'some-app://run' }), { action: 'deny' });
  assert.deepEqual(h.external, ['https://github.com/']);
});

test('reauthentication popups share only the hardened session and get navigation guards', async () => {
  const h = harness();
  startDesktop(h.electron);
  await ready();
  const parent = h.windows[0];
  const result = parent.popup({ url: `${CLOUD_URL}auth/login` });
  assert.equal(result.action, 'allow');
  assert.deepEqual(result.overrideBrowserWindowOptions.webPreferences, { ...WEB_PREFERENCES, session: h.cloudSession });
  const child = new h.electron.BrowserWindow(result.overrideBrowserWindowOptions);
  parent.webContents.emit('did-create-window', child);
  const redirect = event('https://evil.test/');
  child.webContents.emit('will-redirect', redirect);
  assert.equal(redirect.prevented, true);
  assert.ok(child.popup);
});

test('permissions, devices, webviews and downloads are denied', async () => {
  const h = harness();
  startDesktop(h.electron);
  await ready();
  h.cloudSession.request(null, 'media', (allowed) => assert.equal(allowed, false));
  assert.equal(h.cloudSession.check(), false);
  assert.equal(h.cloudSession.device(), false);
  const download = event();
  h.cloudSession.emit('will-download', download);
  assert.equal(download.prevented, true);
  const webview = event();
  h.windows[0].webContents.emit('will-attach-webview', webview);
  assert.equal(webview.prevented, true);
});

test('programmatic document requests cannot bypass origin restrictions', async () => {
  const h = harness();
  startDesktop(h.electron);
  await ready();
  const request = (url, resourceType) => {
    let decision;
    h.cloudSession.filter({ url, resourceType }, (result) => { decision = result.cancel; });
    return decision;
  };
  assert.equal(request(CLOUD_URL, 'mainFrame'), false);
  assert.equal(request(`${AUTH_ORIGIN}/login`, 'mainFrame'), false);
  assert.equal(request('https://evil.test/', 'mainFrame'), true);
  assert.equal(request('https://evil.test/', 'subFrame'), true);
  assert.equal(request('file:///etc/passwd', 'mainFrame'), true);
  assert.equal(request('https://cdn.example.test/image.png', 'image'), false);
  assert.equal(request('https://harnessagent.tokensinfinity.com/api/workspaces', 'xhr'), false);
});

test('failed main load and renderer crashes show recovery; aborted/subframe loads do not', async () => {
  const h = harness();
  startDesktop(h.electron);
  await ready();
  const window = h.windows[0];
  window.webContents.emit('did-fail-load', {}, -3, 'aborted', CLOUD_URL, true);
  window.webContents.emit('did-fail-load', {}, -105, 'DNS', CLOUD_URL, false);
  assert.equal(window.file, undefined);
  window.webContents.emit('did-fail-load', {}, -105, 'DNS', CLOUD_URL, true);
  assert.match(window.file, /error\.html$/);
  const errorURL = window.url;
  let decision;
  h.cloudSession.filter({ url: errorURL, resourceType: 'mainFrame' }, (result) => { decision = result.cancel; });
  assert.equal(decision, false);
  window.webContents.emit('did-fail-load', {}, -6, 'file missing', errorURL, true);
  assert.equal(h.errors.length, 0);
  const reload = h.Menu.template.find((item) => item.label === 'View').submenu[0];
  reload.click(null, window);
  assert.equal(window.url, CLOUD_URL);
  reload.click(null, window);
  assert.equal(window.reloaded, true);
  window.webContents.emit('render-process-gone');
  assert.match(window.file, /error\.html$/);
  reload.click(null, window);
  window.file = undefined;
  window.webContents.emit('did-navigate', {}, CLOUD_URL, 401);
  assert.equal(window.file, undefined);
  window.webContents.emit('did-navigate', {}, CLOUD_URL, 503);
  assert.match(window.file, /error\.html$/);
});

test('production never installs a certificate-verification bypass', async () => {
  const h = harness();
  h.cloudSession.setCertificateVerifyProc = () => assert.fail('TLS validation must use Chromium defaults');
  startDesktop(h.electron);
  await ready();
  assert.equal(h.app.listenerCount('certificate-error'), 0);
});
