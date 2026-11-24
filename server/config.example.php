<?php
// Copia este archivo como config.local.php (fuera del web root) y complétalo.
// Genera un token largo con:  openssl rand -hex 24
return [
    'token'        => '',                       // OBLIGATORIO: Bearer del API
    'web_password' => '',                       // contraseña de la web; si va vacío se usa el token
    // 'secret'    => '',                       // clave para firmar cookies (opcional, se deriva del token)
    // 'db_path'   => '/home/USUARIO/tuna/data/tuna.db',
    // 'journal_mode' => 'DELETE',              // no uses WAL en hosting compartido
    // 'timezone'  => 'America/Guatemala',
    // 'web_enabled' => true,                   // false = solo API
    // 'require_https' => true,
];
