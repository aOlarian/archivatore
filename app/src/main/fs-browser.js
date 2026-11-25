/* MAP
 * §1 Drives & quick pins    L8-L54
 * §2 Directory listing      L56-L120
 * §3 Folder ops & reveal    L122-L148
 */

// §1 Drives & quick pins
const fs = require('fs');
const os = require('os');
const path = require('path');
const { shell } = require('electron');
const { FS_STAT_BATCH, isArchiveFile } = require('./config');

async function listDrives() {
  const letters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
  const checks = [];
  for (const ch of letters) {
    const root = `${ch}:\\`;
    checks.push(
      fs.promises.statfs(root)
        .then((st) => ({
          path: root,
          label: `${ch}:`,
          freeBytes: Number(st.bavail) * Number(st.bsize),
          totalBytes: Number(st.blocks) * Number(st.bsize),
        }))
        .catch(() => null),
    );
  }
  const drives = (await Promise.all(checks)).filter(Boolean);
  const home = os.homedir();
  const candidates = [
    { label: 'Desktop', path: path.join(home, 'Desktop') },
    { label: 'Downloads', path: path.join(home, 'Downloads') },
    { label: 'Documents', path: path.join(home, 'Documents') },
    { label: 'Home', path: home },
  ];
  const pins = [];
  for (const c of candidates) {
    try {
      const st = await fs.promises.stat(c.path);
      if (st.isDirectory()) pins.push(c);
    } catch (_) {}
  }
  return { drives, pins };
}

// §2 Directory listing
async function listDirectory(dirPath, showHidden = false) {
  const target = path.resolve(dirPath || os.homedir());
  const dirents = await fs.promises.readdir(target, { withFileTypes: true });
  const items = [];

  for (let i = 0; i < dirents.length; i += FS_STAT_BATCH) {
    const slice = dirents.slice(i, i + FS_STAT_BATCH);
    const resolved = await Promise.all(
      slice.map(async (d) => {
        const name = d.name;
        if (!showHidden && name.startsWith('.')) return null;
        if (!showHidden && (name === '$RECYCLE.BIN' || name === 'System Volume Information')) return null;
        const fullPath = path.join(target, name);
        let isDir = d.isDirectory();
        let size = 0;
        let mtimeMs = 0;
        let inaccessible = false;
        try {
          const st = await fs.promises.lstat(fullPath);
          if (st.isSymbolicLink()) {
            try {
              const realSt = await fs.promises.stat(fullPath);
              isDir = realSt.isDirectory();
              size = isDir ? 0 : realSt.size;
              mtimeMs = realSt.mtimeMs;
            } catch (_) {
              inaccessible = true;
            }
          } else {
            isDir = st.isDirectory();
            size = isDir ? 0 : st.size;
            mtimeMs = st.mtimeMs;
          }
        } catch (_) {
          inaccessible = true;
        }
        if (inaccessible) return null;
        const ext = isDir ? '' : path.extname(name).toLowerCase();
        return {
          name,
          path: fullPath,
          isDir,
          isArchive: !isDir && isArchiveFile(name),
          ext,
          size,
          mtimeMs,
        };
      }),
    );
    for (const r of resolved) {
      if (r) items.push(r);
    }
  }

  items.sort((a, b) => {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
    return a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' });
  });

  const parent = path.dirname(target);
  return {
    path: target,
    parent: parent !== target ? parent : null,
    items,
  };
}

// §3 Folder ops & reveal
async function createFolder(parentDir, folderName) {
  const clean = String(folderName || '').trim();
  if (!clean || /[\\/:*?"<>|]/.test(clean)) {
    throw new Error('Nume de folder invalid.');
  }
  const dest = path.join(path.resolve(parentDir), clean);
  await fs.promises.mkdir(dest, { recursive: false });
  return dest;
}

function revealInExplorer(targetPath) {
  if (!targetPath) return false;
  if (fs.existsSync(targetPath)) {
    shell.showItemInFolder(path.resolve(targetPath));
    return true;
  }
  const parent = path.dirname(targetPath);
  if (fs.existsSync(parent)) {
    shell.openPath(parent);
    return true;
  }
  return false;
}

module.exports = { listDrives, listDirectory, createFolder, revealInExplorer };
