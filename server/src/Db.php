<?php
declare(strict_types=1);

namespace Tuna;

final class Db
{
    /** Abre (o crea) la base y aplica las migraciones pendientes. */
    public static function open(string $path, string $journalMode = 'DELETE'): \PDO
    {
        if ($path !== ':memory:' && !is_dir(dirname($path))) {
            mkdir(dirname($path), 0700, true);
        }
        $pdo = new \PDO('sqlite:' . $path, null, null, [
            \PDO::ATTR_ERRMODE => \PDO::ERRMODE_EXCEPTION,
            \PDO::ATTR_DEFAULT_FETCH_MODE => \PDO::FETCH_ASSOC,
            \PDO::ATTR_EMULATE_PREPARES => false,
        ]);
        $pdo->exec('PRAGMA foreign_keys = ON');
        $pdo->exec('PRAGMA busy_timeout = 5000');
        // WAL requiere memoria compartida y falla en sistemas de archivos de red
        // (típico en hosting compartido): por defecto usamos DELETE.
        $mode = in_array(strtoupper($journalMode), ['DELETE', 'WAL', 'TRUNCATE'], true) ? strtoupper($journalMode) : 'DELETE';
        $pdo->exec("PRAGMA journal_mode = $mode");
        self::migrate($pdo);
        return $pdo;
    }

    /** Migraciones en orden; cada una se aplica una sola vez y sube user_version. */
    private const MIGRATIONS = ['001_init.sql', '002_notes.sql'];

    private static function migrate(\PDO $pdo): void
    {
        $version = (int) $pdo->query('PRAGMA user_version')->fetchColumn();
        foreach (self::MIGRATIONS as $i => $file) {
            if ($version > $i) {
                continue;
            }
            $pdo->exec('BEGIN IMMEDIATE');
            try {
                $pdo->exec((string) file_get_contents(__DIR__ . '/' . $file));
                $pdo->exec('PRAGMA user_version = ' . ($i + 1));
                $pdo->exec('COMMIT');
            } catch (\Throwable $e) {
                $pdo->exec('ROLLBACK');
                throw $e;
            }
        }
    }
}
