// Tema de colores: se guarda por dispositivo en localStorage. Este archivo se
// carga en <head> sin defer para aplicar el tema antes del primer pintado.
(function () {
  var KEY = 'tuna-theme';
  var root = document.documentElement;

  function read() {
    try { return localStorage.getItem(KEY) || 'auto'; } catch (e) { return 'auto'; }
  }
  function apply(theme) {
    if (theme && theme !== 'auto') root.setAttribute('data-theme', theme);
    else root.removeAttribute('data-theme');
  }
  // La barra del navegador (y de la PWA) sigue el fondo del tema.
  function syncChrome() {
    var meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.setAttribute('content', getComputedStyle(root).getPropertyValue('--bg').trim());
  }

  apply(read());

  document.addEventListener('DOMContentLoaded', function () {
    syncChrome();
    var select = document.getElementById('theme');
    if (!select) return;
    select.value = read();
    if (select.selectedIndex < 0) select.value = 'auto';
    select.addEventListener('change', function () {
      apply(select.value);
      try { localStorage.setItem(KEY, select.value); } catch (e) {}
      syncChrome();
    });
  });
})();
