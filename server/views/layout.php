<?php namespace Tuna; ?><!doctype html>
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<meta name="theme-color" content="#f6f7f8">
<meta name="apple-mobile-web-app-capable" content="yes">
<title><?= e($title) ?> · Tuna</title>
<link rel="manifest" href="<?= asset('manifest.json') ?>">
<link rel="icon" href="<?= asset('icon.svg') ?>" type="image/svg+xml">
<link rel="apple-touch-icon" href="<?= asset('icon.svg') ?>">
<link rel="stylesheet" href="<?= asset('app.css') ?>">
<script src="<?= asset('theme.js') ?>"></script>
</head>
<body>
<header class="top">
  <a class="brand" href="/">tuna<b>pm</b></a>
  <label class="theme"><span class="sr">Tema de colores</span>
    <select id="theme" autocomplete="off">
      <option value="auto">Automático</option>
      <option value="light">Claro</option>
      <option value="dark">Oscuro</option>
      <option value="dracula">Dracula</option>
      <option value="catppuccin-mocha">Catppuccin Mocha</option>
      <option value="catppuccin-macchiato">Catppuccin Macchiato</option>
      <option value="catppuccin-latte">Catppuccin Latte</option>
      <option value="rose-pine">Rosé Pine</option>
      <option value="rose-pine-moon">Rosé Pine Moon</option>
      <option value="rose-pine-dawn">Rosé Pine Dawn</option>
      <option value="tokyo-night">Tokyo Night</option>
      <option value="gruvbox">Gruvbox</option>
      <option value="nord">Nord</option>
      <option value="solarized-light">Solarized Light</option>
    </select></label>
  <?php if ($auth && $nav !== ''): ?><form method="post" action="/logout"><?= csrf_field($csrf) ?><button class="link" type="submit">Salir</button></form><?php endif ?>
</header>
<main>
  <?php if ($flash !== ''): ?><div class="flash"><?= e($flash) ?></div><?php endif ?>
  <?php if ($error !== ''): ?><div class="error"><?= e($error) ?></div><?php endif ?>
  <?= $content ?>
</main>
<?php if ($nav !== ''): ?>
<nav class="tabs">
  <?php foreach ([['inicio', '/', 'Inicio'], ['tareas', '/tasks', 'Tareas'], ['proyectos', '/projects', 'Proyectos'], ['cuotas', '/payments', 'Cuotas'], ['clientes', '/clients', 'Clientes'], ['notas', '/notes', 'Notas']] as [$k, $href, $label]): ?>
    <a href="<?= $href ?>"<?= $nav === $k ? ' class="on"' : '' ?>><?= $label ?></a>
  <?php endforeach ?>
</nav>
<?php endif ?>
</body>
</html>
