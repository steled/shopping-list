(function () {
  var html = document.getElementById('html-root');
  var btn  = document.getElementById('theme-toggle');
  var stored = localStorage.getItem('theme');
  var prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
  var theme = stored || (prefersDark ? 'dark' : 'light');
  html.dataset.theme = theme;
  if (btn) btn.textContent = theme === 'dark' ? '☀️' : '🌙';
  if (btn) btn.addEventListener('click', function () {
    var next = html.dataset.theme === 'dark' ? 'light' : 'dark';
    html.dataset.theme = next;
    localStorage.setItem('theme', next);
    btn.textContent = next === 'dark' ? '☀️' : '🌙';
  });
})();
