/* MAP
 * §1 Queue state & thread scheduling  L7-L95
 * §2 Enqueue & control actions        L97-L202
 * §3 Pump & event handling            L204-L286
 */

// §1 Queue state & thread scheduling
const path = require('path');
const { CPU_COUNT, QUEUE_BROADCAST_MS, SPEED_WINDOW_MS } = require('./config');
const { spawnEngineJob } = require('./job-runner');

class QueueManager {
  constructor(getSettings, onSnapshot, onDirChanged) {
    this.getSettings = getSettings;
    this.onSnapshot = onSnapshot;
    this.onDirChanged = onDirChanged;
    this.jobs = [];
    this.nextId = 1;
    this.paused = false;
    this.children = new Map();
    this.broadcastTimer = null;
    this.lastBroadcast = 0;
  }

  allocateResources() {
    const s = this.getSettings();
    const totalCores = Math.max(1, Math.min(CPU_COUNT, Number(s.totalCores) || 8));
    const parallelCap = Math.max(1, Number(s.parallelJobs) || 8);
    const active = this.jobs.filter((j) => j.status === 'running');
    const queued = this.jobs.filter((j) => j.status === 'queued');
    const demand = active.length + queued.length;

    if (s.execMode === 'sequential') return { maxSlots: 1, threadsPerJob: totalCores };
    if (s.execMode === 'parallel') {
      return { maxSlots: parallelCap, threadsPerJob: Math.max(1, Math.floor(totalCores / parallelCap)) };
    }
    if (demand <= 1) return { maxSlots: 1, threadsPerJob: totalCores };
    const slots = Math.min(parallelCap, demand);
    return { maxSlots: slots, threadsPerJob: Math.max(1, Math.floor(totalCores / slots)) };
  }

  scheduleBroadcast(force = false) {
    if (force) {
      if (this.broadcastTimer) { clearTimeout(this.broadcastTimer); this.broadcastTimer = null; }
      this.lastBroadcast = Date.now();
      this.onSnapshot(this.snapshot());
      return;
    }
    if (this.broadcastTimer) return;
    const delay = Math.max(0, QUEUE_BROADCAST_MS - (Date.now() - this.lastBroadcast));
    this.broadcastTimer = setTimeout(() => {
      this.broadcastTimer = null;
      this.lastBroadcast = Date.now();
      this.onSnapshot(this.snapshot());
    }, delay);
  }

  snapshot() {
    const now = Date.now();
    let totalSpeedBps = 0;
    let activeThreads = 0;
    const counts = { total: this.jobs.length, queued: 0, running: 0, done: 0, error: 0, canceled: 0 };

    for (const j of this.jobs) {
      if (counts[j.status] !== undefined) counts[j.status]++;
      if (j.status === 'running') {
        activeThreads += j.threads || 0;
        if (j.lastProgressAt && now - j.lastProgressAt <= SPEED_WINDOW_MS * 2) totalSpeedBps += j.speedBps || 0;
      }
    }
    return {
      paused: this.paused,
      cpuCores: CPU_COUNT,
      activeThreads,
      totalSpeedBps,
      counts,
      jobs: this.jobs.map((j) => ({ ...j })),
    };
  }

  // §2 Enqueue & control actions
  makeBaseJob(op, cleanSrc, outDir, conflict) {
    return {
      id: this.nextId++, op, sourcePath: cleanSrc, name: path.basename(cleanSrc),
      outputDir: outDir || path.dirname(cleanSrc), outputPath: '',
      format: 'auto', level: 0, conflictPolicy: conflict, extractMode: 'smart',
      status: 'queued', threads: 0, doneFiles: 0, totalFiles: 0, doneBytes: 0,
      totalBytes: 0, inputBytes: 0, outputBytes: 0, speedBps: 0, elapsedMs: 0,
      currentFile: '', skipped: false, error: '', createdAt: Date.now(), lastProgressAt: 0,
    };
  }

  enqueueCompress(items, opts = {}) {
    const s = this.getSettings();
    const format = opts.format || s.format || 'capsule-tar';
    const level = Number.isFinite(Number(opts.level)) ? Number(opts.level) : s.level;
    const conflict = opts.conflictPolicy || s.conflictPolicy || 'rename';
    const outDir = typeof opts.outputDir === 'string' ? opts.outputDir : (s.outputDir || '');
    const created = [];
    for (const src of items || []) {
      const job = this.makeBaseJob('compress', path.resolve(src), outDir, conflict);
      job.format = format;
      job.level = format === 'capsule-tar' ? 0 : level;
      this.jobs.push(job);
      created.push(job.id);
    }
    this.pump();
    this.scheduleBroadcast(true);
    return created;
  }

  enqueueExtract(items, opts = {}) {
    const s = this.getSettings();
    const mode = opts.extractMode || s.extractMode || 'smart';
    const conflict = opts.conflictPolicy || s.conflictPolicy || 'rename';
    const outDir = typeof opts.outputDir === 'string' ? opts.outputDir : (s.outputDir || '');
    const created = [];
    for (const src of items || []) {
      const job = this.makeBaseJob('extract', path.resolve(src), outDir, conflict);
      job.extractMode = mode;
      this.jobs.push(job);
      created.push(job.id);
    }
    this.pump();
    this.scheduleBroadcast(true);
    return created;
  }

  pause() { this.paused = true; this.scheduleBroadcast(true); }
  resume() { this.paused = false; this.pump(); this.scheduleBroadcast(true); }

  cancelJob(id) {
    const job = this.jobs.find((j) => j.id === id);
    if (!job) return false;
    if (job.status === 'queued') { job.status = 'canceled'; job.error = 'Anulat din coadă'; this.scheduleBroadcast(true); return true; }
    if (job.status === 'running') {
      job.status = 'canceled';
      job.error = 'Anulat de utilizator';
      const child = this.children.get(id);
      if (child) { try { child.kill(); } catch (_) {} }
      this.scheduleBroadcast(true);
      return true;
    }
    return false;
  }

  retryJob(id) {
    const job = this.jobs.find((j) => j.id === id);
    if (!job || (job.status !== 'error' && job.status !== 'canceled')) return false;
    Object.assign(job, {
      status: 'queued', error: '', doneFiles: 0, totalFiles: 0, doneBytes: 0,
      totalBytes: 0, speedBps: 0, currentFile: '', outputPath: '', skipped: false,
    });
    this.pump();
    this.scheduleBroadcast(true);
    return true;
  }

  retryAll() {
    let n = 0;
    for (const j of this.jobs) {
      if (j.status === 'error' || j.status === 'canceled') {
        Object.assign(j, {
          status: 'queued', error: '', doneFiles: 0, totalFiles: 0, doneBytes: 0,
          totalBytes: 0, speedBps: 0, currentFile: '', outputPath: '', skipped: false,
        });
        n++;
      }
    }
    if (n > 0) { this.pump(); this.scheduleBroadcast(true); }
    return n;
  }

  removeJob(id) {
    const idx = this.jobs.findIndex((j) => j.id === id);
    if (idx === -1 || this.jobs[idx].status === 'running') return false;
    this.jobs.splice(idx, 1);
    this.scheduleBroadcast(true);
    return true;
  }

  clearDone() {
    this.jobs = this.jobs.filter((j) => j.status === 'running' || j.status === 'queued');
    this.scheduleBroadcast(true);
  }

  moveJob(id, dir) {
    const idx = this.jobs.findIndex((j) => j.id === id && j.status === 'queued');
    if (idx === -1) return false;
    const swap = idx + (dir < 0 ? -1 : 1);
    if (swap < 0 || swap >= this.jobs.length || this.jobs[swap].status !== 'queued') return false;
    [this.jobs[idx], this.jobs[swap]] = [this.jobs[swap], this.jobs[idx]];
    this.scheduleBroadcast(true);
    return true;
  }

  // §3 Pump & event handling
  pump() {
    if (this.paused) return;
    const { maxSlots, threadsPerJob } = this.allocateResources();
    let activeCount = this.jobs.filter((j) => j.status === 'running').length;
    while (activeCount < maxSlots) {
      const next = this.jobs.find((j) => j.status === 'queued');
      if (!next) break;
      activeCount++;
      this.startJob(next, threadsPerJob);
    }
  }

  startJob(job, threads) {
    job.status = 'running';
    job.threads = threads;
    job.startedAt = Date.now();
    const child = spawnEngineJob(job, threads, {
      onEvent: (ev) => this.handleEvent(job, ev),
      onError: (err) => {
        this.children.delete(job.id);
        if (job.status === 'running') { job.status = 'error'; job.error = err.message; }
        this.pump();
        this.scheduleBroadcast(true);
      },
      onClose: (code, stderrText) => {
        this.children.delete(job.id);
        if (job.status === 'running') {
          if (code === 0) { job.status = 'done'; job.elapsedMs = job.elapsedMs || (Date.now() - job.startedAt); }
          else { job.status = 'error'; job.error = job.error || stderrText || `Motorul a ieșit cu codul ${code}`; }
        }
        if (job.status === 'done' && this.onDirChanged) this.onDirChanged(job.outputDir);
        this.pump();
        this.scheduleBroadcast(true);
      },
    });
    this.children.set(job.id, child);
  }

  handleEvent(job, ev) {
    if (ev.type === 'scan') {
      job.totalFiles = ev.totalFiles || 0;
      job.totalBytes = ev.totalBytes || 0;
      this.scheduleBroadcast(false);
    } else if (ev.type === 'progress') {
      job.doneFiles = ev.doneFiles ?? job.doneFiles;
      if (ev.totalFiles) job.totalFiles = ev.totalFiles;
      job.doneBytes = ev.doneBytes ?? job.doneBytes;
      if (ev.totalBytes) job.totalBytes = ev.totalBytes;
      job.currentFile = ev.currentFile || job.currentFile;
      job.speedBps = ev.speedBps || 0;
      job.lastProgressAt = Date.now();
      this.scheduleBroadcast(false);
    } else if (ev.type === 'done') {
      job.status = 'done';
      job.outputPath = ev.outputPath || '';
      job.inputBytes = ev.inputBytes || 0;
      job.outputBytes = ev.outputBytes || 0;
      job.doneFiles = ev.files || job.doneFiles;
      job.totalFiles = ev.files || job.totalFiles;
      job.elapsedMs = ev.elapsedMs || (Date.now() - job.startedAt);
      job.skipped = Boolean(ev.skipped);
      job.speedBps = 0;
    } else if (ev.type === 'error') {
      job.error = ev.message || 'Eroare necunoscută';
    }
  }

  shutdown() {
    for (const child of this.children.values()) { try { child.kill(); } catch (_) {} }
    this.children.clear();
  }
}

module.exports = { QueueManager };
