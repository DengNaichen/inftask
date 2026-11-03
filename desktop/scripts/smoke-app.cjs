// This entry is not packaged. Only the synthetic fixture certificate is trusted.
const assert = require('node:assert/strict');
const { X509Certificate } = require('node:crypto');
const { writeFileSync } = require('node:fs');
const path = require('node:path');
const electron = require('electron');
const { app, BrowserWindow, session, Menu } = electron;
const { startDesktop } = require('../runtime.cjs');
const { AUTH_ORIGIN, WEB_PREFERENCES } = require('../policy.cjs');
const [profile, evidence, phase, cloudURL, authOrigin, fingerprint] = process.argv.slice(2);
app.setPath('userData', path.join(profile, 'profile'));
let cloudSession;
const external = [];
const testElectron = { ...electron, shell: { openExternal: async (url) => { external.push(url); } } };

function pinFixture() {
  cloudSession.setCertificateVerifyProc(({ certificate }, callback) => {
    callback(new X509Certificate(certificate.data).fingerprint256 === fingerprint ? 0 : -2);
  });
}

app.whenReady().then(() => {
  cloudSession = session.fromPartition('persist:inftask-cloud');
  if (phase !== 'cloud') pinFixture();
});
startDesktop(testElectron, phase === 'cloud' ? {} : { cloudURL, authOrigin });

const pause = () => new Promise((resolve) => setTimeout(resolve, 100));
async function wait(label, predicate) {
  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    if (await predicate()) return;
    await pause();
  }
  throw new Error('Timed out waiting for ' + label);
}
async function evaluate(window, code) {
  return window.webContents.executeJavaScript(code);
}
async function screenshot(window, name) {
  window.show();
  window.focus();
  await evaluate(window, 'new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))');
  writeFileSync(path.join(evidence, name), (await window.webContents.capturePage()).toPNG());
}
const loaded = (window) => !window.webContents.isLoadingMainFrame();

(async () => {
  await wait('main window', () => BrowserWindow.getAllWindows().length === 1);
  let main = BrowserWindow.getAllWindows()[0];
  if (phase === 'cloud') {
    await wait('cloud UC sign-in page', () => loaded(main) && main.webContents.getURL().startsWith(AUTH_ORIGIN));
    assert.deepEqual(await evaluate(main, '[typeof require, typeof process]'), ['undefined', 'undefined']);
    assert.match(await evaluate(main, 'document.body.innerText'), /sign|log|登录|登入/i);
    await screenshot(main, 'cloud-login.png');
    console.log('PASS: live cloud redirects to the expected UC origin; login page renders without Node access. No credentials submitted.');
  } else {
    if (phase === 'first') {
      await wait('fixture UC redirect', () => loaded(main) && main.webContents.getURL().startsWith(authOrigin));
      await evaluate(main, "document.querySelector('#sign-in').click()");
    }
    await wait('fixture signed-in page', async () => loaded(main) &&
      main.webContents.getURL() === cloudURL && await evaluate(main, "Boolean(document.querySelector('#signed-in'))"));
    assert.deepEqual(await evaluate(main, '[typeof require, typeof process]'), ['undefined', 'undefined']);
    assert.equal(await evaluate(main, 'document.cookie'), '');
    assert.equal((await cloudSession.cookies.get({ name: 'fixture_session' })).length, 1);

    await evaluate(main, "document.querySelector('#draft').value = 'Unsaved synthetic draft'; window.open('/auth/login', '_blank'); null");
    await wait('same-session login popup', () => BrowserWindow.getAllWindows().length === 2);
    const child = BrowserWindow.getAllWindows().find((window) => window !== main);
    await wait('popup UC page', () => loaded(child) && child.webContents.getURL().startsWith(authOrigin));
    assert.deepEqual(await evaluate(child, '[typeof require, typeof process]'), ['undefined', 'undefined']);
    assert.equal(await evaluate(main, "document.querySelector('#draft').value"), 'Unsaved synthetic draft');
    child.close();
    await evaluate(main, "document.querySelector('#external').click()");
    await wait('external link intercepted', () => external.length === 1);
    assert.equal(external[0], 'https://github.com/DengNaichen/inftask');
    assert.equal(main.webContents.getURL(), cloudURL);

    await main.loadURL(cloudURL + 'bad-redirect').catch(() => {});
    assert.ok(!main.webContents.getURL().startsWith('https://evil.invalid'));
    await main.loadURL(cloudURL);
    await screenshot(main, phase + '-fixture.png');
    const closed = new Promise((resolve) => main.once('closed', resolve));
    main.close();
    await closed;
    assert.equal(BrowserWindow.getAllWindows().length, 0);
    app.emit('activate');
    await wait('Dock reopen', () => BrowserWindow.getAllWindows().length === 1);
    main = BrowserWindow.getAllWindows()[0];
    await wait('reopened signed-in page', () => loaded(main) && main.webContents.getURL() === cloudURL);

    await main.loadURL(cloudURL + 'unavailable').catch(() => {});
    await wait('HTTP 503 recovery page', () => loaded(main) && main.webContents.getURL().endsWith('/error.html'));
    await main.loadURL(cloudURL);

    // A fresh session uses Chromium defaults and must reject this self-signed certificate.
    const tlsProbe = new BrowserWindow({ show: false,
      webPreferences: { ...WEB_PREFERENCES, session: session.fromPartition('tls-probe') } });
    await assert.rejects(tlsProbe.loadURL(cloudURL), /ERR_CERT_AUTHORITY_INVALID/);
    tlsProbe.destroy();
    await cloudSession.fetch(authOrigin + '/stop-cloud');
    await cloudSession.closeAllConnections();
    await main.loadURL(cloudURL).catch(() => {});
    await wait('offline recovery page', () => loaded(main) && main.webContents.getURL().endsWith('/error.html'));
    assert.match(await evaluate(main, 'document.body.innerText'), /Unable to connect/);
    await screenshot(main, 'connection-error.png');
    await cloudSession.fetch(authOrigin + '/start-cloud');
    const view = Menu.getApplicationMenu().items.find((item) => item.label === 'View');
    view.submenu.items.find((item) => item.label === 'Reload').click(undefined, main);
    await wait('successful retry', () => loaded(main) && main.webContents.getURL() === cloudURL);
    assert.equal(await evaluate(main, "Boolean(document.querySelector('#signed-in'))"), true);
    await cloudSession.cookies.flushStore();
    console.log('PASS: ' + phase + ' fixture process');
  }
  app.quit();
})().catch((error) => {
  console.error(error.stack);
  app.exit(1);
});
