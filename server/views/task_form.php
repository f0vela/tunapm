<?php namespace Tuna; $id = $t['id'] ?? 0; ?>
<h1><?= $id ? 'Editar tarea' : 'Nueva tarea' ?></h1>
<form method="post" action="<?= $id ? "/tasks/$id" : '/tasks' ?>" class="card form">
  <?= csrf_field($csrf) ?><input type="hidden" name="next" value="<?= e($next) ?>">
  <label>Título<input name="title" value="<?= e($t['title'] ?? '') ?>" required autofocus></label>
  <label>Proyecto
    <select name="project_id" required>
      <option value="">Elegir…</option>
      <?php foreach ($projects as $p): ?><option value="<?= $p['id'] ?>"<?= selected($p['id'] === (int) ($t['project_id'] ?? 0)) ?>><?= e($p['client_name'] . ' · ' . $p['name']) ?></option><?php endforeach ?>
    </select></label>
  <div class="two">
    <label>Estado
      <select name="status">
        <?php foreach (['pendiente' => 'pendiente', 'en_curso' => 'en curso', 'hecho' => 'hecho'] as $k => $l): ?><option value="<?= $k ?>"<?= selected(($t['status'] ?? 'pendiente') === $k) ?>><?= $l ?></option><?php endforeach ?>
      </select></label>
    <label>Prioridad
      <select name="priority">
        <?php foreach (['baja', 'normal', 'alta', 'urgente'] as $i => $l): ?><option value="<?= $i ?>"<?= selected((int) ($t['priority'] ?? 1) === $i) ?>><?= $l ?></option><?php endforeach ?>
      </select></label>
  </div>
  <label>Fecha límite<input type="date" name="due_date" value="<?= e($t['due_date'] ?? '') ?>"></label>
  <label>Notas<textarea name="notes" rows="4"><?= e($t['notes'] ?? '') ?></textarea></label>
  <button class="btn primary" type="submit">Guardar</button>
</form>
<?php if ($id): ?>
<form method="post" action="/tasks/<?= $id ?>/delete" onsubmit="return confirm('¿Eliminar tarea?')">
  <?= csrf_field($csrf) ?><input type="hidden" name="next" value="<?= e($next) ?>"><button class="btn danger" type="submit">Eliminar</button></form>
<?php endif ?>
