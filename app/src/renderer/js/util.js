/* MAP
 * §1 Formatters, escape & toast  L5-L72
 */

// §1 Formatters, escape & toast
window.ArcUtil = (() => {
  function esc(str) {
    return String(str ?? '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  function formatBytes(bytes) {
    const n = Number(bytes) || 0;
    if (n <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.min(units.length - 1, Math.floor(Math.log(n) / Math.log(1024)));
    const val = n / Math.pow(1024, i);
    return `${val >= 100 || i === 0 ? val.toFixed(0) : val.toFixed(1)} ${units[i]}`;
  }

  function formatSpeed(bps) {
    const n = Number(bps) || 0;
    if (n <= 0) return '0 MB/s';
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB/s`;
    return `${(n / (1024 * 1024)).toFixed(1)} MB/s`;
  }

  function formatDuration(ms) {
    const s = Math.max(0, (Number(ms) || 0) / 1000);
    if (s < 10) return `${s.toFixed(2)}s`;
    if (s < 60) return `${s.toFixed(1)}s`;
    const m = Math.floor(s / 60);
    return `${m}m ${Math.round(s % 60)}s`;
  }

  function formatDate(ms) {
    if (!ms) return '';
    const d = new Date(ms);
    const pad = (x) => String(x).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }

  let toastTimer = null;
  function showToast(msg) {
    const el = document.getElementById('toast');
    if (!el) return;
    el.textContent = msg;
    el.hidden = false;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => { el.hidden = true; }, 3200);
  }

  return { esc, formatBytes, formatSpeed, formatDuration, formatDate, showToast };
})();
