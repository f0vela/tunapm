<?php namespace Tuna; $id = $c['id'] ?? 0; ?>
<h1><?= $id ? 'Editar cliente' : 'Nuevo cliente' ?></h1>
<form method="post" action="<?= $id ? "/clients/$id" : '/clients' ?>" class="card form">
  <?= csrf_field($csrf) ?>
  <label>Nombre<input name="name" value="<?= e($c['name'] ?? '') ?>" required autofocus></label>
  <label>Empresa<input name="company" value="<?= e($c['company'] ?? '') ?>"></label>
  <label>Correo<input type="email" name="email" value="<?= e($c['email'] ?? '') ?>"></label>
  <label>Teléfono<input type="tel" name="phone" value="<?= e($c['phone'] ?? '') ?>"></label>
  <label>Notas<textarea name="notes" rows="4"><?= e($c['notes'] ?? '') ?></textarea></label>
  <button class="btn primary" type="submit">Guardar</button>
</form>
