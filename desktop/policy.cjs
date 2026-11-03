const CLOUD_URL = 'https://harnessagent.tokensinfinity.com/';
const AUTH_ORIGIN = 'https://www.tokensinfinity.com';

// Remote content never gets access to Node or a preload/IPC bridge.
const WEB_PREFERENCES = Object.freeze({
  nodeIntegration: false,
  nodeIntegrationInSubFrames: false,
  nodeIntegrationInWorker: false,
  contextIsolation: true,
  sandbox: true,
  webSecurity: true,
  allowRunningInsecureContent: false,
  webviewTag: false,
  navigateOnDragDrop: false,
});

function navigationPolicy(cloudURL = CLOUD_URL, authOrigin = AUTH_ORIGIN) {
  const origins = new Set([new URL(cloudURL).origin, authOrigin]);
  return (value) => {
    let url;
    try {
      url = new URL(value);
    } catch {
      return 'blocked';
    }
    if (url.username || url.password) return 'blocked';
    if (url.protocol !== 'https:' && url.protocol !== 'http:') return 'blocked';
    if (url.protocol === 'https:' && origins.has(url.origin)) return 'internal';
    return 'external';
  };
}

module.exports = { CLOUD_URL, AUTH_ORIGIN, WEB_PREFERENCES, navigationPolicy };
