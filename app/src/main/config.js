/* MAP
 * §1 Constants & limits      L7-L23
 * §2 Format & level catalog  L25-L62
 * §3 Path & engine resolver  L64-L88
 */

// §1 Constants & limits
const os = require('os');
const path = require('path');

const CPU_COUNT = Math.max(1, os.cpus().length);
const MIN_CORES = 1;
const DEFAULT_TOTAL_CORES = Math.min(CPU_COUNT, Math.max(8, Math.min(16, CPU_COUNT)));
const DEFAULT_PARALLEL_JOBS = 8;
const MAX_PARALLEL_JOBS = 32;
const QUEUE_BROADCAST_MS = 100;
const FS_STAT_BATCH = 64;
const SPEED_WINDOW_MS = 1500;

const CONFLICT_POLICIES = ['rename', 'overwrite', 'skip'];
const EXTRACT_MODES = ['smart', 'folder', 'direct'];
const EXEC_MODES = ['auto', 'parallel', 'sequential'];

// §2 Format & level catalog
const FORMAT_CATALOG = [
  { id: 'capsule-tar', ext: '.tar', label: 'Capsule (.tar · instant, 0%)', supportsLevel: false, multithreaded: true },
  { id: 'capsule-zst', ext: '.tar.zst', label: 'Capsule Fast (.tar.zst · Zstd)', supportsLevel: true, multithreaded: true },
  { id: 'zip', ext: '.zip', label: 'ZIP (.zip · multithreaded)', supportsLevel: true, multithreaded: true },
  { id: '7z', ext: '.7z', label: '7-Zip (.7z · compresie puternică)', supportsLevel: true, multithreaded: true },
  { id: 'rar', ext: '.rar', label: 'RAR5 (.rar · WinRAR)', supportsLevel: true, multithreaded: true },
  { id: 'tar-gz', ext: '.tar.gz', label: 'TAR.GZ (.tar.gz · multi-core)', supportsLevel: true, multithreaded: true },
  { id: 'gz', ext: '.gz', label: 'GZIP (.gz · fișier/tar)', supportsLevel: true, multithreaded: true },
  { id: 'tar-xz', ext: '.tar.xz', label: 'TAR.XZ (.tar.xz · maximă)', supportsLevel: true, multithreaded: true },
  { id: 'tar-bz2', ext: '.tar.bz2', label: 'TAR.BZ2 (.tar.bz2)', supportsLevel: true, multithreaded: false },
];

const COMPRESSION_LEVELS = [
  { value: 0, label: '0 · Store (fără compresie)' },
  { value: 1, label: '1 · Fastest (ultrarapid)' },
  { value: 3, label: '3 · Fast (rapid)' },
  { value: 5, label: '5 · Normal (echilibrat)' },
  { value: 7, label: '7 · High (puternic)' },
  { value: 9, label: '9 · Ultra (spațiu minim)' },
];

const ARCHIVE_SUFFIXES = [
  '.tar.gz', '.tar.zst', '.tar.xz', '.tar.bz2',
  '.tgz', '.tzst', '.txz', '.tbz2',
  '.tar', '.zip', '.7z', '.rar', '.gz', '.zst',
];

function isArchiveFile(name) {
  const lower = String(name || '').toLowerCase();
  if (lower.includes('.archivatore-tmp')) return false;
  return ARCHIVE_SUFFIXES.some((ext) => lower.endsWith(ext));
}

// §3 Path & engine resolver
function resolveEnginePath() {
  if (process.resourcesPath) {
    const packaged = path.join(process.resourcesPath, 'bin', 'archivatore-engine.exe');
    if (require('fs').existsSync(packaged)) return packaged;
  }
  return path.join(__dirname, '..', '..', 'bin', 'archivatore-engine.exe');
}

module.exports = {
  CPU_COUNT,
  MIN_CORES,
  DEFAULT_TOTAL_CORES,
  DEFAULT_PARALLEL_JOBS,
  MAX_PARALLEL_JOBS,
  QUEUE_BROADCAST_MS,
  FS_STAT_BATCH,
  SPEED_WINDOW_MS,
  CONFLICT_POLICIES,
  EXTRACT_MODES,
  EXEC_MODES,
  FORMAT_CATALOG,
  COMPRESSION_LEVELS,
  ARCHIVE_SUFFIXES,
  isArchiveFile,
  resolveEnginePath,
};
