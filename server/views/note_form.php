<?php namespace Tuna; $id = $n['id'] ?? 0; ?>
<h1><?= $id ? 'Editar nota' : 'Nueva nota' ?></h1>
<form method="post" action="<?= $id ? "/notes/$id" : '/notes' ?>" class="card form">
  <?= csrf_field($csrf) ?><input type="hidden" name="next" value="<?= e($next) ?>">
  <label>Título<input name="title" value="<?= e($n['title'] ?? '') ?>" required<?= $id ? '' : ' autofocus' ?>></label>
  <label>Proyecto
    <select name="project_id">
      <option value="">Sin proyecto</option>
      <?php foreach ($projects as $p): ?><option value="<?= $p['id'] ?>"<?= selected($p['id'] === (int) ($n['project_id'] ?? 0)) ?>><?= e($p['name'] . ' · ' . $p['client_name']) ?></option><?php endforeach ?>
    </select></label>
  <label>Etiquetas
    <input name="tags" value="<?= e(implode(' ', $n['tags'] ?? [])) ?>" placeholder="idea reunion pendiente-cliente" autocapitalize="none" autocomplete="off" list="tag-list">
    <small>Separadas por espacio o coma. Sirven para filtrar las notas.</small></label>
  <datalist id="tag-list"><?php foreach ($tags as $t): ?><option value="<?= e($t['tag']) ?>"><?php endforeach ?></datalist>
  <label>Texto<textarea name="body" rows="12"><?= e($n['body'] ?? '') ?></textarea></label>
  <button class="btn primary" type="submit">Guardar</button>
</form>
<?php if ($id): ?>
<form method="post" action="/notes/<?= $id ?>/delete" onsubmit="return confirm('¿Eliminar nota?')">
  <?= csrf_field($csrf) ?><input type="hidden" name="next" value="<?= e($next) ?>"><button class="btn danger" type="submit">Eliminar nota</button></form>
<?php endif ?>
