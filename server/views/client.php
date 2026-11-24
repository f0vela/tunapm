<?php namespace Tuna; ?>
<div class="sec-head"><h1><?= e($c['name']) ?></h1><a class="btn small" href="/clients/<?= $c['id'] ?>/edit">Editar</a></div>
<div class="card">
  <?php if ($c['company']): ?><p><strong><?= e($c['company']) ?></strong></p><?php endif ?>
  <?php if ($c['email']): ?><p><a href="mailto:<?= e($c['email']) ?>"><?= e($c['email']) ?></a></p><?php endif ?>
  <?php if ($c['phone']): ?><p><a href="tel:<?= e($c['phone']) ?>"><?= e($c['phone']) ?></a></p><?php endif ?>
  <?php if ($c['notes']): ?><p><?= e($c['notes']) ?></p><?php endif ?>
</div>
<section>
  <div class="sec-head"><h2>Proyectos</h2><a class="btn small" href="/projects/new?client_id=<?= $c['id'] ?>">+ Proyecto</a></div>
  <?php if (!$projects): ?><p class="empty">Sin proyectos.</p><?php endif ?>
  <ul class="rows"><?php foreach ($projects as $p): ?><li class="row"><a class="grow" href="/projects/<?= $p['id'] ?>"><strong><?= e($p['name']) ?></strong><small><?= e(statusLabel($p['status'])) ?> · <?= e(money($p['budget_cents'], $p['currency'])) ?></small></a></li><?php endforeach ?></ul>
</section>
<form method="post" action="/clients/<?= $c['id'] ?>/delete" onsubmit="return confirm('¿Eliminar cliente?')">
  <?= csrf_field($csrf) ?><button class="btn danger" type="submit">Eliminar cliente</button></form>
