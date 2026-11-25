# tunapm

TUI local en Go para llevar clientes, proyectos, tareas y cuotas de cobro.
Sin servicios externos: todo vive en SQLite en tu máquina.

## Requisitos

- Go 1.27 o superior

## Compilar y ejecutar

```sh
go build -o tuna ./cmd/tuna
./tuna
```

La base de datos se crea en `~/.local/share/tunapm/tuna.db`
(respetando `$XDG_DATA_HOME`). Puedes usar otra ruta:

```sh
./tuna --db /tmp/prueba.db
```

Para probar sin capturar datos:

```sh
./tuna --db /tmp/prueba.db seed
./tuna --db /tmp/prueba.db
```

## Vistas

Arriba están las pestañas `1 Tareas · 2 Proyectos · 3 Clientes · 4 Cobros · 5 Notas · 6 Calendario`.
La vista principal es Tareas. Abajo, la barra de estado muestra el filtro y
los atajos de la vista actual.

### Tareas

| Tecla | Acción |
|---|---|
| `j` / `k` | mover |
| `tab` | abiertas / todas / hechas |
| `/` | buscar |
| `f` | filtrar por proyecto o cliente |
| `n` | nueva tarea |
| `espacio` | marcar hecha / reabrir |
| `s` | rotar estado |
| `+` / `-` | subir / bajar prioridad |
| `e` | editar |
| `enter` | detalle y notas |
| `d` | borrar (confirma) |
| `g` | ir al proyecto |
| `esc` | limpiar filtros |

### Proyectos

`j`/`k` mueven, `n` crea, `e` edita, `S` cambia estado, `c` agrega cuota,
`x` paga la siguiente cuota pendiente, `enter` abre las tareas del proyecto y
`d` borra (con sus tareas y cuotas).

### Clientes

`j`/`k` mueven, `n` crea, `e` edita, `d` borra (avisa si tiene proyectos) y
`enter` abre sus tareas.

### Cobros

Agrupados en vencidos, próximos 30 días, más adelante y pagados recientes,
con totales por moneda. `j`/`k` mueven y `x` marca la cuota como pagada.

### Notas

Notas libres, con o sin proyecto, organizadas por etiquetas. `n` crea, `e`
edita, `d` borra y `enter` la abre completa. `t` filtra por etiqueta, `f` por
proyecto (o "sin proyecto"), `/` busca y `esc` limpia los filtros. Desde
Proyectos, `o` abre las notas del proyecto seleccionado. Las etiquetas se
escriben separadas por espacio o coma y se guardan en minúsculas.

### Calendario

Cuadrícula del mes a la izquierda y agenda a la derecha. Junta, por fecha, los
eventos, las tareas con fecha límite, las cuotas por cobrar y el inicio y la
entrega de cada proyecto. `←`/`→` mueven un día, `↑`/`↓` una semana, `[`/`]`
un mes y `t` vuelve a hoy. `v` cambia la agenda entre mes, semana y día, y
`tab` recorre las entradas del día. `n` crea un evento en el día seleccionado
(hora, proyecto y cliente son opcionales); `e` lo edita y `d` lo borra. En una
tarea, cobro o proyecto, `enter` lleva a su vista; `espacio` marca la tarea
hecha y `x` el cobro pagado.

## Paleta de comandos

`ctrl+k` o `:` abre la paleta. Busca comandos, clientes, proyectos y tareas, o
crea registros al vuelo:

```
t Diseñar logo @web-acme !alta ^viernes
c Juan Pérez #Acme
p Sitio web @acme $5000 usd ^2026-12-20
$ 2500 @web-acme ^15/12
n Idea para el home @web-acme #idea #diseño
e Reunión de avance @web-acme ^viernes 15:30
```

- `@` proyecto (o cliente, al crear un proyecto), con búsqueda difusa; usa
  guiones para nombres con espacios (`@web-acme`).
- `#` empresa del cliente; en una nota, cada `#palabra` es una etiqueta; en un evento, `#tipo`
  (`#llamada`, `#presentacion`, `#lanzamiento`). Sin `#tipo` se deduce del título.
- `!baja|media|alta|urgente`.
- `^fecha`: `hoy`, `mañana`, `lun`…`dom`, `+3d`, `+2s`, `15/12`,
  `2026-12-15`.
- `$monto` y `q`/`gtq` o `usd`; GTQ por defecto.

Si falta un dato obligatorio, al pulsar `enter` se abre el formulario con lo
que ya escribiste. `tab` abre el formulario directamente.

## Reglas de cobro

- Los montos se guardan en centavos y se muestran como `Q 12,500.00` o
  `$ 1,200.00`.
- Cobro **total**: una sola cuota por proyecto.
- Cobro **parcial**: varias cuotas; el proyecto muestra *Cobrado*,
  *Pendiente* y *Saldo sin programar*.

## Pruebas

```sh
go vet ./...
go test ./...
```

## Estructura

```
cmd/tuna/          arranque, flags y datos de ejemplo
internal/store/    SQLite, migración y CRUD
internal/money/    formato y parseo de montos
internal/dates/    fechas rápidas y formato relativo
internal/quickadd/ sintaxis de creación rápida
internal/ui/       modelo Bubble Tea, vistas, paleta y formularios
```
