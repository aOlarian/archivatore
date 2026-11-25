/* MAP
 * §1 App initialization & batch actions  L5-L92
 */

// §1 App initialization & batch actions
(async () => {
  const { showToast } = window.ArcUtil;
  const Explorer = window.ArcExplorer;
  const Queue = window.ArcQueue;
  const Controls = window.ArcControls;

  const [probe, settings, initialSnap] = await Promise.all([
    window.archivatore.probeTools(),
    window.archivatore.getSettings(),
    window.archivatore.queueSnapshot(),
  ]);

  const state = { probe, settings };
  Controls.applyTheme(settings.theme);
  Controls.populateFormatsAndLevels(probe, settings);
  Controls.setupSplitter(settings.splitRatio);
  Controls.bindSettingsControls(state, probe);

  const btnCompress = document.getElementById('btn-compress-all');
  const btnExtract = document.getElementById('btn-extract-all');
  const selCount = document.getElementById('sel-count');

  function updateBatchButtons(selectedItems) {
    const count = selectedItems.length;
    const archives = selectedItems.filter((it) => it.isArchive);
    selCount.textContent = `${count} selectate`;
    btnCompress.disabled = count === 0;
    btnCompress.textContent = count > 0 ? `Compress All (${count})` : 'Compress All';
    btnExtract.disabled = archives.length === 0;
    btnExtract.textContent = archives.length > 0 ? `Extract All (${archives.length})` : 'Extract All';
  }

  Explorer.init(updateBatchButtons);
  Explorer.setShowHidden(Boolean(settings.showHidden));
  Queue.init();
  Queue.render(initialSnap);

  Controls.setupDialogs(state, () => Explorer.navigate(Explorer.getCurrentPath(), false));

  document.getElementById('btn-sel-folders').addEventListener('click', () => Explorer.selectByKind('folders'));
  document.getElementById('btn-sel-archives').addEventListener('click', () => Explorer.selectByKind('archives'));
  document.getElementById('btn-sel-none').addEventListener('click', () => Explorer.selectByKind('none'));

  btnCompress.addEventListener('click', async () => {
    const items = Explorer.getSelectedItems();
    if (!items.length) return;
    await window.archivatore.enqueueCompress(
      items.map((it) => it.path),
      {
        format: state.settings.format,
        level: state.settings.level,
        conflictPolicy: state.settings.conflictPolicy,
        outputDir: state.settings.outputDir,
      },
    );
    Explorer.selectByKind('none');
    showToast(`Adăugate ${items.length} elemente în coada de compresie`);
  });

  btnExtract.addEventListener('click', async () => {
    const archives = Explorer.getSelectedItems().filter((it) => it.isArchive);
    if (!archives.length) return;
    await window.archivatore.enqueueExtract(
      archives.map((it) => it.path),
      {
        extractMode: state.settings.extractMode,
        conflictPolicy: state.settings.conflictPolicy,
        outputDir: state.settings.outputDir,
      },
    );
    Explorer.selectByKind('none');
    showToast(`Adăugate ${archives.length} arhive la dezarhivare`);
  });

  window.addEventListener('keydown', (e) => {
    const tag = (document.activeElement && document.activeElement.tagName) || '';
    const inInput = tag === 'INPUT' || tag === 'SELECT' || tag === 'TEXTAREA';
    if (e.key === 'F5') {
      e.preventDefault();
      Explorer.navigate(Explorer.getCurrentPath(), false);
    } else if (!inInput && e.key === 'Backspace') {
      e.preventDefault();
      document.getElementById('btn-up').click();
    } else if (!inInput && e.ctrlKey && e.key.toLowerCase() === 'a') {
      e.preventDefault();
      const chkAll = document.getElementById('chk-all');
      Explorer.selectByKind(chkAll.checked ? 'none' : 'all');
    }
  });

  window.archivatore.onQueueUpdate((snap) => Queue.render(snap));
  window.archivatore.onDirChanged((dirPath) => Explorer.refreshIfSame(dirPath));

  await Explorer.loadDrives();
  await Explorer.navigate(settings.lastPath || '');
})();
