/* MAP
 * §1 Defaults & persistence  L7-L68
 */

// §1 Defaults & persistence
const fs = require('fs');
const path = require('path');
const { app } = require('electron');
const {
  CPU_COUNT,
  DEFAULT_TOTAL_CORES,
  DEFAULT_PARALLEL_JOBS,
  MAX_PARALLEL_JOBS,
  CONFLICT_POLICIES,
  EXTRACT_MODES,
  EXEC_MODES,
} = require('./config');

function defaults() {
  return {
    theme: 'system',
    format: 'capsule-tar',
    level: 5,
    execMode: 'auto',
    parallelJobs: DEFAULT_PARALLEL_JOBS,
    totalCores: DEFAULT_TOTAL_CORES,
    conflictPolicy: 'rename',
    extractMode: 'smart',
    outputDir: '',
    lastPath: require('os').homedir(),
    splitRatio: 0.56,
    showHidden: false,
  };
}

function getSettingsPath() {
  return path.join(app.getPath('userData'), 'settings.json');
}

function clampSettings(raw) {
  const d = defaults();
  const s = Object.assign({}, d, raw || {});
  if (!['system', 'light', 'dark'].includes(s.theme)) s.theme = 'system';
  if (!EXEC_MODES.includes(s.execMode)) s.execMode = 'auto';
  if (!CONFLICT_POLICIES.includes(s.conflictPolicy)) s.conflictPolicy = 'rename';
  if (!EXTRACT_MODES.includes(s.extractMode)) s.extractMode = 'smart';
  s.parallelJobs = Math.max(1, Math.min(MAX_PARALLEL_JOBS, Number(s.parallelJobs) || DEFAULT_PARALLEL_JOBS));
  s.totalCores = Math.max(1, Math.min(CPU_COUNT, Number(s.totalCores) || DEFAULT_TOTAL_CORES));
  s.level = Math.max(0, Math.min(9, Number.isFinite(Number(s.level)) ? Number(s.level) : 5));
  s.splitRatio = Math.max(0.3, Math.min(0.8, Number(s.splitRatio) || 0.56));
  s.showHidden = Boolean(s.showHidden);
  s.outputDir = typeof s.outputDir === 'string' ? s.outputDir : '';
  s.lastPath = typeof s.lastPath === 'string' && s.lastPath ? s.lastPath : d.lastPath;
  return s;
}

function loadSettings() {
  try {
    const p = getSettingsPath();
    if (fs.existsSync(p)) {
      return clampSettings(JSON.parse(fs.readFileSync(p, 'utf8')));
    }
  } catch (_) {}
  return defaults();
}

function saveSettings(next) {
  const clamped = clampSettings(next);
  try {
    const p = getSettingsPath();
    fs.mkdirSync(path.dirname(p), { recursive: true });
    fs.writeFileSync(p, JSON.stringify(clamped, null, 2), 'utf8');
  } catch (_) {}
  return clamped;
}

module.exports = { loadSettings, saveSettings, clampSettings };
