<?php namespace Tuna; ?>
<div class="sec-head"><h1>Tareas</h1><a class="btn small primary" href="/tasks/new?<?= $project_id ? 'project_id=' . $project_id . '&' : '' ?>next=/tasks">+ Nueva</a></div>
<form method="get" class="filters">
  <input type="search" name="q" value="<?= e($q) ?>" placeholder="Buscar…">
  <select name="mode" onchange="this.form.submit()">
    <?php foreach (['abiertas' => 'Abiertas', 'hechas' => 'Hechas', 'todas' => 'Todas'] as $k => $l): ?><option value="<?= $k ?>"<?= selected($mode === $k) ?>><?= $l ?></option><?php endforeach ?>
  </select>
  <select name="project_id" onchange="this.form.submit()">
    <option value="">Todos los proyectos</option>
    <?php foreach ($projects as $p): ?><option value="<?= $p['id'] ?>"<?= selected($p['id'] === $project_id) ?>><?= e($p['name']) ?></option><?php endforeach ?>
  </select>
  <button class="btn small" type="submit">Filtrar</button>
</form>
<?= View::partial('_task_rows', ['items' => $tasks, 'today' => $today, 'url' => $url, 'csrf' => $csrf]) ?>
