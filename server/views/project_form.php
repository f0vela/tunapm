<?php namespace Tuna; $id = $p['id'] ?? 0; ?>
<h1><?= $id ? 'Editar proyecto' : 'Nuevo proyecto' ?></h1>
<form method="post" action="<?= $id ? "/projects/$id" : '/projects' ?>" class="card form">
  <?= csrf_field($csrf) ?>
  <label>Nombre<input name="name" value="<?= e($p['name'] ?? '') ?>" required autofocus></label>
  <label>Cliente
    <select name="client_id" required>
      <option value="">Elegir…</option>
      <?php foreach ($clients as $c): ?><option value="<?= $c['id'] ?>"<?= selected($c['id'] === (int) ($p['client_id'] ?? 0)) ?>><?= e($c['name'] . ($c['company'] ? " ({$c['company']})" : '')) ?></option><?php endforeach ?>
    </select></label>
  <label>Descripción<textarea name="description" rows="3"><?= e($p['description'] ?? '') ?></textarea></label>
  <div class="two">
    <label>Estado
      <select name="status">
        <?php foreach (['prospecto', 'activo', 'pausado', 'completado', 'cancelado'] as $s): ?><option<?= selected(($p['status'] ?? 'activo') === $s) ?>><?= $s ?></option><?php endforeach ?>
      </select></label>
    <label>Cobro
      <select name="billing_type">
        <option value="total"<?= selected(($p['billing_type'] ?? 'total') === 'total') ?>>total (1 cuota)</option>
        <option value="parcial"<?= selected(($p['billing_type'] ?? '') === 'parcial') ?>>parcial</option>
      </select></label>
  </div>
  <div class="two">
    <label>Presupuesto<input name="budget" inputmode="decimal" value="<?= e(amount((int) ($p['budget_cents'] ?? 0))) ?>" placeholder="5000"></label>
    <label>Moneda
      <select name="currency">
        <option value="GTQ"<?= selected(($p['currency'] ?? 'GTQ') === 'GTQ') ?>>Q (GTQ)</option>
        <option value="USD"<?= selected(($p['currency'] ?? '') === 'USD') ?>>$ (USD)</option>
      </select></label>
  </div>
  <div class="two">
    <label>Inicio<input type="date" name="start_date" value="<?= e($p['start_date'] ?? '') ?>"></label>
    <label>Entrega<input type="date" name="due_date" value="<?= e($p['due_date'] ?? '') ?>"></label>
  </div>
  <button class="btn primary" type="submit">Guardar</button>
</form>
