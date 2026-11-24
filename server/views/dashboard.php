<?php namespace Tuna; ?>
<h1>Inicio</h1>
<?php if ($overdue): ?><a class="alert" href="/payments"><?= $overdue ?> cuota(s) vencida(s)</a><?php endif ?>
<section>
  <div class="sec-head"><h2>Tareas abiertas</h2><a class="btn small" href="/tasks/new?next=/">+ Tarea</a></div>
  <?= View::partial('_task_rows', ['items' => $tasks, 'today' => $today, 'url' => $url, 'csrf' => $csrf]) ?>
  <a class="more" href="/tasks">Ver todas →</a>
</section>
<section>
  <div class="sec-head"><h2>Próximas cuotas</h2></div>
  <?= View::partial('_payment_rows', ['items' => $payments, 'today' => $today, 'url' => $url, 'csrf' => $csrf]) ?>
  <a class="more" href="/payments">Ver todas →</a>
</section>
