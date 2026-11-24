<?php namespace Tuna; ?>
<?php if (!$items): ?><p class="empty">Sin notas.</p><?php endif ?>
<ul class="rows">
<?php foreach ($items as $n): ?>
<li class="row note">
  <a class="grow" href="/notes/<?= $n['id'] ?>/edit?next=<?= rawurlencode($url) ?>">
    <strong><?= e($n['title']) ?></strong>
    <small><?= e($n['project_name'] !== '' ? $n['project_name'] : 'Sin proyecto') ?> · <?= e(human(localDate($n['updated_at']), $today)) ?></small>
    <?php if ($n['body'] !== ''): ?><small class="preview"><?= e(excerpt($n['body'])) ?></small><?php endif ?>
  </a>
  <?php if ($n['tags']): ?><span class="row-tags"><?php foreach ($n['tags'] as $t): ?><a class="tag" href="/notes?tag=<?= rawurlencode($t) ?>">#<?= e($t) ?></a><?php endforeach ?></span><?php endif ?>
</li>
<?php endforeach ?>
</ul>
