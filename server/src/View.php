<?php
declare(strict_types=1);

namespace Tuna;

function e(mixed $v): string { return htmlspecialchars((string) $v, ENT_QUOTES | ENT_SUBSTITUTE, 'UTF-8'); }

function money(int $cents, string $currency = 'GTQ'): string
{
    $sign = $cents < 0 ? '-' : '';
    $cents = abs($cents);
    $sym = strtoupper($currency) === 'USD' ? '$' : 'Q';
    return sprintf('%s%s %s.%02d', $sign, $sym, number_format(intdiv($cents, 100), 0, '.', ','), $cents % 100);
}

/** Interpreta "5000", "5,000", "$ 1,200.50", "Q12.5" → centavos; null si es inválido. */
function parseMoney(string $s): ?int
{
    $s = trim($s);
    $neg = str_starts_with($s, '-');
    $s = ltrim($neg ? substr($s, 1) : $s, '$Qq');
    $s = str_replace([',', ' ', '_'], '', $s);
    if ($s === '' || !preg_match('/^(\d*)(?:\.(\d{0,2}))?$/', $s, $m) || ($m[1] === '' && ($m[2] ?? '') === '')) {
        return null;
    }
    if (strlen($m[1]) > 15) {
        return null;
    }
    $cents = (int) ($m[1] === '' ? '0' : $m[1]) * 100 + (int) str_pad($m[2] ?? '', 2, '0');
    return $neg ? -$cents : $cents;
}

function amount(int $cents): string { return $cents === 0 ? '' : sprintf('%d.%02d', intdiv($cents, 100), $cents % 100); }

function human(?string $date, string $today): string
{
    if ($date === null || $date === '') {
        return '';
    }
    if ($date === $today) {
        return 'hoy';
    }
    $t = date_create_immutable($date);
    $b = date_create_immutable($today);
    if (!$t || !$b) {
        return $date;
    }
    $days = (int) $b->diff($t)->format('%r%a');
    if ($days === 1) {
        return 'mañana';
    }
    if ($days === -1) {
        return 'ayer';
    }
    return $t->format('Y') === $b->format('Y') ? $t->format('d/m') : $date;
}

/** Marca de tiempo UTC (RFC 3339) → fecha local YYYY-MM-DD, según la zona horaria de PHP. */
function localDate(string $stamp): string
{
    $t = date_create_immutable($stamp);
    return $t ? $t->setTimezone(new \DateTimeZone(date_default_timezone_get()))->format('Y-m-d') : substr($stamp, 0, 10);
}

/** Resume el texto de una nota en una línea para las listas. */
function excerpt(string $body): string
{
    $line = implode(' ', preg_split('/\s+/u', $body, -1, PREG_SPLIT_NO_EMPTY) ?: []);
    if (function_exists('mb_strlen')) {
        return mb_strlen($line, 'UTF-8') > 140 ? mb_substr($line, 0, 140, 'UTF-8') . '…' : $line;
    }
    return strlen($line) > 140 ? substr($line, 0, 140) . '…' : $line;
}

// ---------- calendario (semana de lunes a domingo) ----------

function validDate(string $s): bool
{
    return (bool) preg_match('/^(\d{4})-(\d{2})-(\d{2})$/', $s, $m) && checkdate((int) $m[2], (int) $m[3], (int) $m[1]);
}

const MONTHS = ['enero', 'febrero', 'marzo', 'abril', 'mayo', 'junio', 'julio', 'agosto', 'septiembre', 'octubre', 'noviembre', 'diciembre'];
const WEEKDAYS = ['domingo', 'lunes', 'martes', 'miércoles', 'jueves', 'viernes', 'sábado'];
const WEEKDAYS_SHORT = ['lu', 'ma', 'mi', 'ju', 'vi', 'sá', 'do'];

function calDate(string $date): \DateTimeImmutable { return new \DateTimeImmutable($date . ' 00:00:00', new \DateTimeZone('UTC')); }
function addDays(string $date, int $days): string { return calDate($date)->modify(($days >= 0 ? '+' : '') . $days . ' days')->format('Y-m-d'); }

/** Suma meses; si el día no existe en el mes destino usa el último (31 ene + 1 mes = 28 feb). */
function addMonths(string $date, int $months): string
{
    $t = calDate($date);
    $first = $t->modify('first day of this month')->modify(($months >= 0 ? '+' : '') . $months . ' months');
    $day = min((int) $t->format('j'), (int) $first->format('t'));
    return $first->format('Y-m-') . sprintf('%02d', $day);
}

function weekStart(string $date): string
{
    $t = calDate($date);
    return $t->modify('-' . (((int) $t->format('w') + 6) % 7) . ' days')->format('Y-m-d');
}

/** @return list<string> los 7 días (lunes a domingo) de la semana de $date */
function weekDays(string $date): array
{
    $start = weekStart($date);
    return array_map(fn (int $i) => addDays($start, $i), range(0, 6));
}

/** @return list<list<string>> semanas completas que cubren el mes de $date */
function monthGrid(string $date): array
{
    $t = calDate($date);
    $last = $t->modify('last day of this month')->format('Y-m-d');
    $weeks = [];
    for ($day = weekStart($t->modify('first day of this month')->format('Y-m-d')); $day <= $last; $day = addDays($day, 7)) {
        $weeks[] = weekDays($day);
    }
    return $weeks;
}

function monthTitle(string $date): string { $t = calDate($date); return MONTHS[(int) $t->format('n') - 1] . ' ' . $t->format('Y'); }
function dayTitle(string $date): string
{
    $t = calDate($date);
    return WEEKDAYS[(int) $t->format('w')] . ' ' . $t->format('j') . ' de ' . MONTHS[(int) $t->format('n') - 1];
}
function weekTitle(string $date): string
{
    $days = weekDays($date);
    $a = calDate($days[0]);
    $b = calDate($days[6]);
    $ma = MONTHS[(int) $a->format('n') - 1];
    $mb = MONTHS[(int) $b->format('n') - 1];
    if ($ma === $mb) {
        return $a->format('j') . ' - ' . $b->format('j') . ' de ' . $mb . ' ' . $b->format('Y');
    }
    return $a->format('j') . ' ' . substr($ma, 0, 3) . ' - ' . $b->format('j') . ' ' . substr($mb, 0, 3) . ' ' . $b->format('Y');
}

/** Etiqueta corta de una entrada del calendario. */
function itemLabel(array $it): string
{
    return match ($it['type']) {
        'task' => 'tarea', 'payment' => 'cobro', 'project_start' => 'inicio', 'project_due' => 'entrega',
        default => Repo::EVENT_KINDS[$it['kind']] ?? 'evento',
    };
}

/** Título para mostrar; en un cobro incluye el monto. */
function itemText(array $it): string
{
    if ($it['type'] !== 'payment') {
        return $it['title'];
    }
    return trim(money($it['amount_cents'], $it['currency']) . ' ' . $it['title']);
}

/** Clases CSS de una entrada según su tipo y estado. */
function itemClass(array $it): string
{
    $class = str_starts_with($it['type'], 'project') ? 't-project' : 't-' . $it['type'];
    return $class . ($it['overdue'] ? ' overdue' : '') . ($it['done'] ? ' done' : '');
}

/** Dónde se edita cada entrada. */
function itemUrl(array $it, string $next): string
{
    $q = '?next=' . rawurlencode($next);
    return match ($it['type']) {
        'event' => '/events/' . $it['id'] . '/edit' . $q,
        'task' => '/tasks/' . $it['id'] . '/edit' . $q,
        'payment' => '/payments/' . $it['id'] . '/edit' . $q,
        default => '/projects/' . $it['id'],
    };
}

function pct(int $done, int $total): int { return $total === 0 ? 0 : intdiv($done * 100, $total); }
function prio(int $p): string { return ['baja', 'normal', 'alta', 'urgente'][max(0, min(3, $p))]; }
function statusLabel(string $s): string { return str_replace('_', ' ', $s); }

function paymentStatus(array $p, string $today): string
{
    if ($p['paid_date'] !== null) {
        return 'pagado';
    }
    return $p['scheduled_date'] < $today ? 'vencido' : 'pendiente';
}

/** URL de un estático con su fecha de modificación, para invalidar la caché del navegador al actualizar. */
function asset(string $name): string
{
    $mtime = @filemtime(__DIR__ . '/../public/static/' . $name);
    return '/static/' . $name . ($mtime ? '?v=' . $mtime : '');
}

final class View
{
    public static string $csrf = '';
    public static bool $auth = false;

    /** Renderiza views/$name.php dentro del layout. */
    public static function render(string $name, array $d, string $nav, string $title, int $status = 200, string $error = ''): void
    {
        $ctx = [
            'today' => date('Y-m-d'),
            'url' => $_SERVER['REQUEST_URI'] ?? '/',
            'flash' => (string) ($_GET['ok'] ?? ''),
            'error' => $error,
            'csrf' => self::$csrf,
        ];
        $content = self::partial($name, $d + $ctx);
        http_response_code($status);
        header('Content-Type: text/html; charset=utf-8');
        header('Cache-Control: no-store');
        echo self::partial('layout', $ctx + ['content' => $content, 'nav' => $nav, 'title' => $title, 'auth' => self::$auth]);
    }

    public static function partial(string $name, array $vars): string
    {
        extract($vars, EXTR_SKIP);
        ob_start();
        require __DIR__ . '/../views/' . $name . '.php';
        return (string) ob_get_clean();
    }

    public static function fail(int $status, string $message): void
    {
        self::render('error', ['message' => $message], '', 'Error', $status);
    }
}

function csrf_field(string $csrf): string { return '<input type="hidden" name="_csrf" value="' . e($csrf) . '">'; }

function selected(bool $cond): string { return $cond ? ' selected' : ''; }
