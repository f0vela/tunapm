# Tuna en PHP 8 (API + web) para hosting compartido

Mismo contrato de API y mismo esquema SQLite que la versión Go, así que la TUI (`TUNA_API`) y cualquier app
móvil funcionan igual. Sin Composer, sin extensiones raras: solo PHP 8.1+ con `pdo_sqlite`.

```
server/
  public/        ← web root (index.php, .htaccess, static/)
  src/           ← Api, Repo, Db, Web, View, Config
  views/         ← plantillas de la web
  data/          ← tuna.db (nunca bajo el web root)
  config.local.php   ← token y contraseña (lo creas tú)
```

## Probar en local
```sh
cd server
cp config.example.php config.local.php   # pon token y web_password
TUNA_REQUIRE_HTTPS=0 php -S 127.0.0.1:8080 -t public public/index.php
curl -H "Authorization: Bearer TU_TOKEN" http://127.0.0.1:8080/api/clients
tests/smoke.sh                            # contrato del API + web (desde la raíz del repo)
```

## Instalar en DreamHost (compartido)
(Pasos del panel de memoria: confirma los nombres de los menús.)

1. **Dominio:** crea un subdominio (p. ej. `tuna.tudominio.com`) en *Domains → Manage Websites*.
   Activa **HTTPS** con Let's Encrypt (gratis) y elige **PHP 8.x** para ese dominio.
2. **Directorio web:** el web root debe ser `~/tuna/public`. En la configuración del dominio cambia *Web directory*
   a `tuna/public`. Si tu panel no lo permite, borra la carpeta del dominio y haz
   `ln -s ~/tuna/public ~/tuna.tudominio.com`.
3. **Sube el código** (por SSH/SFTP), sin la base local ni la config:
   ```sh
   rsync -av --exclude config.local.php --exclude 'data/*.db*' --exclude tests server/ usuario@servidor:~/tuna/
   ```
4. **Configura** por SSH:
   ```sh
   cd ~/tuna && cp config.example.php config.local.php && chmod 600 config.local.php
   nano config.local.php            # token y web_password
   chmod 700 data
   ```
5. **Migra tus datos** (cierra la TUI antes): 
   `scp ~/.local/share/tunapm/tuna.db usuario@servidor:~/tuna/data/tuna.db`
   Si no subes nada, se crea una base vacía en el primer uso.
6. **Verifica:** `curl https://tuna.tudominio.com/api/health` → `{"status":"ok"}`; abre la web y entra con tu contraseña.
7. **TUI y móvil:** `export TUNA_API=https://tuna.tudominio.com TUNA_TOKEN=...` y ejecuta `tuna`.

## Seguridad
- HTTPS obligatorio por defecto (`require_https`); el servidor rechaza peticiones en claro.
- Token comparado con `hash_equals`, retardo ante fallos; consultas siempre preparadas; salida HTML escapada.
- La web usa cookie firmada (HttpOnly, Secure, SameSite=Strict) y token CSRF en cada POST.
- Base y config fuera del web root; `config.local.php` con permisos 600.
- SQLite en modo `DELETE` (WAL falla en discos de red). Haz copias: `cp data/tuna.db backup-$(date +%F).db`.
- Si el hosting no pasa el header `Authorization` a PHP, el `.htaccess` ya incluye la regla que lo reenvía.

## Problemas típicos
- **401 con token correcto:** el header Authorization se está perdiendo; revisa que `.htaccess` esté activo (`AllowOverride`).
- **500:** mira `error_log` del dominio; suele ser falta de `pdo_sqlite` o permisos de escritura en `data/`.
- **"se requiere HTTPS":** entra por `https://` (o, detrás de proxy, que envíe `X-Forwarded-Proto: https`).
