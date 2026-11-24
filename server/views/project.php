<?php namespace Tuna; ?>
<div class="sec-head"><h1><?= e($p['name']) ?></h1><a class="btn small" href="/projects/<?= $p['id'] ?>/edit">Editar</a></div>
<div class="card">
  <p><a href="/clients/<?= $p['client_id'] ?>"><?= e($p['client_name']) ?></a> · <span class="pill"><?= e(statusLabel($p['status'])) ?></span></p>
  <?php if ($p['description']): ?><p><?= e($p['description']) ?></p><?php endif ?>
  <p><small>Presupuesto <?= e(money($p['budget_cents'], $p['currency'])) ?> · cobro <?= e($p['billing_type']) ?><?= $p['due_date'] ? ' · entrega ' . e(human($p['due_date'], $today)) : '' ?></small></p>
  <p><small>Programado <?= e(money($totals['scheduled_cents'], $p['currency'])) ?> · pagado <?= e(money($totals['paid_cents'], $p['currency'])) ?></small></p>
</div>
<section>
  <div class="sec-head"><h2>Tareas</h2><a class="btn small" href="/tasks/new?project_id=<?= $p['id'] ?>&next=/projects/<?= $p['id'] ?>">+ Tarea</a></div>
  <?= View::partial('_task_rows', ['items' => $tasks, 'today' => $today, 'url' => $url, 'csrf' => $csrf]) ?>
</section>
<section>
  <div class="sec-head"><h2>Cuotas</h2><a class="btn small" href="/payments/new?project_id=<?= $p['id'] ?>&next=/projects/<?= $p['id'] ?>">+ Cuota</a></div>
  <?= View::partial('_payment_rows', ['items' => $payments, 'today' => $today, 'url' => $url, 'csrf' => $csrf]) ?>
</section>
<form method="post" action="/projects/<?= $p['id'] ?>/delete" onsubmit="return confirm('¿Eliminar proyecto con sus tareas y cuotas?')">
  <?= csrf_field($csrf) ?><button class="btn danger" type="submit">Eliminar proyecto</button></form>
