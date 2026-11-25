<?php namespace Tuna; $id = $ev['id'] ?? 0; ?>
<h1><?= $id ? 'Editar evento' : 'Nuevo evento' ?></h1>
<form method="post" action="<?= $id ? "/events/$id" : '/events' ?>" class="card form">
  <?= csrf_field($csrf) ?><input type="hidden" name="next" value="<?= e($next) ?>">
  <label>Título<input name="title" value="<?= e($ev['title'] ?? '') ?>" required<?= $id ? '' : ' autofocus' ?> placeholder="Reunión de avance"></label>
  <label>Tipo
    <select name="kind">
      <?php foreach (Repo::EVENT_KINDS as $k => $l): ?><option value="<?= $k ?>"<?= selected(($ev['kind'] ?? 'reunion') === $k) ?>><?= $l ?></option><?php endforeach ?>
    </select></label>
  <div class="two">
    <label>Fecha<input type="date" name="date" value="<?= e($ev['date'] ?? '') ?>" required></label>
    <label>Hora<input type="time" name="time" value="<?= e($ev['time'] ?? '') ?>">
      <small>Vacía = todo el día.</small></label>
  </div>
  <label>Proyecto
    <select name="project_id">
      <option value="">Sin proyecto</option>
      <?php foreach ($projects as $p): ?><option value="<?= $p['id'] ?>"<?= selected($p['id'] === (int) ($ev['project_id'] ?? 0)) ?>><?= e($p['name'] . ' · ' . $p['client_name']) ?></option><?php endforeach ?>
    </select></label>
  <label>Cliente
    <select name="client_id">
      <option value="">Sin cliente (o el del proyecto)</option>
      <?php foreach ($clients as $c): ?><option value="<?= $c['id'] ?>"<?= selected($c['id'] === (int) ($ev['client_id'] ?? 0)) ?>><?= e($c['name'] . ($c['company'] !== '' ? ' · ' . $c['company'] : '')) ?></option><?php endforeach ?>
    </select></label>
  <label>Notas<textarea name="notes" rows="4"><?= e($ev['notes'] ?? '') ?></textarea></label>
  <button class="btn primary" type="submit">Guardar</button>
</form>
<?php if ($id): ?>
<form method="post" action="/events/<?= $id ?>/delete" onsubmit="return confirm('¿Eliminar evento?')">
  <?= csrf_field($csrf) ?><input type="hidden" name="next" value="<?= e($next) ?>"><button class="btn danger" type="submit">Eliminar evento</button></form>
<?php endif ?>
