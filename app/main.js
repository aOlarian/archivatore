/* MAP
 * §1 Electron app bootstrap & window  L7-L95
 */

// §1 Electron app bootstrap & window
const path = require('path');
const { app, BrowserWindow, nativeTheme } = require('electron');
const { loadSettings, saveSettings } = require('./src/main/settings');
const { QueueManager } = require('./src/main/queue-manager');
const { registerIpc } = require('./src/main/ipc');
const { showEnterprise } = require('./enterprise.cjs');

const SMOKE = process.env.ARCHIVATORE_SMOKE === '1';
if (process.env.ARCHIVATORE_USER_DATA) {
  app.setPath('userData', process.env.ARCHIVATORE_USER_DATA);
}
let mainWin = null;
let settings = null;
let queue = null;

if (!SMOKE && !app.requestSingleInstanceLock()) {
  app.quit();
}

app.on('second-instance', () => {
  if (mainWin && !mainWin.isDestroyed()) {
    if (mainWin.isMinimized()) mainWin.restore();
    mainWin.focus();
  }
});

function createWindow() {
  mainWin = new BrowserWindow({
    width: 1360,
    height: 860,
    minWidth: 980,
    minHeight: 620,
    title: 'Archivatore',
    icon: path.join(__dirname, 'build', 'icon.png'),
    backgroundColor: '#162224',
    autoHideMenuBar: true,
    show: false,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });

  mainWin.loadFile(path.join(__dirname, 'src', 'renderer', 'index.html'));
  mainWin.once('ready-to-show', () => {
    if (!SMOKE) {
      mainWin.show();
      showEnterprise(mainWin);
    } else {
      mainWin.showInactive();
    }
  });
  mainWin.on('focus', () => {
    if (mainWin && !mainWin.isDestroyed()) mainWin.webContents.focus();
  });
  mainWin.on('closed', () => { mainWin = null; });
}

app.whenReady().then(() => {
  settings = loadSettings();
  if (SMOKE && process.env.ARCHIVATORE_START_DIR) {
    settings.lastPath = process.env.ARCHIVATORE_START_DIR;
  }
  nativeTheme.themeSource = settings.theme || 'system';

  queue = new QueueManager(
    () => settings,
    (snap) => {
      if (mainWin && !mainWin.isDestroyed()) {
        mainWin.webContents.send('queue:update', snap);
      }
    },
    (dirPath) => {
      if (mainWin && !mainWin.isDestroyed()) {
        mainWin.webContents.send('fs:dirChanged', dirPath);
      }
    },
  );

  registerIpc({
    getMainWin: () => mainWin,
    getSettings: () => settings,
    updateSettings: (patch) => {
      settings = saveSettings({ ...settings, ...patch });
      return settings;
    },
    queue,
  });

  createWindow();

  app.on('activate', () => {
    if (!mainWin) createWindow();
  });
});

app.on('before-quit', () => {
  if (queue) queue.shutdown();
});

app.on('window-all-closed', () => {
  app.quit();
});
