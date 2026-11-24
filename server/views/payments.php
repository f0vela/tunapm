<?php namespace Tuna; ?>
<div class="sec-head"><h1>Cuotas</h1><a class="btn small primary" href="/payments/new?next=/payments">+ Nueva</a></div>
<?= View::partial('_payment_rows', ['items' => $payments, 'today' => $today, 'url' => $url, 'csrf' => $csrf]) ?>
