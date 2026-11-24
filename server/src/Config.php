<?php
declare(strict_types=1);

namespace Tuna;

final class Config
{
    /** Carga config.local.php (junto a src/) y deja que las variables TUNA_* lo sobreescriban. */
    public static function load(string $root): array
    {
        $cfg = [
            'db_path'        => $root . '/data/tuna.db',
            'journal_mode'   => 'DELETE',
            'token'          => '',
            'web_enabled'    => true,
            'web_password'   => '',
            'secret'         => '',
            'require_https'  => true,
            'timezone'       => 'America/Guatemala',
        ];
        $local = $root . '/config.local.php';
        if (is_file($local)) {
            $cfg = array_replace($cfg, (array) require $local);
        }
        foreach (['db_path' => 'TUNA_DB', 'token' => 'TUNA_TOKEN', 'web_password' => 'TUNA_WEB_PASSWORD',
                  'secret' => 'TUNA_SECRET', 'timezone' => 'TUNA_TZ', 'journal_mode' => 'TUNA_JOURNAL'] as $key => $env) {
            if (($v = getenv($env)) !== false && $v !== '') {
                $cfg[$key] = $v;
            }
        }
        if (($v = getenv('TUNA_REQUIRE_HTTPS')) !== false && $v !== '') {
            $cfg['require_https'] = !in_array(strtolower($v), ['0', 'false', 'no'], true);
        }
        if (($v = getenv('TUNA_WEB')) !== false && $v !== '') {
            $cfg['web_enabled'] = !in_array(strtolower($v), ['0', 'false', 'no'], true);
        }
        if ($cfg['web_password'] === '') {
            $cfg['web_password'] = $cfg['token'];
        }
        if ($cfg['secret'] === '') {
            $cfg['secret'] = hash('sha256', 'tuna-secret:' . $cfg['token'] . $cfg['web_password']);
        }
        return $cfg;
    }
}
