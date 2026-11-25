# Tuna: API, TUI remota y web app

Una sola base SQLite vive en el servidor. Todo lo demás habla con ella a través del API.

```
 TUI (Mac)  ──┐
 Web app    ──┼──►  API REST (tuna serve)  ──►  tuna.db (SQLite)
 App móvil  ──┘
```

> **Hosting compartido (DreamHost):** usa la versión PHP en `server/` (ver `server/README.md`); expone el mismo API y la misma web.
> Lo de abajo describe el servidor en Go (`tuna serve`), útil para VPS o para tu Mac.

## Servidor (Go)

```sh
export TUNA_TOKEN=$(openssl rand -hex 24)        # token del API
export TUNA_WEB_PASSWORD='tu-contraseña'          # opcional; default = token
tuna -db /var/lib/tuna/tuna.db serve -addr 127.0.0.1:8080 -web -secure-cookie
```

- `/api/...` → API JSON (header `Authorization: Bearer $TUNA_TOKEN`).
- `/` → web app (con `-web`), login por contraseña, pensada para móvil (instalable como PWA).
- Escuchar fuera de localhost sin token se rechaza. Pon **HTTPS** delante (Caddy, nginx o Tailscale);
  el token y la cookie viajan en claro si no.
- Para migrar tus datos: copia `~/.local/share/tunapm/tuna.db` al servidor (con la TUI cerrada).

Alternativa: la web como proceso aparte, hablando con el API:
`tuna -api https://api.ejemplo.com -token $TUNA_TOKEN web -addr :8081`

## TUI contra el servidor

```sh
export TUNA_API=https://tuna.ejemplo.com TUNA_TOKEN=...
tuna            # sin TUNA_API sigue usando SQLite local
```

## Endpoints

| Recurso   | Rutas |
|-----------|-------|
| clients   | `GET/POST /api/clients`, `GET/PUT/DELETE /api/clients/{id}`, `GET /api/clients/{id}/projects`, `/project-count` |
| projects  | `GET/POST /api/projects`, `GET/PUT/DELETE /api/projects/{id}`, `/payments`, `/totals`; `GET /api/projects/totals`, `/progress` |
| tasks     | `GET /api/tasks?mode=abiertas\|hechas\|todas&project_id=&client_id=&search=`, `POST`, `GET/PUT/DELETE /api/tasks/{id}` |
| payments  | `GET/POST /api/payments`, `GET/PUT/DELETE /api/payments/{id}`, `POST /api/payments/{id}/pay` `{"date":"YYYY-MM-DD"}` |
| notes     | `GET /api/notes?project_id=<id>\|none&tag=&search=`, `POST`, `GET/PUT/DELETE /api/notes/{id}`, `GET /api/notes/tags` |
| events    | `GET /api/events?from=&to=`, `POST`, `GET/PUT/DELETE /api/events/{id}` |
| agenda    | `GET /api/agenda?from=YYYY-MM-DD&to=YYYY-MM-DD` (calendario ya armado) |
| health    | `GET /api/health` (sin auth) |

Notas: `{"title","body","project_id","tags"}`. `project_id` es opcional (`null` = nota sin proyecto; `project_id=none`
en el listado las filtra). `tags` es una lista; el servidor las normaliza (minúsculas, sin `#`, guiones en vez de
espacios, sin duplicados). `GET /api/notes/tags` devuelve `[{"tag","count"}]`. Al borrar un proyecto, sus notas se
conservan y quedan sin proyecto. `/api/snapshot` incluye `notes`.

Eventos: `{"title","kind","date","time","project_id","client_id","notes"}`. `kind` es `reunion`, `llamada`,
`presentacion`, `lanzamiento` u `otro` (cualquier otro valor se guarda como `otro`). `time` es opcional (`HH:MM`;
`null` = todo el día). `project_id` y `client_id` son opcionales y, si se borran, el evento se conserva.
`/api/snapshot` incluye `events`.

Agenda: lista por fecha y hora de `{date,time,type,kind,id,title,detail,project_id,done,overdue,amount_cents,currency}`.
`type` es `event`, `task` (tareas con fecha límite), `payment` (cuotas sin pagar), `project_start` o `project_due`
(proyectos no cancelados). No guarda nada: sale de las tablas existentes. `id` es el de la fila de origen.

Montos en centavos (`budget_cents`, `amount_cents`), fechas `YYYY-MM-DD`. Errores: `{"code","error"}` con
404 `not_found`, 409 `client_has_projects` / `total_one_quote`, 422 `invalid_amount` / `bad_request`, 401 `unauthorized`.

## TUI rápida y con soporte sin conexión (caché local)

Con `TUNA_API` definido, la TUI **no** habla directo con el API: lee de un espejo SQLite local
(`~/.local/share/tunapm/cache-<hash>.db`, uno por servidor) y escribe en una cola que se envía en segundo plano.

- **Lecturas** instantáneas (salen del espejo). Al abrir se ve lo último sincronizado.
- **Escrituras** instantáneas: se aplican al espejo y se encolan. Lo creado sin conexión usa ids temporales y se
  cambia por el id real cuando el servidor responde.
- **Sincronización** cada ~20 s y tras cada cambio: primero envía la cola, y cuando está vacía trae el estado del
  servidor con `GET /api/snapshot` (ETag/304, gzip en PHP), así se ven los cambios hechos desde la web.
- **Sin conexión**: sigue funcionando; la barra superior muestra `● SIN CONEXIÓN · N pendientes` y se envía todo al volver la red.
- **Rechazos**: si el servidor rechaza un cambio (p. ej. borraste un cliente al que la web le agregó un proyecto), la TUI
  avisa y el espejo vuelve al estado del servidor. Si el mismo dato se editó en dos sitios a la vez, gana el último en llegar.
- Al salir intenta enviar lo pendiente (máx. ~4 s); si no hay red, queda para la próxima vez.

Comandos: `tuna status` (modo, conexión y cambios sin enviar) y `tuna sync` (fuerza un ciclo y lo reporta).
Usa una sola TUI a la vez por servidor (comparten el mismo espejo).
