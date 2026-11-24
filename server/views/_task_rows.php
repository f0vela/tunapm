<?php namespace Tuna; ?>
<?php if (!$items): ?><p class="empty">Sin tareas.</p><?php endif ?>
<ul class="rows">
<?php foreach ($items as $t): $done = $t['status'] === 'hecho'; ?>
<li class="row <?= $done ? 'done' : '' ?>">
  <form method="post" action="/tasks/<?= $t['id'] ?>/toggle"><?= csrf_field($csrf) ?><input type="hidden" name="next" value="<?= e($url) ?>">
    <button class="check" type="submit" aria-label="Completar"><?= $done ? '✓' : '' ?></button></form>
  <a class="grow" href="/tasks/<?= $t['id'] ?>/edit?next=<?= rawurlencode($url) ?>">
    <strong><?= e($t['title']) ?></strong>
    <small><?= e($t['client_name']) ?> · <?= e($t['project_name']) ?><?php if ($t['due_date']): ?> · <span class="<?= !$done && $t['due_date'] < $today ? 'late' : '' ?>"><?= e(human($t['due_date'], $today)) ?></span><?php endif ?></small>
  </a>
  <?php if ($t['priority'] >= 2): ?><span class="pill p<?= $t['priority'] ?>"><?= prio($t['priority']) ?></span><?php endif ?>
  <?php if ($t['status'] === 'en_curso'): ?><span class="pill">en curso</span><?php endif ?>
</li>
<?php endforeach ?>
</ul>
