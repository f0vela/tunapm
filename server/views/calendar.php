<?php namespace Tuna; $link = fn (string $v, string $d) => '/calendar?view=' . $v . '&amp;date=' . $d; ?>
<div class="sec-head"><h1><?= e($heading) ?></h1><a class="btn small primary" href="/events/new?date=<?= $date ?>&amp;next=<?= rawurlencode($url) ?>">+ Evento</a></div>
<div class="cal-nav">
  <span class="seg">
    <?php foreach (['dia' => 'Día', 'semana' => 'Semana', 'mes' => 'Mes'] as $v => $label): ?><a href="<?= $link($v, $date) ?>"<?= $view === $v ? ' class="on"' : '' ?>><?= $label ?></a><?php endforeach ?>
  </span>
  <span class="seg">
    <a href="<?= $link($view, $prev) ?>" aria-label="Anterior">‹</a>
    <a href="<?= $link($view, $today) ?>">Hoy</a>
    <a href="<?= $link($view, $nextDate) ?>" aria-label="Siguiente">›</a>
  </span>
</div>
<?php if ($weeks): ?>
<div class="cal">
  <?php foreach (WEEKDAYS_SHORT as $wd): ?><span class="cal-wd"><?= $wd ?></span><?php endforeach ?>
  <?php foreach ($weeks as $week): foreach ($week as $d): ?>
  <a class="cal-day<?= $d['in_month'] ? '' : ' out' ?><?= $d['today'] ? ' today' : '' ?><?= $d['selected'] ? ' sel' : '' ?>" href="<?= $link('mes', $d['date']) ?>" aria-label="<?= e($d['title']) ?>, <?= count($d['items']) ?> entradas">
    <span class="num"><?= $d['num'] ?></span>
    <?php if ($d['items']): ?>
    <span class="dots"><?php foreach (array_slice($d['items'], 0, 4) as $it): ?><i class="<?= itemClass($it) ?>"></i><?php endforeach ?></span>
    <span class="cal-items"><?php foreach (array_slice($d['items'], 0, 3) as $it): ?><em class="<?= itemClass($it) ?>"><?= e(trim($it['time'] . ' ' . itemText($it))) ?></em><?php endforeach ?><?php if (count($d['items']) > 3): ?><em class="rest">+<?= count($d['items']) - 3 ?> más</em><?php endif ?></span>
    <?php endif ?>
  </a>
  <?php endforeach; endforeach ?>
</div>
<?php endif ?>
<?php foreach ($days as $d): ?>
<section class="day">
  <div class="sec-head"><h2><?= e($d['title']) ?><?= $d['today'] ? ' · hoy' : '' ?></h2><?php if ($view !== 'dia'): ?><a class="more" href="/events/new?date=<?= $d['date'] ?>&amp;next=<?= rawurlencode($url) ?>">+ Evento</a><?php endif ?></div>
  <?php if ($d['items']): ?><?= View::partial('_agenda_rows', ['items' => $d['items'], 'today' => $today, 'url' => $url, 'csrf' => $csrf]) ?><?php else: ?><p class="day-empty">Nada programado.</p><?php endif ?>
</section>
<?php endforeach ?>
