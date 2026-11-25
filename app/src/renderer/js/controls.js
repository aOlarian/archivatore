/* MAP
 * §1 Top bar controls, theme, dialogs & splitter  L5-L158
 */

// §1 Top bar controls, theme, dialogs & splitter
window.ArcControls = (() => {
  const { esc, showToast } = window.ArcUtil;

  const THEME_LABELS = { system: 'Temă: Sistem', light: 'Temă: Light', dark: 'Temă: Dark' };
  const THEME_ORDER = ['system', 'light', 'dark'];

  function applyTheme(theme) {
    if (theme === 'light' || theme === 'dark') {
      document.documentElement.setAttribute('data-theme', theme);
    } else {
      document.documentElement.removeAttribute('data-theme');
    }
    const btn = document.getElementById('btn-theme');
    if (btn) btn.textContent = THEME_LABELS[theme] || THEME_LABELS.system;
  }

  function populateFormatsAndLevels(probe, settings) {
    const selFmt = document.getElementById('sel-format');
    const selLvl = document.getElementById('sel-level');
    const formats = probe.formats || probe.catalog || [];

    selFmt.innerHTML = formats.map((f) => {
      const avail = f.canCompress !== false;
      return `<option value="${esc(f.id)}" ${avail ? '' : 'disabled'}>${esc(f.label)}${avail ? '' : ' (lipsește unealta)'}</option>`;
    }).join('');
    selFmt.value = settings.format || 'capsule-tar';

    const levels = probe.levels || [];
    selLvl.innerHTML = levels.map((l) => `<option value="${l.value}">${esc(l.label)}</option>`).join('');
    selLvl.value = String(settings.level ?? 5);

    const syncLevelDisabled = () => {
      const isCapsule = selFmt.value === 'capsule-tar';
      selLvl.disabled = isCapsule;
      if (isCapsule) selLvl.title = 'Capsule (.tar) nu comprimă — viteză maximă';
      else selLvl.title = '';
    };
    syncLevelDisabled();
    selFmt.addEventListener('change', syncLevelDisabled);

    const grid = document.getElementById('tool-grid');
    if (grid && probe.tools) {
      const rows = [
        ['Motor nativ Go (Capsule, ZIP, GZIP)', 'Activ (built-in)'],
        ['Nuclee CPU detectate', `${probe.cpuCores} fire logice`],
        ['Zstd multi-thread (.tar.zst / .zst)', probe.tools.zstd || 'Lipșește (fallback bsdtar)'],
        ['WinRAR Rar.exe (.rar)', probe.tools.rar || 'Neinstalat'],
        ['7-Zip / bsdtar (.7z, .tar.xz, .tar.bz2)', probe.tools.sevenZip || probe.tools.tar || 'Neinstalat'],
      ];
      grid.innerHTML = rows.map(([k, v]) => {
        const ok = !String(v).startsWith('Neinstalat') && !String(v).startsWith('Lip');
        return `<span>${esc(k)}</span><span class="${ok ? 'tool-ok' : 'tool-miss'}">${esc(v)}</span>`;
      }).join('');
    }
  }

  function setupDialogs(state, onRefresh) {
    const dlgMkdir = document.getElementById('dlg-mkdir');
    const inpName = document.getElementById('mkdir-name');
    const errMkdir = document.getElementById('mkdir-err');
    const closeMkdir = () => dlgMkdir.close();

    document.getElementById('btn-new-folder').addEventListener('click', () => {
      inpName.value = '';
      errMkdir.hidden = true;
      dlgMkdir.showModal();
      setTimeout(() => inpName.focus(), 20);
    });
    document.getElementById('mkdir-close').addEventListener('click', closeMkdir);
    document.getElementById('mkdir-cancel').addEventListener('click', closeMkdir);
    document.getElementById('mkdir-form').addEventListener('submit', async (e) => {
      e.preventDefault();
      errMkdir.hidden = true;
      try {
        await window.archivatore.mkdir(window.ArcExplorer.getCurrentPath(), inpName.value);
        closeMkdir();
        onRefresh();
      } catch (err) {
        errMkdir.textContent = err.message;
        errMkdir.hidden = false;
      }
    });

    const dlgSet = document.getElementById('dlg-settings');
    const inpOut = document.getElementById('set-outdir');
    const syncOutLabel = () => {
      const val = state.settings.outputDir || '';
      inpOut.value = val;
      const btnOut = document.getElementById('btn-out-dir');
      btnOut.textContent = val ? `Ieșire: ${val.split(/[\\/]/).pop() || val}` : 'Ieșire: Pe loc';
      btnOut.title = val ? `Destinație fixă: ${val}` : 'Ieșire pe loc, lângă fiecare folder/arhivă';
    };
    syncOutLabel();

    document.getElementById('btn-settings').addEventListener('click', () => dlgSet.showModal());
    document.getElementById('set-close').addEventListener('click', () => dlgSet.close());
    document.getElementById('set-done').addEventListener('click', () => dlgSet.close());

    const pickOutputFolder = async () => {
      const picked = await window.archivatore.pickDir(state.settings.outputDir || window.ArcExplorer.getCurrentPath());
      if (picked !== null) {
        state.settings = await window.archivatore.setSettings({ outputDir: picked });
        syncOutLabel();
      }
    };
    document.getElementById('set-outdir-pick').addEventListener('click', pickOutputFolder);
    document.getElementById('set-outdir-clear').addEventListener('click', async () => {
      state.settings = await window.archivatore.setSettings({ outputDir: '' });
      syncOutLabel();
    });
    document.getElementById('btn-out-dir').addEventListener('click', pickOutputFolder);
    return { syncOutLabel };
  }

  function setupSplitter(initialRatio) {
    const left = document.getElementById('pane-left');
    const right = document.getElementById('pane-right');
    const splitter = document.getElementById('splitter');
    const ws = document.getElementById('workspace');

    const applyRatio = (r) => {
      const pct = Math.max(30, Math.min(75, r * 100));
      left.style.flex = `0 0 ${pct}%`;
      right.style.flex = '1 1 auto';
    };
    applyRatio(initialRatio || 0.56);

    let dragging = false;
    splitter.addEventListener('mousedown', (e) => {
      e.preventDefault();
      dragging = true;
      splitter.classList.add('dragging');
    });
    window.addEventListener('mousemove', (e) => {
      if (!dragging) return;
      const rect = ws.getBoundingClientRect();
      const ratio = (e.clientX - rect.left) / Math.max(1, rect.width);
      applyRatio(ratio);
    });
    window.addEventListener('mouseup', (e) => {
      if (!dragging) return;
      dragging = false;
      splitter.classList.remove('dragging');
      const rect = ws.getBoundingClientRect();
      const ratio = Math.max(0.3, Math.min(0.75, (e.clientX - rect.left) / Math.max(1, rect.width)));
      window.archivatore.setSettings({ splitRatio: ratio }).catch(() => {});
    });
  }

  function bindSettingsControls(state, probe) {
    const selFmt = document.getElementById('sel-format');
    const selLvl = document.getElementById('sel-level');
    const selExec = document.getElementById('sel-exec');
    const inpJobs = document.getElementById('inp-jobs');
    const inpCores = document.getElementById('inp-cores');
    const selExtract = document.getElementById('sel-extract-mode');
    const selConflict = document.getElementById('sel-conflict');
    const chkHidden = document.getElementById('chk-hidden');

    inpCores.max = String(probe.cpuCores || 64);
    selExec.value = state.settings.execMode;
    inpJobs.value = String(state.settings.parallelJobs);
    inpCores.value = String(state.settings.totalCores);
    selExtract.value = state.settings.extractMode;
    selConflict.value = state.settings.conflictPolicy;
    chkHidden.checked = Boolean(state.settings.showHidden);

    const persistPatch = async (patch) => {
      state.settings = await window.archivatore.setSettings(patch);
    };
    selFmt.addEventListener('change', () => persistPatch({ format: selFmt.value }));
    selLvl.addEventListener('change', () => persistPatch({ level: Number(selLvl.value) }));
    selExec.addEventListener('change', () => persistPatch({ execMode: selExec.value }));
    inpJobs.addEventListener('change', () => persistPatch({ parallelJobs: Number(inpJobs.value) }));
    inpCores.addEventListener('change', () => persistPatch({ totalCores: Number(inpCores.value) }));
    selExtract.addEventListener('change', () => persistPatch({ extractMode: selExtract.value }));
    selConflict.addEventListener('change', () => persistPatch({ conflictPolicy: selConflict.value }));
    chkHidden.addEventListener('change', () => {
      persistPatch({ showHidden: chkHidden.checked });
      window.ArcExplorer.setShowHidden(chkHidden.checked);
    });

    document.getElementById('btn-theme').addEventListener('click', async () => {
      const cur = state.settings.theme || 'system';
      const next = THEME_ORDER[(THEME_ORDER.indexOf(cur) + 1) % THEME_ORDER.length];
      state.settings = await window.archivatore.setSettings({ theme: next });
      applyTheme(next);
    });
    showToast('Motor multi-threading pregătit');
  }

  return { applyTheme, populateFormatsAndLevels, setupDialogs, setupSplitter, bindSettingsControls };
})();
