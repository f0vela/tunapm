<?php
declare(strict_types=1);

namespace Tuna;

$root = dirname(__DIR__);
foreach (['Errors', 'Config', 'Db', 'Repo', 'Api', 'Web', 'View'] as $f) {
    require_once "$root/src/$f.php";
}

$cfg = Config::load($root);
date_default_timezone_set($cfg['timezone']);
$method = $_SERVER['REQUEST_METHOD'] ?? 'GET';
$path = rawurldecode(parse_url($_SERVER['REQUEST_URI'] ?? '/', PHP_URL_PATH) ?: '/');
$path = $path === '/' ? '/' : rtrim($path, '/');
// Servidor embebido de PHP (desarrollo): deja que sirva los archivos estáticos.
if (PHP_SAPI === 'cli-server' && $path !== '/' && is_file(__DIR__ . $path)) {
    return false;
}
$isApi = str_starts_with($path, '/api/') || $path === '/api';

header('X-Content-Type-Options: nosniff');
header('Referrer-Policy: same-origin');

try {
    if ($cfg['token'] === '') {
        throw new AppError(500, 'internal', 'servidor sin configurar: define token en config.local.php');
    }
    $https = ($_SERVER['HTTPS'] ?? '') !== '' && $_SERVER['HTTPS'] !== 'off'
        || ($_SERVER['HTTP_X_FORWARDED_PROTO'] ?? '') === 'https';
    if ($cfg['require_https'] && !$https) {
        throw new AppError(400, 'bad_request', 'se requiere HTTPS');
    }
    if ($https) {
        header('Strict-Transport-Security: max-age=31536000');
    }

    $repo = new Repo(Db::open($cfg['db_path'], $cfg['journal_mode']));
    if ($isApi) {
        (new Api($repo, $cfg['token']))->handle($method, $path);
    } elseif ($cfg['web_enabled']) {
        (new Web($repo, $cfg, $https))->handle($method, $path);
    } else {
        throw new AppError(404, 'not_found', 'no encontrado');
    }
} catch (AppError $e) {
    $isApi ? Api::error($e->status, $e->errCode, $e->getMessage()) : View::fail($e->status, $e->getMessage());
} catch (\Throwable $e) {
    error_log('tuna: ' . $e);
    $isApi ? Api::error(500, 'internal', 'error interno') : View::fail(500, 'Error interno');
}
