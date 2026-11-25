/* MAP
 * §1 build engine Go, icon & NSIS installer  L5-L41
 */

// §1 build engine Go, icon & NSIS installer
const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFileSync, execSync } = require('child_process');

const ROOT = __dirname;
const PROJECT_ROOT = path.dirname(ROOT);
const ENGINE_DIR = path.join(PROJECT_ROOT, 'engine');
const ENGINE_OUT = path.join(ROOT, 'bin', 'archivatore-engine.exe');
const SHARED_NM = process.env.ARCHIVATORE_NODE_MODULES || '';
const LOCAL_NM = path.join(ROOT, 'node_modules');
const OUT = fs.mkdtempSync(path.join(os.tmpdir(), 'archivatore-dist-'));

try {
  fs.mkdirSync(path.dirname(ENGINE_OUT), { recursive: true });
  console.log('[1/3] Compilez motorul Go archivatore-engine.exe…');
  execFileSync('go', ['build', '-ldflags=-s -w', '-o', ENGINE_OUT, '.'], {
    cwd: ENGINE_DIR,
    stdio: 'inherit',
  });

  if (!fs.existsSync(path.join(ROOT, 'build', 'icon.ico'))) {
    execFileSync('python', ['-I', path.join(ROOT, 'build', 'make-icon.py')], { stdio: 'inherit' });
  }
  if (!fs.existsSync(LOCAL_NM)) {
    if (SHARED_NM && fs.existsSync(SHARED_NM)) {
      execSync(`cmd.exe /c mklink /J "${LOCAL_NM}" "${SHARED_NM}"`, { stdio: 'inherit' });
    } else {
      throw new Error('Missing app/node_modules. Run "npm install" inside app/ first.');
    }
  }

  console.log('[2/3] Construiesc kitul instalabil Archivatore (NSIS)…');
  execFileSync(process.execPath, [
    path.join(LOCAL_NM, 'electron-builder', 'cli.js'), '--win', 'nsis', '--x64',
    '-c.win.signAndEditExecutable=false', `-c.directories.output=${OUT}`,
  ], { cwd: ROOT, stdio: ['ignore', 'inherit', 'inherit'] });

  const built = path.join(OUT, 'archivatore-install.exe');
  if (!fs.existsSync(built)) throw new Error('electron-builder nu a produs archivatore-install.exe');
  const rootCopy = path.join(PROJECT_ROOT, 'archivatore-install.exe');
  fs.copyFileSync(built, rootCopy);
  console.log('[3/3 GATA]', rootCopy, `(${Math.round(fs.statSync(rootCopy).size / 1048576)} MB)`);
} finally {
  fs.rmSync(OUT, { recursive: true, force: true });
}
