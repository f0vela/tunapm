<?php namespace Tuna; $id = $p['id'] ?? 0; ?>
<h1><?= $id ? 'Editar cuota' : 'Nueva cuota' ?></h1>
<form method="post" action="<?= $id ? "/payments/$id" : '/payments' ?>" class="card form">
  <?= csrf_field($csrf) ?><input type="hidden" name="next" value="<?= e($next) ?>">
  <label>Proyecto
    <select name="project_id" required>
      <option value="">—</option>
      <?php foreach ($projects as $pr): ?><option value="<?= $pr['id'] ?>"<?= selected($pr['id'] === (int) ($p['project_id'] ?? 0)) ?>><?= e($pr['client_name'] . ' · ' . $pr['name'] . ' (' . $pr['currency'] . ')') ?></option><?php endforeach ?>
    </select></label>
  <label>Monto<input name="amount" inputmode="decimal" value="<?= e(amount((int) ($p['amount_cents'] ?? 0))) ?>" required placeholder="2500"></label>
  <div class="two">
    <label>Fecha programada<input type="date" name="scheduled_date" value="<?= e($p['scheduled_date'] ?? '') ?>" required></label>
    <label>Fecha de pago<input type="date" name="paid_date" value="<?= e($p['paid_date'] ?? '') ?>"></label>
  </div>
  <label>Notas<textarea name="notes" rows="3"><?= e($p['notes'] ?? '') ?></textarea></label>
  <button class="btn primary" type="submit">Guardar</button>
</form>
<?php if ($id): ?>
<form method="post" action="/payments/<?= $id ?>/delete" onsubmit="return confirm('¿Eliminar cuota?')">
  <?= csrf_field($csrf) ?><input type="hidden" name="next" value="<?= e($next) ?>"><button class="btn danger" type="submit">Eliminar</button></form>
<?php endif ?>
