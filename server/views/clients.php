<?php namespace Tuna; ?>
<div class="sec-head"><h1>Clientes</h1><a class="btn small primary" href="/clients/new">+ Nuevo</a></div>
<?php if (!$clients): ?><p class="empty">Sin clientes.</p><?php endif ?>
<ul class="rows">
<?php foreach ($clients as $c): ?><li class="row"><a class="grow" href="/clients/<?= $c['id'] ?>"><strong><?= e($c['name']) ?></strong><small><?= e($c['company']) ?></small></a></li><?php endforeach ?>
</ul>
