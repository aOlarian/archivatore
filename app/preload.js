/* MAP
 * §1 Context bridge IPC surface  L5-L35
 */

// §1 Context bridge IPC surface
const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('archivatore', {
  probeTools: () => ipcRenderer.invoke('tools:probe'),
  getSettings: () => ipcRenderer.invoke('settings:get'),
  setSettings: (patch) => ipcRenderer.invoke('settings:set', patch),
  listDrives: () => ipcRenderer.invoke('fs:drives'),
  listDir: (dirPath, showHidden) => ipcRenderer.invoke('fs:list', dirPath, showHidden),
  mkdir: (parentDir, name) => ipcRenderer.invoke('fs:mkdir', parentDir, name),
  reveal: (targetPath) => ipcRenderer.invoke('fs:reveal', targetPath),
  pickDir: (defaultPath) => ipcRenderer.invoke('dialog:pickDir', defaultPath),
  queueSnapshot: () => ipcRenderer.invoke('queue:snapshot'),
  enqueueCompress: (items, opts) => ipcRenderer.invoke('queue:enqueueCompress', items, opts),
  enqueueExtract: (items, opts) => ipcRenderer.invoke('queue:enqueueExtract', items, opts),
  pauseQueue: () => ipcRenderer.invoke('queue:pause'),
  resumeQueue: () => ipcRenderer.invoke('queue:resume'),
  cancelJob: (id) => ipcRenderer.invoke('queue:cancel', id),
  retryJob: (id) => ipcRenderer.invoke('queue:retry', id),
  retryAll: () => ipcRenderer.invoke('queue:retryAll'),
  removeJob: (id) => ipcRenderer.invoke('queue:remove', id),
  clearDone: () => ipcRenderer.invoke('queue:clearDone'),
  moveJob: (id, dir) => ipcRenderer.invoke('queue:move', id, dir),
  onQueueUpdate: (cb) => ipcRenderer.on('queue:update', (_, snap) => cb(snap)),
  onDirChanged: (cb) => ipcRenderer.on('fs:dirChanged', (_, dirPath) => cb(dirPath)),
});
