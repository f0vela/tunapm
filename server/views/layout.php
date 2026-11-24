<?php namespace Tuna; ?><!doctype html>
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<meta name="theme-color" content="#0f172a">
<meta name="apple-mobile-web-app-capable" content="yes">
<title><?= e($title) ?> · Tuna</title>
<link rel="manifest" href="/static/manifest.json">
<link rel="icon" href="/static/icon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="/static/icon.svg">
<link rel="stylesheet" href="/static/app.css">
</head>
<body>
<header class="top">
  <a class="brand" href="/">🐟 Tuna</a>
  <?php if ($auth && $nav !== ''): ?><form method="post" action="/logout"><?= csrf_field($csrf) ?><button class="link" type="submit">Salir</button></form><?php endif ?>
</header>
<main>
  <?php if ($flash !== ''): ?><div class="flash"><?= e($flash) ?></div><?php endif ?>
  <?php if ($error !== ''): ?><div class="error"><?= e($error) ?></div><?php endif ?>
  <?= $content ?>
</main>
<?php if ($nav !== ''): ?>
<nav class="tabs">
  <?php foreach ([['inicio', '/', '⌂', 'Inicio'], ['tareas', '/tasks', '✓', 'Tareas'], ['proyectos', '/projects', '▤', 'Proyectos'], ['cuotas', '/payments', '$', 'Cuotas'], ['clientes', '/clients', '☺', 'Clientes']] as [$k, $href, $icon, $label]): ?>
    <a href="<?= $href ?>"<?= $nav === $k ? ' class="on"' : '' ?>><span><?= $icon ?></span><?= $label ?></a>
  <?php endforeach ?>
</nav>
<?php endif ?>
</body>
</html>
