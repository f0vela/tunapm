<?php namespace Tuna; ?>
<div class="sec-head"><h1>Proyectos</h1><a class="btn small primary" href="/projects/new">+ Nuevo</a></div>
<?php if (!$rows): ?><p class="empty">Sin proyectos.</p><?php endif ?>
<ul class="rows">
<?php foreach ($rows as $p): ?>
<li class="row"><a class="grow" href="/projects/<?= $p['id'] ?>">
  <strong><?= e($p['name']) ?></strong>
  <small><?= e($p['client_name']) ?> · <?= e(statusLabel($p['status'])) ?><?= $p['due_date'] ? ' · ' . e(human($p['due_date'], $today)) : '' ?></small>
  <div class="bar"><i style="width:<?= pct($p['progress']['done'], $p['progress']['total']) ?>%"></i></div>
  <small><?= $p['progress']['done'] ?>/<?= $p['progress']['total'] ?> tareas · cobrado <?= e(money($p['totals']['paid_cents'], $p['currency'])) ?> de <?= e(money($p['budget_cents'], $p['currency'])) ?></small>
</a></li>
<?php endforeach ?>
</ul>
