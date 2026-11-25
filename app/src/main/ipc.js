/* MAP
 * §1 Probe tools & IPC registration  L8-L96
 */

// §1 Probe tools & IPC registration
const { execFile } = require('child_process');
const { ipcMain, dialog, nativeTheme } = require('electron');
const {
  CPU_COUNT,
  FORMAT_CATALOG,
  COMPRESSION_LEVELS,
  resolveEnginePath,
} = require('./config');
const { listDrives, listDirectory, createFolder, revealInExplorer } = require('./fs-browser');

function probeEngineTools() {
  return new Promise((resolve) => {
    const exe = resolveEnginePath();
    execFile(exe, ['probe-tools'], { windowsHide: true, timeout: 5000 }, (err, stdout) => {
      if (!err && stdout) {
        try {
          const parsed = JSON.parse(stdout.trim());
          return resolve(parsed);
        } catch (_) {}
      }
      resolve({
        cpuCores: CPU_COUNT,
        tools: { tar: '', zstd: '', rar: '', sevenZip: '' },
        formats: FORMAT_CATALOG,
      });
    });
  });
}

function registerIpc({ getMainWin, getSettings, updateSettings, queue }) {
  ipcMain.handle('tools:probe', async () => {
    const probe = await probeEngineTools();
    return {
      ...probe,
      catalog: FORMAT_CATALOG,
      levels: COMPRESSION_LEVELS,
    };
  });

  ipcMain.handle('settings:get', () => getSettings());
  ipcMain.handle('settings:set', (_, patch) => {
    const next = updateSettings(patch);
    nativeTheme.themeSource = next.theme || 'system';
    return next;
  });

  ipcMain.handle('fs:drives', () => listDrives());
  ipcMain.handle('fs:list', (_, dirPath, showHidden) => listDirectory(dirPath, showHidden));
  ipcMain.handle('fs:mkdir', (_, parentDir, name) => createFolder(parentDir, name));
  ipcMain.handle('fs:reveal', (_, targetPath) => revealInExplorer(targetPath));

  ipcMain.handle('dialog:pickDir', async (_, defaultPath) => {
    const win = getMainWin();
    const res = await dialog.showOpenDialog(win, {
      defaultPath: defaultPath || undefined,
      properties: ['openDirectory', 'createDirectory'],
    });
    if (win && !win.isDestroyed()) win.webContents.focus();
    if (res.canceled || !res.filePaths.length) return null;
    return res.filePaths[0];
  });

  ipcMain.handle('queue:snapshot', () => queue.snapshot());
  ipcMain.handle('queue:enqueueCompress', (_, items, opts) => queue.enqueueCompress(items, opts));
  ipcMain.handle('queue:enqueueExtract', (_, items, opts) => queue.enqueueExtract(items, opts));
  ipcMain.handle('queue:pause', () => { queue.pause(); return queue.snapshot(); });
  ipcMain.handle('queue:resume', () => { queue.resume(); return queue.snapshot(); });
  ipcMain.handle('queue:cancel', (_, id) => queue.cancelJob(id));
  ipcMain.handle('queue:retry', (_, id) => queue.retryJob(id));
  ipcMain.handle('queue:retryAll', () => queue.retryAll());
  ipcMain.handle('queue:remove', (_, id) => queue.removeJob(id));
  ipcMain.handle('queue:clearDone', () => { queue.clearDone(); return queue.snapshot(); });
  ipcMain.handle('queue:move', (_, id, dir) => queue.moveJob(id, dir));
}

module.exports = { registerIpc, probeEngineTools };
