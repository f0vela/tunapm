<?php namespace Tuna; /* $dated: muestra la fecha además de la hora (listas que mezclan días) */ ?>
<ul class="rows">
<?php foreach ($items as $it): ?>
<li class="row agenda <?= itemClass($it) ?>">
  <span class="when"><?php if (!empty($dated)): ?><?= e(human($it['date'], $today)) ?><?= $it['time'] !== '' ? '<br>' : '' ?><?php endif ?><?= e($it['time']) ?></span>
  <a class="grow" href="<?= e(itemUrl($it, $url)) ?>">
    <strong><?= e(itemText($it)) ?></strong>
    <?php if ($it['detail'] !== ''): ?><small><?= e($it['detail']) ?></small><?php endif ?>
  </a>
  <span class="pill <?= itemClass($it) ?>"><?= e(itemLabel($it)) ?><?= $it['overdue'] ? ' · vencido' : '' ?></span>
</li>
<?php endforeach ?>
</ul>
