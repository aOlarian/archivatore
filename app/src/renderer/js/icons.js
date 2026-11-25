/* MAP
 * §1 Inline SVG icon helpers  L5-L30
 */

// §1 Inline SVG icon helpers
window.ArcIcons = (() => {
  const folder = '<svg class="ico-svg ico-dir" viewBox="0 0 24 24" fill="currentColor"><path d="M3 5a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5z"/></svg>';
  const archive = '<svg class="ico-svg ico-arch" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M12 3v18M10 7h2M12 10h2M10 13h2M12 16h2"/></svg>';
  const file = '<svg class="ico-svg ico-file" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>';
  const drive = '<svg class="ico-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="5" width="20" height="14" rx="2"/><circle cx="17" cy="14" r="1.2" fill="currentColor"/></svg>';

  return { folder, archive, file, drive };
})();
