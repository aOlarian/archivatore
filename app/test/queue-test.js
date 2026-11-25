/* MAP
 * §1 Parallel queue & dynamic enqueue test  L5-L96
 */

// §1 Parallel queue & dynamic enqueue test
const assert = require('assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { QueueManager } = require('../src/main/queue-manager');

async function waitUntil(pred, timeoutMs = 30000) {
  const start = Date.now();
  while (!pred()) {
    if (Date.now() - start > timeoutMs) throw new Error('Timeout waiting for queue');
    await new Promise((r) => setTimeout(r, 50));
  }
}

async function run() {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'arc-qtest-'));
  try {
    const folders = [];
    for (let i = 0; i < 10; i++) {
      const dir = path.join(tmp, `Folder_${i}_ăâî`);
      fs.mkdirSync(dir, { recursive: true });
      for (let f = 0; f < 15; f++) {
        fs.writeFileSync(path.join(dir, `f_${f}.txt`), `payload-${i}-${f}-`.repeat(200));
      }
      folders.push(dir);
    }

    let settings = {
      execMode: 'parallel',
      parallelJobs: 8,
      totalCores: 16,
      format: 'capsule-tar',
      level: 0,
      conflictPolicy: 'rename',
      extractMode: 'smart',
      outputDir: '',
    };

    const dirChanges = [];
    const qm = new QueueManager(
      () => settings,
      () => {},
      (d) => dirChanges.push(d),
    );

    // 1) Enqueue first 8 folders in parallel
    qm.enqueueCompress(folders.slice(0, 8), { format: 'zip', level: 3 });
    const midSnap = qm.snapshot();
    assert.ok(midSnap.counts.running >= 1 && midSnap.counts.running <= 8, 'parallel jobs should start immediately');

    // 2) Dynamically add 2 more folders mid-flight while running
    qm.enqueueCompress(folders.slice(8), { format: 'capsule-tar', level: 0 });
    assert.strictEqual(qm.snapshot().counts.total, 10, 'should have 10 queued/running jobs');

    await waitUntil(() => {
      const s = qm.snapshot();
      return s.counts.running === 0 && s.counts.queued === 0;
    });

    const afterComp = qm.snapshot();
    assert.strictEqual(afterComp.counts.done, 10, `expected 10 done, got ${JSON.stringify(afterComp.counts)}`);
    assert.strictEqual(afterComp.counts.error, 0);

    // Verify archives exist on disk
    const archives = [];
    for (let i = 0; i < 8; i++) {
      const p = path.join(tmp, `Folder_${i}_ăâî.zip`);
      assert.ok(fs.existsSync(p), `missing ${p}`);
      archives.push(p);
    }
    for (let i = 8; i < 10; i++) {
      const p = path.join(tmp, `Folder_${i}_ăâî.tar`);
      assert.ok(fs.existsSync(p), `missing ${p}`);
      archives.push(p);
    }

    // 3) Enqueue extract all 10 in-place with rename (should create Folder_i_ăâî-1)
    qm.enqueueExtract(archives, { extractMode: 'smart', conflictPolicy: 'rename' });
    await waitUntil(() => {
      const s = qm.snapshot();
      return s.counts.running === 0 && s.counts.queued === 0;
    });

    const afterExt = qm.snapshot();
    assert.strictEqual(afterExt.counts.done, 20, 'expected 20 total done jobs');
    assert.strictEqual(afterExt.counts.error, 0);
    for (let i = 0; i < 10; i++) {
      const extFolder = path.join(tmp, `Folder_${i}_ăâî-1`, 'f_0.txt');
      assert.ok(fs.existsSync(extFolder), `missing extracted file ${extFolder}`);
    }

    // 4) Sequential mode check (uses all totalCores)
    settings.execMode = 'sequential';
    settings.totalCores = 8;
    qm.enqueueCompress([folders[0]], { format: 'tar-gz', level: 5, conflictPolicy: 'rename' });
    await waitUntil(() => qm.snapshot().counts.done === 21);
    const seqJob = qm.snapshot().jobs[20];
    assert.strictEqual(seqJob.threads, 8, 'sequential mode should allocate all 8 cores');
    assert.ok(dirChanges.length >= 21, 'dir change notifications emitted');

    console.log('PASS queue-test: 21/21 parallel, mid-flight enqueue, extract & sequential jobs verified.');
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
