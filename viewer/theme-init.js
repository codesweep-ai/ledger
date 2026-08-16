(function themeInit() {
  var t = null;
  var m = /[?&]theme=(light|dark)\b/.exec(location.search);
  if (m) t = m[1]; // URL override wins; not persisted
  if (t !== 'light' && t !== 'dark') {
    try { t = localStorage.getItem('ledger-theme'); } catch (e) { /* file:// may block storage */ }
  }
  if (t !== 'light' && t !== 'dark') {
    t = (window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches) ? 'light' : 'dark';
  }
  document.documentElement.setAttribute('data-theme', t);
})();