/* MAP
 * §1 Queue rendering & actions  L5-L128
 */

// §1 Queue rendering & actions
window.ArcQueue = (() => {
  const { esc, formatBytes, formatSpeed, formatDuration } = window.ArcUtil;
  let activeTab = 'all';
  let lastSnapshot = null;

  function renderTelemetry(snap) {
    const dot = document.getElementById('act-dot');
    const txt = document.getElementById('act-status');
    const pThreads = document.getElementById('pill-threads');
    const pSpeed = document.getElementById('pill-speed');
    const pCounts = document.getElementById('pill-counts');
    const btnPause = document.getElementById('btn-pause');

    const { running, queued, done, error } = snap.counts;
    dot.className = 'act-dot ' + (snap.paused ? 'paused' : (running > 0 ? 'running' : (error > 0 ? 'error' : '')));
    if (snap.paused) txt.textContent = 'Coadă în pauză';
    else if (running > 0) txt.textContent = `Se procesează ${running} ${running === 1 ? 'lucrare' : 'lucrări'}…`;
    else txt.textContent = 'Pregătit (poți adăuga oricând în coadă)';

    pThreads.textContent = `${snap.activeThreads} / ${snap.cpuCores} nuclee active`;
    pThreads.classList.toggle('active', running > 0);
    pSpeed.textContent = formatSpeed(snap.totalSpeedBps);
    pCounts.textContent = `${queued} în coadă · ${done} gata${error ? ` · ${error} erori` : ''}`;
    btnPause.textContent = snap.paused ? 'Reia coada' : 'Pauză coadă';
  }

  function renderJobCard(j) {
    const pct = j.status === 'done'
      ? 100
      : (j.totalBytes > 0 ? Math.min(99, Math.round((j.doneBytes / j.totalBytes) * 100)) : (j.totalFiles > 0 ? Math.round((j.doneFiles / j.totalFiles) * 100) : 0));

    const opBadge = j.op === 'compress'
      ? `<span class="job-op compress">COMPRIMA</span>`
      : `<span class="job-op extract">EXTRAGE</span>`;

    const fmtTag = j.op === 'compress'
      ? `<span class="job-tag">${esc(j.format)} · L${j.level}</span>`
      : `<span class="job-tag">${esc(j.extractMode)}</span>`;

    const thrTag = j.threads ? `<span class="job-tag">${j.threads} nuclee</span>` : '';

    let meta = '';
    if (j.status === 'queued') {
      meta = 'În așteptare…';
    } else if (j.status === 'running') {
      const filesPart = j.totalFiles ? `${j.doneFiles}/${j.totalFiles} fișiere · ` : '';
      const bytesPart = j.totalBytes ? `${formatBytes(j.doneBytes)} / ${formatBytes(j.totalBytes)}` : formatBytes(j.doneBytes);
      meta = `${filesPart}${bytesPart} · ${formatSpeed(j.speedBps)}${j.currentFile ? ` · ${esc(j.currentFile)}` : ''}`;
    } else if (j.status === 'done') {
      if (j.skipped) {
        meta = `Sărit (există deja) · ${formatDuration(j.elapsedMs)}`;
      } else {
        const ratio = j.op === 'compress' && j.inputBytes > 0 && j.outputBytes > 0
          ? Math.round((1 - j.outputBytes / j.inputBytes) * 100)
          : null;
        const ratioHtml = ratio !== null && ratio > 0 ? `<span class="saved-badge">-${ratio}% spațiu</span>` : '';
        meta = `${j.doneFiles} fișiere · ${formatBytes(j.inputBytes)} → ${formatBytes(j.outputBytes)} · ${formatDuration(j.elapsedMs)} ${ratioHtml}`;
      }
    } else {
      meta = j.status === 'canceled' ? 'Anulat' : 'Eșuat';
    }

    const actions = [];
    if (j.status === 'queued') {
      actions.push(`<button class="ghost mini-btn" data-act="up" data-id="${j.id}" title="Mută mai sus">↑</button>`);
      actions.push(`<button class="ghost mini-btn" data-act="down" data-id="${j.id}" title="Mută mai jos">↓</button>`);
      actions.push(`<button class="danger mini-btn" data-act="cancel" data-id="${j.id}">Anulează</button>`);
    } else if (j.status === 'running') {
      actions.push(`<button class="danger mini-btn" data-act="cancel" data-id="${j.id}">Oprește</button>`);
    } else {
      if (j.outputPath) {
        actions.push(`<button class="ghost mini-btn" data-act="reveal" data-path="${esc(j.outputPath)}">Arată în Explorer</button>`);
      }
      if (j.status === 'error' || j.status === 'canceled') {
        actions.push(`<button class="mini-btn" data-act="retry" data-id="${j.id}">Reîncearcă</button>`);
      }
      actions.push(`<button class="ghost mini-btn" data-act="remove" data-id="${j.id}">Șterge</button>`);
    }

    const targetShown = j.outputPath || j.outputDir;
    return (
      `<article class="job-card ${esc(j.status)}">` +
        `<div class="job-head">${opBadge}<span class="job-name" title="${esc(j.sourcePath)}">${esc(j.name)}</span>${fmtTag}${thrTag}</div>` +
        `<div class="job-path" title="${esc(targetShown)}">→ ${esc(targetShown)}</div>` +
        `<div class="job-prog-wrap"><div class="job-prog-bar" style="width:${pct}%"></div></div>` +
        (j.error ? `<div class="job-err">${esc(j.error)}</div>` : '') +
        `<div class="job-foot"><span>${meta}</span><span class="spacer"></span>${actions.join('')}</div>` +
      `</article>`
    );
  }

  function render(snap) {
    lastSnapshot = snap;
    renderTelemetry(snap);
    const listEl = document.getElementById('q-list');
    if (!listEl) return;

    const jobs = snap.jobs.filter((j) => {
      if (activeTab === 'all') return true;
      if (activeTab === 'error') return j.status === 'error' || j.status === 'canceled';
      return j.status === activeTab;
    });

    if (!jobs.length) {
      listEl.innerHTML = '<div class="empty-state">Coada este goală. Selectează foldere sau arhive din stânga și apasă Compress All sau Extract All.</div>';
      return;
    }
    listEl.innerHTML = jobs.slice().reverse().map(renderJobCard).join('');
  }

  function init() {
    document.getElementById('q-tabs').addEventListener('click', (e) => {
      const tab = e.target.closest('[data-qtab]');
      if (!tab) return;
      activeTab = tab.dataset.qtab;
      document.querySelectorAll('.q-tab').forEach((b) => b.classList.toggle('active', b === tab));
      if (lastSnapshot) render(lastSnapshot);
    });

    document.getElementById('q-list').addEventListener('click', (e) => {
      const btn = e.target.closest('[data-act]');
      if (!btn) return;
      const act = btn.dataset.act;
      const id = Number(btn.dataset.id);
      if (act === 'cancel') window.archivatore.cancelJob(id);
      else if (act === 'retry') window.archivatore.retryJob(id);
      else if (act === 'remove') window.archivatore.removeJob(id);
      else if (act === 'up') window.archivatore.moveJob(id, -1);
      else if (act === 'down') window.archivatore.moveJob(id, 1);
      else if (act === 'reveal') window.archivatore.reveal(btn.dataset.path);
    });

    document.getElementById('btn-pause').addEventListener('click', () => {
      if (lastSnapshot && lastSnapshot.paused) window.archivatore.resumeQueue();
      else window.archivatore.pauseQueue();
    });
    document.getElementById('btn-retry-all').addEventListener('click', () => window.archivatore.retryAll());
    document.getElementById('btn-clear-done').addEventListener('click', () => window.archivatore.clearDone());
  }

  return { init, render };
})();
