/* MAP
 * §1 Electron UI & IPC smoke test  L5-L115
 */

// §1 Electron UI & IPC smoke test
const assert = require('assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { execFile } = require('child_process');

const APP_DIR = path.resolve(__dirname, '..');
const ELECTRON_EXE = path.join(APP_DIR, 'node_modules', 'electron', 'dist', 'electron.exe');

async function run() {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'arc-uismoke-'));
  const userDataDir = path.join(tmp, 'userdata');
  const workDir = path.join(tmp, 'work');
  const sampleDir = path.join(workDir, 'Proiect_Test');
  fs.mkdirSync(sampleDir, { recursive: true });
  fs.mkdirSync(userDataDir, { recursive: true });
  fs.writeFileSync(path.join(sampleDir, 'hello.txt'), 'Archivatore UI smoke payload\n'.repeat(100));

  const driverJs = `
    const { app, BrowserWindow } = require('electron');
    const fs = require('fs');
    require(${JSON.stringify(path.join(APP_DIR, 'main.js'))});
    app.whenReady().then(async () => {
      await new Promise((r) => setTimeout(r, 900));
      const win = BrowserWindow.getAllWindows()[0];
      if (!win) { console.error('NO WINDOW'); app.exit(1); return; }
      const wc = win.webContents;
      const errors = [];
      wc.on('console-message', (_, lvl, msg) => { if (lvl >= 3) errors.push(msg); });

      const res = await wc.executeJavaScript(\`
        (async () => {
          const workDir = ${JSON.stringify(workDir)};
          for (let i = 0; i < 30; i++) {
            if (window.ArcExplorer && window.ArcExplorer.getCurrentPath() === workDir) break;
            await new Promise((r) => setTimeout(r, 100));
          }
          await new Promise((r) => setTimeout(r, 150));
          window.ArcExplorer.selectByKind('folders');
          document.getElementById('btn-compress-all').click();
          for (let i = 0; i < 60; i++) {
            const snap = await window.archivatore.queueSnapshot();
            if (snap.counts.done >= 1 && snap.counts.running === 0) break;
            await new Promise((r) => setTimeout(r, 120));
          }
          await new Promise((r) => setTimeout(r, 250));
          const snap1 = await window.archivatore.queueSnapshot();
          window.ArcExplorer.selectByKind('archives');
          document.getElementById('btn-extract-all').click();
          for (let i = 0; i < 60; i++) {
            const snap = await window.archivatore.queueSnapshot();
            if (snap.counts.done >= 2 && snap.counts.running === 0) break;
            await new Promise((r) => setTimeout(r, 120));
          }
          await new Promise((r) => setTimeout(r, 250));
          const snap2 = await window.archivatore.queueSnapshot();
          document.documentElement.setAttribute('data-theme', 'light');
          return {
            title: document.title,
            currentPath: window.ArcExplorer.getCurrentPath(),
            rows: document.querySelectorAll('#fs-tbody tr.fs-row').length,
            done1: snap1.counts.done,
            done2: snap2.counts.done,
            errors: snap2.counts.error,
          };
        })()
      \`);

      await new Promise((r) => setTimeout(r, 150));
      const lightImg = await wc.capturePage();
      fs.writeFileSync(${JSON.stringify(path.join(__dirname, 'smoke-light.png'))}, lightImg.toPNG());
      await wc.executeJavaScript("document.documentElement.setAttribute('data-theme', 'dark')");
      await new Promise((r) => setTimeout(r, 150));
      const darkImg = await wc.capturePage();
      fs.writeFileSync(${JSON.stringify(path.join(__dirname, 'smoke-dark.png'))}, darkImg.toPNG());

      console.log('UI_SMOKE_RESULT=' + JSON.stringify({ ...res, consoleErrors: errors }));
      if (errors.length) process.exitCode = 1;
      app.quit();
    });
  `;

  const driverPath = path.join(tmp, 'driver.js');
  fs.writeFileSync(driverPath, driverJs);

  try {
    const out = await new Promise((resolve, reject) => {
      execFile(ELECTRON_EXE, [driverPath], {
        env: {
          ...process.env,
          ARCHIVATORE_SMOKE: '1',
          ARCHIVATORE_USER_DATA: userDataDir,
          ARCHIVATORE_START_DIR: workDir,
        },
        windowsHide: true,
        timeout: 30000,
      }, (err, stdout, stderr) => {
        if (err) reject(new Error(`${err.message}\n${stdout}\n${stderr}`));
        else resolve(stdout);
      });
    });

    const line = out.split(/\r?\n/).find((l) => l.startsWith('UI_SMOKE_RESULT='));
    assert.ok(line, `Missing UI_SMOKE_RESULT in output:\n${out}`);
    const parsed = JSON.parse(line.slice('UI_SMOKE_RESULT='.length));
    console.log('UI smoke:', parsed);
    assert.strictEqual(parsed.title, 'Archivatore');
    assert.strictEqual(parsed.currentPath, workDir);
    assert.ok(parsed.rows >= 3, 'Explorer should show folder + tar + extracted folder');
    assert.strictEqual(parsed.done1, 1, 'Compress All via UI should finish 1 job');
    assert.strictEqual(parsed.done2, 2, 'Extract All via UI should finish 2 jobs');
    assert.strictEqual(parsed.errors, 0, 'Zero queue errors');
    assert.strictEqual(parsed.consoleErrors.length, 0, 'Zero console errors');
    assert.ok(fs.existsSync(path.join(workDir, 'Proiect_Test.tar')));
    assert.ok(fs.existsSync(path.join(workDir, 'Proiect_Test-1', 'hello.txt')));
    console.log('PASS ui-smoke.');
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
