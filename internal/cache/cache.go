// Package cache implementa store.Repo con un espejo SQLite local y una cola de
// cambios que se sincroniza con el API en segundo plano.
//
//   - Las lecturas salen del espejo (instantáneas).
//   - Las escrituras se aplican al espejo y se encolan; un worker las envía al API.
//     Sin conexión todo sigue funcionando y la cola se vacía al volver la red.
//   - Lo creado sin conexión usa ids temporales (>= TempBase) que se cambian por los
//     reales cuando el servidor responde.
//   - Cuando no quedan cambios pendientes, el espejo se reemplaza con el estado del
//     servidor (consulta barata con ETag), así se ven los cambios hechos desde la web.
package cache

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"tunapm/internal/client"
	"tunapm/internal/store"
)

// TempBase es el primer id que se asigna a filas creadas localmente.
const TempBase int64 = 1_000_000_000

func IsTemp(id int64) bool { return id >= TempBase }

// Estados que muestra la TUI.
const (
	StateConnecting = "conectando"
	StateOnline     = "en línea"
	StateOffline    = "sin conexión"
	StateError      = "error"
)

const syncInterval = 20 * time.Second

// Remote es lo que la caché necesita del API (lo cumple *client.Client).
type Remote interface {
	store.Repo
	SnapshotIfChanged(etag string) (store.Snapshot, string, bool, error)
}

type Repo struct {
	*store.Store // espejo: las lecturas se resuelven aquí
	api          Remote

	mu  sync.Mutex // serializa escrituras locales y cambios del espejo hechos por el worker
	gen uint64     // sube con cada escritura local (para descartar pulls obsoletos)

	syncMu    sync.Mutex // un ciclo de sincronización a la vez
	forcePull bool

	kick    chan struct{}
	changes chan struct{}
	stop    chan struct{}
	done    chan struct{}
	first   chan struct{}
	firstOK sync.Once
	closed  sync.Once

	pending atomic.Int64
	smu     sync.Mutex
	state   string
	detail  string
	notices []string
}

var _ store.Repo = (*Repo)(nil)

// Open abre el espejo en path y arranca la sincronización con api.
func Open(path string, api Remote) (*Repo, error) {
	st, err := store.Open(path)
	if err != nil {
		return nil, err
	}
	if _, err := st.DB().Exec(`
		CREATE TABLE IF NOT EXISTS outbox (
			seq    INTEGER PRIMARY KEY AUTOINCREMENT,
			op     TEXT NOT NULL,
			kind   TEXT NOT NULL,
			ent_id INTEGER NOT NULL,
			body   TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		st.Close()
		return nil, err
	}
	if err := st.ReserveIDRange(TempBase); err != nil {
		st.Close()
		return nil, err
	}
	r := &Repo{
		Store: st, api: api,
		kick: make(chan struct{}, 1), changes: make(chan struct{}, 1),
		stop: make(chan struct{}), done: make(chan struct{}), first: make(chan struct{}),
		state: StateConnecting,
	}
	r.refreshPending()
	go r.run()
	return r, nil
}

// PendingCount lee cuántos cambios hay sin enviar en el espejo de path (sin abrir el worker).
func PendingCount(path string) (int, error) {
	st, err := store.Open(path)
	if err != nil {
		return 0, err
	}
	defer st.Close()
	var n int
	if err := st.DB().QueryRow("SELECT COUNT(*) FROM outbox").Scan(&n); err != nil {
		return 0, nil // tabla aún no creada: nada pendiente
	}
	return n, nil
}

// ---------- API para la TUI ----------

// Changes avisa (sin bloquear) cuando el espejo cambió por la sincronización o cambió el estado.
func (r *Repo) Changes() <-chan struct{} { return r.changes }

// SyncStatus devuelve el estado de la conexión y cuántos cambios faltan por enviar.
func (r *Repo) SyncStatus() (state string, pending int, detail string) {
	r.smu.Lock()
	defer r.smu.Unlock()
	return r.state, int(r.pending.Load()), r.detail
}

// TakeNotices devuelve (y vacía) los avisos, p. ej. cambios que el servidor rechazó.
func (r *Repo) TakeNotices() []string {
	r.smu.Lock()
	defer r.smu.Unlock()
	n := r.notices
	r.notices = nil
	return n
}

// WaitFirstSync espera hasta d a que termine el primer ciclo de sincronización.
func (r *Repo) WaitFirstSync(d time.Duration) {
	select {
	case <-r.first:
	case <-time.After(d):
	}
}

// IsEmpty indica si el espejo aún no tiene datos (primer arranque).
func (r *Repo) IsEmpty() bool {
	var n int
	_ = r.DB().QueryRow("SELECT (SELECT COUNT(*) FROM clients) + (SELECT COUNT(*) FROM projects)").Scan(&n)
	return n == 0
}

// SyncNow ejecuta un ciclo completo (enviar cola + traer cambios) y devuelve su resultado.
func (r *Repo) SyncNow() error { return r.cycle() }

// Close intenta enviar lo pendiente (máx. ~4 s) y cierra el espejo.
func (r *Repo) Close() error {
	r.closed.Do(func() { close(r.stop) })
	fin := make(chan struct{})
	go func() { _ = r.cycle(); close(fin) }()
	select {
	case <-fin:
	case <-time.After(4 * time.Second):
		return nil // se abandona el intento; lo pendiente se enviará en la próxima sesión
	}
	<-r.done
	return r.Store.Close()
}

// ---------- escrituras (espejo + cola) ----------

func (r *Repo) write(kind, op string, local func() (int64, any, error)) error {
	r.mu.Lock()
	id, body, err := local()
	if err == nil {
		// Borrar algo que nunca llegó al servidor no necesita operación: purgeTemp ya
		// descartó su creación.
		if !(op == "delete" && IsTemp(id)) {
			err = r.enqueue(kind, op, id, body)
		}
		r.gen++
	}
	r.mu.Unlock()
	if err != nil {
		return err
	}
	select {
	case r.kick <- struct{}{}:
	default:
	}
	return nil
}

func (r *Repo) CreateClient(c *store.Client) error {
	return r.write("client", "create", func() (int64, any, error) {
		err := r.Store.CreateClient(c)
		return c.ID, c, err // el id se lee DESPUÉS de crear (id temporal del espejo)
	})
}

func (r *Repo) UpdateClient(c *store.Client) error {
	return r.write("client", "update", func() (int64, any, error) {
		return c.ID, c, r.Store.UpdateClient(c)
	})
}

func (r *Repo) DeleteClient(id int64) error {
	return r.write("client", "delete", func() (int64, any, error) {
		if err := r.Store.DeleteClient(id); err != nil {
			return id, nil, err
		}
		return id, nil, r.purgeTemp("client", id, nil)
	})
}

func (r *Repo) CreateProject(p *store.Project) error {
	return r.write("project", "create", func() (int64, any, error) {
		err := r.Store.CreateProject(p)
		return p.ID, p, err // el id se lee DESPUÉS de crear (id temporal del espejo)
	})
}

func (r *Repo) UpdateProject(p *store.Project) error {
	return r.write("project", "update", func() (int64, any, error) {
		return p.ID, p, r.Store.UpdateProject(p)
	})
}

func (r *Repo) DeleteProject(id int64) error {
	return r.write("project", "delete", func() (int64, any, error) {
		// Hijos creados sin conexión que se borrarán en cascada: su cola ya no tiene sentido.
		children := map[string][]int64{}
		if tasks, err := r.Store.ListTasks(store.TaskFilter{Mode: store.FilterAll, ProjectID: &id}); err == nil {
			for _, t := range tasks {
				children["task"] = append(children["task"], t.ID)
			}
		}
		if pays, err := r.Store.PaymentsByProject(id); err == nil {
			for _, p := range pays {
				children["payment"] = append(children["payment"], p.ID)
			}
		}
		if err := r.Store.DeleteProject(id); err != nil {
			return id, nil, err
		}
		if err := r.detachPendingNotes(id); err != nil {
			return id, nil, err
		}
		return id, nil, r.purgeTemp("project", id, children)
	})
}

func (r *Repo) CreateTask(t *store.Task) error {
	return r.write("task", "create", func() (int64, any, error) {
		err := r.Store.CreateTask(t)
		return t.ID, t, err // el id se lee DESPUÉS de crear (id temporal del espejo)
	})
}

func (r *Repo) UpdateTask(t *store.Task) error {
	return r.write("task", "update", func() (int64, any, error) {
		return t.ID, t, r.Store.UpdateTask(t)
	})
}

func (r *Repo) DeleteTask(id int64) error {
	return r.write("task", "delete", func() (int64, any, error) {
		if err := r.Store.DeleteTask(id); err != nil {
			return id, nil, err
		}
		return id, nil, r.purgeTemp("task", id, nil)
	})
}

func (r *Repo) CreateNote(n *store.Note) error {
	return r.write("note", "create", func() (int64, any, error) {
		err := r.Store.CreateNote(n)
		return n.ID, n, err // el id se lee DESPUÉS de crear (id temporal del espejo)
	})
}

func (r *Repo) UpdateNote(n *store.Note) error {
	return r.write("note", "update", func() (int64, any, error) {
		return n.ID, n, r.Store.UpdateNote(n)
	})
}

func (r *Repo) DeleteNote(id int64) error {
	return r.write("note", "delete", func() (int64, any, error) {
		if err := r.Store.DeleteNote(id); err != nil {
			return id, nil, err
		}
		return id, nil, r.purgeTemp("note", id, nil)
	})
}

// detachPendingNotes: al borrar un proyecto sus notas quedan sin proyecto, así que
// las notas aún en la cola no deben seguir apuntando a él.
func (r *Repo) detachPendingNotes(projectID int64) error {
	rows, err := r.DB().Query("SELECT seq, body FROM outbox WHERE kind = 'note' AND body <> ''")
	if err != nil {
		return err
	}
	fixes := map[int64]string{}
	for rows.Next() {
		var seq int64
		var body string
		if err := rows.Scan(&seq, &body); err != nil {
			rows.Close()
			return err
		}
		var m map[string]json.RawMessage
		var ref int64
		if json.Unmarshal([]byte(body), &m) != nil || json.Unmarshal(m["project_id"], &ref) != nil || ref != projectID {
			continue
		}
		m["project_id"] = json.RawMessage("null")
		nb, _ := json.Marshal(m)
		fixes[seq] = string(nb)
	}
	rows.Close()
	for seq, body := range fixes {
		if _, err := r.DB().Exec("UPDATE outbox SET body = ? WHERE seq = ?", body, seq); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) CreatePayment(p *store.Payment) error {
	return r.write("payment", "create", func() (int64, any, error) {
		err := r.Store.CreatePayment(p)
		return p.ID, p, err // el id se lee DESPUÉS de crear (id temporal del espejo)
	})
}

func (r *Repo) UpdatePayment(p *store.Payment) error {
	return r.write("payment", "update", func() (int64, any, error) {
		return p.ID, p, r.Store.UpdatePayment(p)
	})
}

func (r *Repo) MarkPaymentPaid(id int64, date string) error {
	return r.write("payment", "pay", func() (int64, any, error) {
		return id, map[string]string{"date": date}, r.Store.MarkPaymentPaid(id, date)
	})
}

func (r *Repo) DeletePayment(id int64) error {
	return r.write("payment", "delete", func() (int64, any, error) {
		if err := r.Store.DeletePayment(id); err != nil {
			return id, nil, err
		}
		return id, nil, r.purgeTemp("payment", id, nil)
	})
}

// ---------- cola ----------

type op struct {
	seq   int64
	op    string
	kind  string
	entID int64
	body  string
}

func (r *Repo) enqueue(kind, opName string, id int64, body any) error {
	raw := ""
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		raw = string(b)
	}
	if _, err := r.DB().Exec("INSERT INTO outbox (op, kind, ent_id, body) VALUES (?, ?, ?, ?)", opName, kind, id, raw); err != nil {
		return err
	}
	r.refreshPending()
	return nil
}

func (r *Repo) refreshPending() {
	var n int64
	_ = r.DB().QueryRow("SELECT COUNT(*) FROM outbox").Scan(&n)
	r.pending.Store(n)
}

// purgeTemp: al borrar algo que nunca llegó al servidor se descartan sus operaciones
// pendientes (y las de sus hijos), en vez de crear y borrar en el servidor.
// Debe llamarse con r.mu tomado y después de borrar en el espejo.
func (r *Repo) purgeTemp(kind string, id int64, children map[string][]int64) error {
	var err error
	if IsTemp(id) {
		_, err = r.DB().Exec("DELETE FROM outbox WHERE kind = ? AND ent_id = ?", kind, id)
	}
	for k, ids := range children {
		for _, cid := range ids {
			if IsTemp(cid) && err == nil {
				_, err = r.DB().Exec("DELETE FROM outbox WHERE kind = ? AND ent_id = ?", k, cid)
			}
		}
	}
	r.refreshPending()
	return err
}

func (r *Repo) nextOp() (op, bool, error) {
	var o op
	err := r.DB().QueryRow("SELECT seq, op, kind, ent_id, body FROM outbox ORDER BY seq LIMIT 1").
		Scan(&o.seq, &o.op, &o.kind, &o.entID, &o.body)
	if errors.Is(err, sql.ErrNoRows) {
		return o, false, nil
	}
	return o, err == nil, err
}

// rewriteRefs cambia, en las operaciones pendientes, un id temporal por el real.
func (r *Repo) rewriteRefs(kind string, oldID, newID int64) error {
	if _, err := r.DB().Exec("UPDATE outbox SET ent_id = ? WHERE kind = ? AND ent_id = ?", newID, kind, oldID); err != nil {
		return err
	}
	var childKinds []string
	var field string
	switch kind {
	case "client":
		childKinds, field = []string{"project"}, "client_id"
	case "project":
		childKinds, field = []string{"task", "payment", "note"}, "project_id"
	default:
		return nil
	}
	for _, ck := range childKinds {
		rows, err := r.DB().Query("SELECT seq, body FROM outbox WHERE kind = ? AND body <> ''", ck)
		if err != nil {
			return err
		}
		type fix struct {
			seq  int64
			body string
		}
		var fixes []fix
		for rows.Next() {
			var seq int64
			var body string
			if err := rows.Scan(&seq, &body); err != nil {
				rows.Close()
				return err
			}
			var m map[string]json.RawMessage
			if json.Unmarshal([]byte(body), &m) != nil {
				continue
			}
			var ref int64
			if json.Unmarshal(m[field], &ref) != nil || ref != oldID {
				continue
			}
			m[field], _ = json.Marshal(newID)
			nb, _ := json.Marshal(m)
			fixes = append(fixes, fix{seq, string(nb)})
		}
		rows.Close()
		for _, f := range fixes {
			if _, err := r.DB().Exec("UPDATE outbox SET body = ? WHERE seq = ?", f.body, f.seq); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Repo) meta(key string) string {
	var v string
	_ = r.DB().QueryRow("SELECT value FROM meta WHERE key = ?", key).Scan(&v)
	return v
}

func (r *Repo) setMeta(key, value string) {
	_, _ = r.DB().Exec("INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", key, value)
}

// ---------- sincronización ----------

func (r *Repo) run() {
	defer close(r.done)
	_ = r.cycle()
	t := time.NewTicker(syncInterval)
	defer t.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-r.kick:
			_ = r.cycle()
		case <-t.C:
			_ = r.cycle()
		}
	}
}

func (r *Repo) notify() {
	select {
	case r.changes <- struct{}{}:
	default:
	}
}

func (r *Repo) setState(state, detail string) {
	r.smu.Lock()
	changed := r.state != state || r.detail != detail
	r.state, r.detail = state, detail
	r.smu.Unlock()
	if changed {
		r.notify()
	}
}

func (r *Repo) addNotice(format string, args ...any) {
	r.smu.Lock()
	r.notices = append(r.notices, fmt.Sprintf(format, args...))
	r.smu.Unlock()
	r.notify()
}

// cycle envía la cola y, si quedó vacía, trae el estado del servidor.
func (r *Repo) cycle() error {
	r.syncMu.Lock()
	defer r.syncMu.Unlock()
	defer r.firstOK.Do(func() { close(r.first) })

	err := r.flush()
	if err == nil {
		err = r.pull()
	}
	switch {
	case err == nil:
		r.setState(StateOnline, "")
	case errors.Is(err, client.ErrOffline):
		r.setState(StateOffline, err.Error())
	default:
		r.setState(StateError, err.Error())
	}
	return err
}

// flush envía en orden las operaciones pendientes. Devuelve error solo si hay que
// reintentar más tarde (sin red, servidor caído, token inválido).
func (r *Repo) flush() error {
	for {
		r.mu.Lock()
		o, ok, err := r.nextOp()
		r.mu.Unlock()
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		newID, err := r.send(o)
		switch {
		case err == nil:
		case errors.Is(err, client.ErrOffline), errors.Is(err, client.ErrServer), errors.Is(err, client.ErrUnauthorized):
			return err
		case errors.Is(err, errDependency):
			r.forcePull = true // depende de algo que ya se rechazó: se descarta en silencio
		case errors.Is(err, store.ErrNotFound) && o.op != "create":
			// ya no existe en el servidor (borrado desde otro lado): nada que hacer
		default:
			r.forcePull = true
			r.addNotice("El servidor rechazó un cambio (%s de %s): %v", o.op, kindLabel(o.kind), err)
		}

		r.mu.Lock()
		if err == nil && o.op == "create" && newID != 0 && newID != o.entID {
			if rerr := r.Store.RemapID(o.kind, o.entID, newID); rerr != nil {
				r.mu.Unlock()
				return rerr
			}
			if rerr := r.rewriteRefs(o.kind, o.entID, newID); rerr != nil {
				r.mu.Unlock()
				return rerr
			}
		}
		_, derr := r.DB().Exec("DELETE FROM outbox WHERE seq = ?", o.seq)
		r.refreshPending()
		r.mu.Unlock()
		if derr != nil {
			return derr
		}
		r.notify()
	}
}

func kindLabel(kind string) string {
	switch kind {
	case "client":
		return "cliente"
	case "project":
		return "proyecto"
	case "task":
		return "tarea"
	case "note":
		return "nota"
	}
	return "cuota"
}

var errDependency = errors.New("depende de un elemento que no se sincronizó")

// send ejecuta una operación contra el API. Para creaciones devuelve el id real.
func (r *Repo) send(o op) (int64, error) {
	if IsTemp(o.entID) && o.op != "create" {
		return 0, errDependency // su creación fue rechazada y descartada
	}
	switch o.kind {
	case "client":
		var c store.Client
		if err := decodeBody(o, &c); err != nil {
			return 0, err
		}
		switch o.op {
		case "create":
			c.ID = 0
			err := r.api.CreateClient(&c)
			return c.ID, err
		case "update":
			c.ID = o.entID
			return 0, r.api.UpdateClient(&c)
		case "delete":
			return 0, r.api.DeleteClient(o.entID)
		}
	case "project":
		var p store.Project
		if err := decodeBody(o, &p); err != nil {
			return 0, err
		}
		if o.op != "delete" && IsTemp(p.ClientID) {
			return 0, errDependency
		}
		switch o.op {
		case "create":
			p.ID = 0
			err := r.api.CreateProject(&p)
			return p.ID, err
		case "update":
			p.ID = o.entID
			return 0, r.api.UpdateProject(&p)
		case "delete":
			return 0, r.api.DeleteProject(o.entID)
		}
	case "task":
		var t store.Task
		if err := decodeBody(o, &t); err != nil {
			return 0, err
		}
		if o.op != "delete" && IsTemp(t.ProjectID) {
			return 0, errDependency
		}
		switch o.op {
		case "create":
			t.ID = 0
			err := r.api.CreateTask(&t)
			return t.ID, err
		case "update":
			t.ID = o.entID
			return 0, r.api.UpdateTask(&t)
		case "delete":
			return 0, r.api.DeleteTask(o.entID)
		}
	case "note":
		var n store.Note
		if err := decodeBody(o, &n); err != nil {
			return 0, err
		}
		if o.op != "delete" && n.ProjectID != nil && IsTemp(*n.ProjectID) {
			return 0, errDependency
		}
		switch o.op {
		case "create":
			n.ID = 0
			err := r.api.CreateNote(&n)
			return n.ID, err
		case "update":
			n.ID = o.entID
			return 0, r.api.UpdateNote(&n)
		case "delete":
			return 0, r.api.DeleteNote(o.entID)
		}
	case "payment":
		switch o.op {
		case "pay":
			var b struct {
				Date string `json:"date"`
			}
			if err := decodeBody(o, &b); err != nil {
				return 0, err
			}
			return 0, r.api.MarkPaymentPaid(o.entID, b.Date)
		case "delete":
			return 0, r.api.DeletePayment(o.entID)
		}
		var p store.Payment
		if err := decodeBody(o, &p); err != nil {
			return 0, err
		}
		if IsTemp(p.ProjectID) {
			return 0, errDependency
		}
		switch o.op {
		case "create":
			p.ID = 0
			err := r.api.CreatePayment(&p)
			return p.ID, err
		case "update":
			p.ID = o.entID
			return 0, r.api.UpdatePayment(&p)
		}
	}
	return 0, fmt.Errorf("operación desconocida: %s %s", o.op, o.kind)
}

func decodeBody(o op, v any) error {
	if o.body == "" {
		return nil
	}
	return json.Unmarshal([]byte(o.body), v)
}

// pull reemplaza el espejo con el estado del servidor, solo si no hay nada pendiente.
func (r *Repo) pull() error {
	r.mu.Lock()
	gen := r.gen
	etag := r.meta("etag")
	if r.forcePull {
		etag = ""
	}
	pending := r.pending.Load()
	r.mu.Unlock()
	if pending > 0 {
		return nil
	}

	snap, tag, changed, err := r.api.SnapshotIfChanged(etag)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.gen != gen || r.pending.Load() > 0 {
		return nil // hubo cambios locales mientras tanto: se reintenta en el próximo ciclo
	}
	if err := r.Store.ReplaceAll(snap); err != nil {
		return err
	}
	r.setMeta("etag", tag)
	r.forcePull = false
	r.notify()
	return nil
}
