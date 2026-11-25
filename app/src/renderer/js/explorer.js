/* MAP
 * §1 Explorer state & navigation  L5-L75
 * §2 Selection & sorting          L77-L148
 * §3 Render drives & table        L150-L220
 */

// §1 Explorer state & navigation
window.ArcExplorer = (() => {
  const { esc, formatBytes, formatDate, showToast } = window.ArcUtil;
  const Icons = window.ArcIcons;

  const state = {
    currentPath: '',
    parentPath: null,
    items: [],
    filtered: [],
    selected: new Set(),
    lastCheckedIdx: -1,
    sortKey: 'name',
    sortAsc: true,
    filterText: '',
    showHidden: false,
    history: [],
    histIdx: -1,
    navSeq: 0,
    onSelectionChange: null,
  };

  function getEls() {
    return {
      drives: document.getElementById('drive-strip'),
      pathInp: document.getElementById('inp-path'),
      filterInp: document.getElementById('inp-filter'),
      tbody: document.getElementById('fs-tbody'),
      chkAll: document.getElementById('chk-all'),
      btnBack: document.getElementById('btn-back'),
      btnFwd: document.getElementById('btn-fwd'),
      btnUp: document.getElementById('btn-up'),
    };
  }

  async function loadDrives() {
    const { drives, pins } = await window.archivatore.listDrives();
    const strip = getEls().drives;
    if (!strip) return;
    let html = '';
    for (const d of drives) {
      const free = formatBytes(d.freeBytes);
      html += `<button class="drive-pill" data-go="${esc(d.path)}" title="${esc(d.path)} (${esc(free)} liber)">${Icons.drive} ${esc(d.label)}</button>`;
    }
    if (pins.length) html += '<span class="drive-sep"></span>';
    for (const p of pins) {
      html += `<button class="pin-pill" data-go="${esc(p.path)}" title="${esc(p.path)}">${esc(p.label)}</button>`;
    }
    strip.innerHTML = html;
    updateDriveHighlight();
  }

  function updateDriveHighlight() {
    const cur = state.currentPath.toLowerCase();
    document.querySelectorAll('.drive-pill').forEach((btn) => {
      const dp = (btn.dataset.go || '').toLowerCase();
      btn.classList.toggle('active', dp && cur.startsWith(dp));
    });
  }

  async function navigate(dirPath, pushHist = true) {
    const seq = ++state.navSeq;
    try {
      const res = await window.archivatore.listDir(dirPath, state.showHidden);
      if (seq !== state.navSeq) return;
      state.currentPath = res.path;
      state.parentPath = res.parent;
      state.items = res.items || [];
      state.selected.clear();
      state.lastCheckedIdx = -1;
      if (pushHist) {
        state.history = state.history.slice(0, state.histIdx + 1);
        if (state.history[state.history.length - 1] !== res.path) {
          state.history.push(res.path);
          state.histIdx = state.history.length - 1;
        }
      }
      const els = getEls();
      els.pathInp.value = res.path;
      els.btnBack.disabled = state.histIdx <= 0;
      els.btnFwd.disabled = state.histIdx >= state.history.length - 1;
      els.btnUp.disabled = !res.parent;
      updateDriveHighlight();
      applyFilterAndSort();
      window.archivatore.setSettings({ lastPath: res.path }).catch(() => {});
    } catch (err) {
      showToast(`Nu pot deschide folderul: ${err.message}`);
    }
  }

  // §2 Selection & sorting
  function applyFilterAndSort() {
    const q = state.filterText.trim().toLowerCase();
    let list = q
      ? state.items.filter((it) => it.name.toLowerCase().includes(q))
      : state.items.slice();

    const dir = state.sortAsc ? 1 : -1;
    list.sort((a, b) => {
      if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
      if (state.sortKey === 'size') return (a.size - b.size) * dir;
      if (state.sortKey === 'mtimeMs') return (a.mtimeMs - b.mtimeMs) * dir;
      if (state.sortKey === 'type') {
        const ta = a.isDir ? '0' : (a.ext || 'z');
        const tb = b.isDir ? '0' : (b.ext || 'z');
        return ta.localeCompare(tb) * dir;
      }
      return a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' }) * dir;
    });
    state.filtered = list;
    renderTable();
    notifySelection();
  }

  function notifySelection() {
    const els = getEls();
    const visiblePaths = state.filtered.map((it) => it.path);
    const checkedVisible = visiblePaths.filter((p) => state.selected.has(p)).length;
    els.chkAll.checked = visiblePaths.length > 0 && checkedVisible === visiblePaths.length;
    els.chkAll.indeterminate = checkedVisible > 0 && checkedVisible < visiblePaths.length;
    if (state.onSelectionChange) {
      const selectedItems = state.items.filter((it) => state.selected.has(it.path));
      state.onSelectionChange(selectedItems);
    }
  }

  function toggleRow(idx, shiftKey) {
    const item = state.filtered[idx];
    if (!item) return;
    const willCheck = !state.selected.has(item.path);
    if (shiftKey && state.lastCheckedIdx >= 0 && state.lastCheckedIdx < state.filtered.length) {
      const lo = Math.min(state.lastCheckedIdx, idx);
      const hi = Math.max(state.lastCheckedIdx, idx);
      for (let i = lo; i <= hi; i++) {
        if (willCheck) state.selected.add(state.filtered[i].path);
        else state.selected.delete(state.filtered[i].path);
      }
    } else {
      if (willCheck) state.selected.add(item.path);
      else state.selected.delete(item.path);
    }
    state.lastCheckedIdx = idx;
    renderTable();
    notifySelection();
  }

  function selectByKind(kind) {
    state.selected.clear();
    for (const it of state.filtered) {
      if (kind === 'all') state.selected.add(it.path);
      else if (kind === 'folders' && it.isDir) state.selected.add(it.path);
      else if (kind === 'archives' && it.isArchive) state.selected.add(it.path);
    }
    renderTable();
    notifySelection();
  }

  // §3 Render drives & table
  function renderTable() {
    const tbody = getEls().tbody;
    if (!tbody) return;
    if (!state.filtered.length) {
      tbody.innerHTML = '<tr><td colspan="5" class="empty-state">Folder gol sau niciun rezultat pentru filtru.</td></tr>';
      return;
    }
    const rows = [];
    for (let i = 0; i < state.filtered.length; i++) {
      const it = state.filtered[i];
      const checked = state.selected.has(it.path);
      const ico = it.isDir ? Icons.folder : (it.isArchive ? Icons.archive : Icons.file);
      const typeLabel = it.isDir ? 'Folder' : (it.isArchive ? '<span class="badge-arch">ARHIVA</span>' : (it.ext ? it.ext.slice(1).toUpperCase() : 'Fișier'));
      const sizeStr = it.isDir ? '—' : formatBytes(it.size);
      rows.push(
        `<tr class="fs-row${checked ? ' checked' : ''}" data-idx="${i}">` +
          `<td class="chk-col"><input class="row-chk" type="checkbox" data-idx="${i}" ${checked ? 'checked' : ''}></td>` +
          `<td><span class="name-cell">${ico}<span title="${esc(it.name)}">${esc(it.name)}</span></span></td>` +
          `<td>${typeLabel}</td>` +
          `<td class="num">${sizeStr}</td>` +
          `<td class="num">${formatDate(it.mtimeMs)}</td>` +
        `</tr>`,
      );
    }
    tbody.innerHTML = rows.join('');
  }

  function init(onSelectionChange) {
    state.onSelectionChange = onSelectionChange;
    const els = getEls();

    els.drives.addEventListener('click', (e) => {
      const btn = e.target.closest('[data-go]');
      if (btn) navigate(btn.dataset.go);
    });
    els.btnBack.addEventListener('click', () => {
      if (state.histIdx > 0) navigate(state.history[--state.histIdx], false);
    });
    els.btnFwd.addEventListener('click', () => {
      if (state.histIdx < state.history.length - 1) navigate(state.history[++state.histIdx], false);
    });
    els.btnUp.addEventListener('click', () => {
      if (state.parentPath) navigate(state.parentPath);
    });
    document.getElementById('btn-refresh').addEventListener('click', () => {
      if (state.currentPath) navigate(state.currentPath, false);
    });
    els.pathInp.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') navigate(els.pathInp.value.trim());
    });
    els.filterInp.addEventListener('input', () => {
      state.filterText = els.filterInp.value;
      applyFilterAndSort();
    });
    els.chkAll.addEventListener('change', () => {
      selectByKind(els.chkAll.checked ? 'all' : 'none');
    });

    document.querySelectorAll('th[data-sort]').forEach((th) => {
      th.addEventListener('click', () => {
        const key = th.dataset.sort;
        if (state.sortKey === key) state.sortAsc = !state.sortAsc;
        else { state.sortKey = key; state.sortAsc = true; }
        applyFilterAndSort();
      });
    });

    els.tbody.addEventListener('click', (e) => {
      const tr = e.target.closest('tr.fs-row');
      if (!tr) return;
      const idx = Number(tr.dataset.idx);
      if (e.target.classList.contains('row-chk') || e.target.closest('td.chk-col')) {
        toggleRow(idx, e.shiftKey);
      } else if (e.shiftKey || e.ctrlKey) {
        toggleRow(idx, e.shiftKey);
      }
    });

    els.tbody.addEventListener('dblclick', (e) => {
      if (e.target.closest('td.chk-col')) return;
      const tr = e.target.closest('tr.fs-row');
      if (!tr) return;
      const item = state.filtered[Number(tr.dataset.idx)];
      if (item && item.isDir) navigate(item.path);
    });
  }

  return {
    init,
    loadDrives,
    navigate,
    selectByKind,
    setShowHidden: (v) => { state.showHidden = Boolean(v); if (state.currentPath) navigate(state.currentPath, false); },
    getCurrentPath: () => state.currentPath,
    getSelectedItems: () => state.items.filter((it) => state.selected.has(it.path)),
    refreshIfSame: (dirPath) => {
      if (dirPath && state.currentPath && dirPath.toLowerCase() === state.currentPath.toLowerCase()) {
        navigate(state.currentPath, false);
      }
    },
  };
})();
