const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { CLOUD_URL, AUTH_ORIGIN, WEB_PREFERENCES, navigationPolicy } = require('./policy.cjs');

function startDesktop(electron, { cloudURL = CLOUD_URL, authOrigin = AUTH_ORIGIN } = {}) {
  const { app, BrowserWindow, Menu, session, shell, dialog } = electron;
  const classify = navigationPolicy(cloudURL, authOrigin);
  const errorFile = path.join(__dirname, 'error.html');
  const errorURL = pathToFileURL(errorFile).href;
  let mainWindow;
  let cloudSession;

  app.setName('InfTask');
  app.enableSandbox();
  if (!app.requestSingleInstanceLock()) {
    app.quit();
    return;
  }

  function reportExternalFailure() {
    dialog.showErrorBox('Unable to open link', 'Please open the link in your browser manually.');
  }

  function openExternal(url) {
    // Never hand custom protocols, file URLs or credential-bearing URLs to the OS.
    if (classify(url) === 'external') {
      void shell.openExternal(url).catch(reportExternalFailure);
    }
  }

  function loadCloud(window) {
    // did-fail-load owns the recovery UI; do not log URLs containing OAuth state.
    void window.loadURL(cloudURL).catch(() => {});
  }

  function showRecovery(window) {
    void window.loadFile(errorFile).catch(() => {
      dialog.showErrorBox('Unable to load InfTask', 'Quit and reopen the application.');
    });
  }

  function protectWindow(window) {
    const contents = window.webContents;
    contents.on('will-frame-navigate', (event) => {
      if (classify(event.url) === 'internal') return;
      event.preventDefault();
      if (event.isMainFrame) openExternal(event.url);
    });
    contents.on('will-redirect', (event) => {
      if (classify(event.url) !== 'internal') event.preventDefault();
    });
    contents.on('will-attach-webview', (event) => event.preventDefault());
    contents.setWindowOpenHandler(({ url }) => {
      if (classify(url) !== 'internal') {
        openExternal(url);
        return { action: 'deny' };
      }
      // A same-session login window preserves the original page's unsaved draft.
      return {
        action: 'allow',
        outlivesOpener: false,
        overrideBrowserWindowOptions: {
          width: 1000, height: 760, title: 'InfTask',
          webPreferences: { ...WEB_PREFERENCES, session: cloudSession },
        },
      };
    });
    contents.on('did-create-window', (child) => protectWindow(child));
    contents.on('did-fail-load', (_event, code, _description, url, isMainFrame) => {
      // Aborted loads are expected during redirects and explicit navigation.
      if (!isMainFrame || code === -3 || url === errorURL) return;
      showRecovery(window);
    });
    contents.on('did-navigate', (_event, _url, status) => {
      if (status >= 500) showRecovery(window);
    });
    contents.on('render-process-gone', () => showRecovery(window));
  }

  function showMainWindow() {
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      mainWindow.show();
      mainWindow.focus();
      return;
    }
    mainWindow = new BrowserWindow({
      width: 1280, height: 900, minWidth: 640, minHeight: 480,
      title: 'InfTask', backgroundColor: '#f5f6f8',
      webPreferences: { ...WEB_PREFERENCES, session: cloudSession },
    });
    const window = mainWindow;
    protectWindow(window);
    window.on('closed', () => { mainWindow = null; });
    loadCloud(window);
  }

  function reload(_item, window) {
    if (!window) return;
    if (window.webContents.getURL() === errorURL) loadCloud(window);
    else window.webContents.reload();
  }

  app.on('second-instance', () => {
    if (cloudSession) showMainWindow();
  });
  app.on('activate', () => {
    if (cloudSession) showMainWindow();
  });
  // macOS keeps the app in the Dock after closing its last window.
  app.on('window-all-closed', () => {
    if (process.platform !== 'darwin') app.quit();
  });
  app.whenReady().then(() => {
    cloudSession = session.fromPartition('persist:inftask-cloud');
    cloudSession.setPermissionRequestHandler((_contents, _permission, callback) => callback(false));
    cloudSession.setPermissionCheckHandler(() => false);
    cloudSession.setDevicePermissionHandler(() => false);
    cloudSession.on('will-download', (event) => event.preventDefault());
    cloudSession.webRequest.onBeforeRequest((details, callback) => {
      const isDocument = details.resourceType === 'mainFrame' || details.resourceType === 'subFrame';
      // loadURL and HTTP redirects are also checked, not just clicked links.
      callback({ cancel: isDocument && details.url !== errorURL && classify(details.url) !== 'internal' });
    });
    Menu.setApplicationMenu(Menu.buildFromTemplate([
      { role: 'appMenu' },
      { role: 'fileMenu' },
      { role: 'editMenu' },
      { label: 'View', submenu: [
        { label: 'Reload', accelerator: 'CmdOrCtrl+R', click: reload },
        { label: 'Open InfTask Home', click: () => { showMainWindow(); loadCloud(mainWindow); } },
        { type: 'separator' },
        { role: 'resetZoom' }, { role: 'zoomIn' }, { role: 'zoomOut' },
        { type: 'separator' }, { role: 'togglefullscreen' },
      ] },
      { role: 'windowMenu' },
    ]));
    showMainWindow();
  }).catch(() => {
    dialog.showErrorBox('Unable to start InfTask', 'Quit and reopen the application.');
    app.quit();
  });
}

module.exports = { startDesktop };
