/* MAP
 * §1 Engine child process runner & event parser  L5-L95
 */

// §1 Engine child process runner & event parser
const { spawn } = require('child_process');
const { resolveEnginePath } = require('./config');

function spawnEngineJob(job, threads, handlers) {
  const engineExe = resolveEnginePath();
  const args = job.op === 'compress'
    ? [
        'compress', '--source', job.sourcePath, '--output-dir', job.outputDir,
        '--format', job.format, '--level', String(job.level),
        '--threads', String(threads), '--conflict', job.conflictPolicy,
      ]
    : [
        'extract', '--source', job.sourcePath, '--output-dir', job.outputDir,
        '--mode', job.extractMode, '--threads', String(threads),
        '--conflict', job.conflictPolicy,
      ];

  const child = spawn(engineExe, args, { windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'] });
  let stdoutBuf = '';
  let stderrBuf = '';

  const processLine = (raw) => {
    let ev;
    try { ev = JSON.parse(raw); } catch (_) { return; }
    handlers.onEvent(ev);
  };

  child.stdout.on('data', (chunk) => {
    stdoutBuf += chunk.toString('utf8');
    let nl;
    while ((nl = stdoutBuf.indexOf('\n')) !== -1) {
      const line = stdoutBuf.slice(0, nl).trim();
      stdoutBuf = stdoutBuf.slice(nl + 1);
      if (line) processLine(line);
    }
  });

  child.stderr.on('data', (chunk) => {
    stderrBuf += chunk.toString('utf8');
  });

  child.on('error', (err) => handlers.onError(err));
  child.on('close', (code) => {
    if (stdoutBuf.trim()) processLine(stdoutBuf.trim());
    handlers.onClose(code, stderrBuf.trim());
  });

  return child;
}

module.exports = { spawnEngineJob };
