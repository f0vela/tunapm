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
