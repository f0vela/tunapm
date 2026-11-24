<?php namespace Tuna; ?>
<?php if (!$items): ?><p class="empty">Sin cuotas.</p><?php endif ?>
<ul class="rows">
<?php foreach ($items as $p): $st = paymentStatus($p, $today); ?>
<li class="row">
  <a class="grow" href="/payments/<?= $p['id'] ?>/edit?next=<?= rawurlencode($url) ?>">
    <strong><?= e(money($p['amount_cents'], $p['currency'])) ?></strong>
    <small><?= e($p['client_name']) ?> · <?= e($p['project_name']) ?> · <?= e(human($p['scheduled_date'], $today)) ?></small>
  </a>
  <?php if ($st === 'pagado'): ?><span class="pill ok">pagada</span>
    <form method="post" action="/payments/<?= $p['id'] ?>/unpay" onsubmit="return confirm('¿Marcar como NO pagada?')"><?= csrf_field($csrf) ?><input type="hidden" name="next" value="<?= e($url) ?>">
      <button class="btn small" type="submit">Deshacer</button></form>
  <?php else: ?>
    <?php if ($st === 'vencido'): ?><span class="pill p3">vencida</span><?php endif ?>
    <form method="post" action="/payments/<?= $p['id'] ?>/pay"><?= csrf_field($csrf) ?><input type="hidden" name="next" value="<?= e($url) ?>">
      <button class="btn small" type="submit">Pagada</button></form>
  <?php endif ?>
</li>
<?php endforeach ?>
</ul>
