<?php namespace Tuna;
$base = fn (string $t) => '/notes?' . http_build_query(array_filter(['tag' => $t, 'q' => $q, 'project_id' => $project], fn ($v) => $v !== ''));
$new = array_filter(['project_id' => $project !== 'none' ? $project : '', 'tag' => $tag], fn ($v) => $v !== '') + ['next' => $url];
?>
<div class="sec-head"><h1>Notas</h1><a class="btn small primary" href="/notes/new?<?= e(http_build_query($new)) ?>">+ Nueva</a></div>
<form method="get" class="filters notes-filters">
  <input type="search" name="q" value="<?= e($q) ?>" placeholder="Buscar…">
  <select name="project_id" onchange="this.form.submit()">
    <option value="">Todos los proyectos</option>
    <option value="none"<?= selected($project === 'none') ?>>Sin proyecto</option>
    <?php foreach ($projects as $p): ?><option value="<?= $p['id'] ?>"<?= selected((string) $p['id'] === $project) ?>><?= e($p['name']) ?></option><?php endforeach ?>
  </select>
  <?php if ($tag !== ''): ?><input type="hidden" name="tag" value="<?= e($tag) ?>"><?php endif ?>
  <button class="btn small" type="submit">Filtrar</button>
</form>
<?php if ($tags): ?>
<nav class="tags" aria-label="Etiquetas">
  <a class="tag<?= $tag === '' ? ' on' : '' ?>" href="<?= e($base('')) ?>">Todas</a>
  <?php foreach ($tags as $t): ?><a class="tag<?= $t['tag'] === $tag ? ' on' : '' ?>" href="<?= e($base($t['tag'])) ?>">#<?= e($t['tag']) ?> <small><?= $t['count'] ?></small></a><?php endforeach ?>
</nav>
<?php endif ?>
<?= View::partial('_note_rows', ['items' => $notes, 'today' => $today, 'url' => $url, 'csrf' => $csrf]) ?>
