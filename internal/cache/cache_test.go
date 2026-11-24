package cache_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"tunapm/internal/api"
	"tunapm/internal/cache"
	"tunapm/internal/client"
	"tunapm/internal/store"
)

type env struct {
	server *store.Store // "el servidor": base real detrás del API
	down   *atomic.Bool // true = el API responde 503 (sin servicio)
	repo   *cache.Repo
}

func newEnv(t *testing.T, offline bool) *env {
	t.Helper()
	srvStore, err := store.Open(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srvStore.Close() })

	down := &atomic.Bool{}
	down.Store(offline)
	inner := api.New(srvStore, "tok")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if down.Load() {
			http.Error(w, "caído", http.StatusServiceUnavailable)
			return
		}
		inner.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	repo, err := cache.Open(filepath.Join(t.TempDir(), "cache.db"), client.New(srv.URL, "tok"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })
	return &env{server: srvStore, down: down, repo: repo}
}

// settle ejecuta ciclos hasta que cond sea cierta (el worker en segundo plano también corre).
func settle(t *testing.T, e *env, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		_ = e.repo.SyncNow()
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("la sincronización no llegó al estado esperado")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func pending(e *env) int { _, n, _ := e.repo.SyncStatus(); return n }

func TestOnlineCreateIsInstantThenGetsRealID(t *testing.T) {
	e := newEnv(t, false)
	c := store.Client{Name: "Ana"}
	if err := e.repo.CreateClient(&c); err != nil {
		t.Fatal(err)
	}
	if !cache.IsTemp(c.ID) {
		t.Fatalf("id local debería ser temporal, got %d", c.ID)
	}
	if got, _ := e.repo.ListClients(); len(got) != 1 {
		t.Fatalf("lectura local inmediata: %d clientes", len(got))
	}
	settle(t, e, func() bool { return pending(e) == 0 })

	local, _ := e.repo.ListClients()
	remote, _ := e.server.ListClients()
	if len(local) != 1 || len(remote) != 1 || cache.IsTemp(local[0].ID) || local[0].ID != remote[0].ID {
		t.Fatalf("ids tras sincronizar: local=%+v remoto=%+v", local, remote)
	}
}

func TestOfflineWorkThenSync(t *testing.T) {
	e := newEnv(t, true)
	_ = e.repo.SyncNow()
	if state, _, _ := e.repo.SyncStatus(); state != cache.StateOffline {
		t.Fatalf("estado = %q, want offline", state)
	}

	c := store.Client{Name: "Ana"}
	must(t, e.repo.CreateClient(&c))
	p := store.Project{ClientID: c.ID, Name: "Web", Status: "activo", Currency: "GTQ", BillingType: store.BillingParcial}
	must(t, e.repo.CreateProject(&p))
	task := store.Task{ProjectID: p.ID, Title: "Diseño", Status: store.TaskPendiente, Priority: 2}
	must(t, e.repo.CreateTask(&task))
	pay := store.Payment{ProjectID: p.ID, AmountCents: 50000, ScheduledDate: "2026-12-01"}
	must(t, e.repo.CreatePayment(&pay))
	task.Status = store.TaskHecho
	must(t, e.repo.UpdateTask(&task))
	must(t, e.repo.MarkPaymentPaid(pay.ID, "2026-12-02"))

	// Todo se ve local y hay 6 cambios pendientes.
	if tasks, _ := e.repo.ListTasks(store.TaskFilter{Mode: store.FilterAll}); len(tasks) != 1 || tasks[0].ClientName != "Ana" {
		t.Fatalf("tareas locales: %+v", tasks)
	}
	if n := pending(e); n != 6 {
		t.Fatalf("pendientes = %d, want 6", n)
	}
	if remote, _ := e.server.ListClients(); len(remote) != 0 {
		t.Fatal("el servidor no debería tener nada todavía")
	}

	e.down.Store(false) // vuelve la conexión
	settle(t, e, func() bool { return pending(e) == 0 })

	clients, _ := e.server.ListClients()
	projects, _ := e.server.ListProjects()
	tasks, _ := e.server.ListTasks(store.TaskFilter{Mode: store.FilterAll})
	pays, _ := e.server.ListPayments()
	if len(clients) != 1 || len(projects) != 1 || len(tasks) != 1 || len(pays) != 1 {
		t.Fatalf("servidor: %d/%d/%d/%d", len(clients), len(projects), len(tasks), len(pays))
	}
	if projects[0].ClientID != clients[0].ID || tasks[0].ProjectID != projects[0].ID || pays[0].ProjectID != projects[0].ID {
		t.Fatalf("referencias mal remapeadas: %+v %+v %+v", projects[0], tasks[0], pays[0])
	}
	if tasks[0].Status != store.TaskHecho || tasks[0].CompletedAt == nil || pays[0].PaidDate == nil {
		t.Fatalf("estados no llegaron: %+v %+v", tasks[0], pays[0])
	}
	// El espejo quedó con los mismos ids reales.
	lt, _ := e.repo.ListTasks(store.TaskFilter{Mode: store.FilterAll})
	if len(lt) != 1 || lt[0].ID != tasks[0].ID || lt[0].ProjectID != projects[0].ID {
		t.Fatalf("espejo tras sincronizar: %+v", lt)
	}
	if state, _, _ := e.repo.SyncStatus(); state != cache.StateOnline {
		t.Fatalf("estado = %q, want en línea", state)
	}
}

func TestPullBringsRemoteChangesAndNotifies(t *testing.T) {
	e := newEnv(t, false)
	settle(t, e, func() bool { return true })
	// Vaciar avisos previos.
	for len(e.repo.Changes()) > 0 {
		<-e.repo.Changes()
	}
	remote := store.Client{Name: "Desde la web"}
	must(t, e.server.CreateClient(&remote))

	settle(t, e, func() bool { l, _ := e.repo.ListClients(); return len(l) == 1 })
	select {
	case <-e.repo.Changes():
	case <-time.After(time.Second):
		t.Fatal("no hubo aviso de cambios")
	}
}

func TestServerRejectionIsReportedAndStateRestored(t *testing.T) {
	e := newEnv(t, false)
	c := store.Client{Name: "Ana"}
	must(t, e.server.CreateClient(&c))
	p := store.Project{ClientID: c.ID, Name: "P1", Status: "activo", Currency: "GTQ", BillingType: "total"}
	must(t, e.server.CreateProject(&p))
	settle(t, e, func() bool { l, _ := e.repo.ListProjects(); return len(l) == 1 })

	// El servidor gana un proyecto que el espejo no conoce…
	p2 := store.Project{ClientID: c.ID, Name: "P2", Status: "activo", Currency: "GTQ", BillingType: "total"}
	must(t, e.server.CreateProject(&p2))
	// …y localmente se borra P1 y luego el cliente (el espejo lo permite).
	must(t, e.repo.DeleteProject(p.ID))
	must(t, e.repo.DeleteClient(c.ID))

	settle(t, e, func() bool { l, _ := e.repo.ListClients(); return pending(e) == 0 && len(l) == 1 })
	notices := e.repo.TakeNotices()
	if len(notices) == 0 {
		t.Fatal("debía avisar que el servidor rechazó el borrado")
	}
	if l, _ := e.repo.ListProjects(); len(l) != 1 || l[0].Name != "P2" {
		t.Fatalf("el espejo debía recuperar el estado del servidor: %+v", l)
	}
}

func TestDeletingUnsyncedItemCancelsItsOperations(t *testing.T) {
	e := newEnv(t, true)
	c := store.Client{Name: "Ana"}
	must(t, e.repo.CreateClient(&c))
	p := store.Project{ClientID: c.ID, Name: "Web", Status: "activo", Currency: "GTQ", BillingType: "total"}
	must(t, e.repo.CreateProject(&p))
	task := store.Task{ProjectID: p.ID, Title: "x", Status: "pendiente", Priority: 1}
	must(t, e.repo.CreateTask(&task))
	if n := pending(e); n != 3 {
		t.Fatalf("pendientes = %d", n)
	}
	must(t, e.repo.DeleteProject(p.ID)) // borra proyecto y tarea nunca enviados
	if n := pending(e); n != 1 {
		t.Fatalf("tras borrar el proyecto debe quedar solo la creación del cliente, hay %d", n)
	}
	e.down.Store(false)
	settle(t, e, func() bool { return pending(e) == 0 })
	if projects, _ := e.server.ListProjects(); len(projects) != 0 {
		t.Fatalf("el servidor no debía recibir el proyecto: %+v", projects)
	}
	if clients, _ := e.server.ListClients(); len(clients) != 1 {
		t.Fatalf("el cliente sí debía llegar: %+v", clients)
	}
}

func TestLocalValidationQueuesNothing(t *testing.T) {
	e := newEnv(t, true)
	c := store.Client{Name: "Ana"}
	must(t, e.repo.CreateClient(&c))
	p := store.Project{ClientID: c.ID, Name: "Web", Status: "activo", Currency: "GTQ", BillingType: "total"}
	must(t, e.repo.CreateProject(&p))
	before := pending(e)
	bad := store.Payment{ProjectID: p.ID, AmountCents: 0, ScheduledDate: "2026-12-01"}
	if err := e.repo.CreatePayment(&bad); !errors.Is(err, store.ErrInvalidAmount) {
		t.Fatalf("err = %v", err)
	}
	if pending(e) != before {
		t.Fatal("una operación inválida no debe encolarse")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// Con un servidor lento (200 ms por petición), leer y escribir en la TUI no debe esperar a la red.
func TestReadsAndWritesDoNotWaitForNetwork(t *testing.T) {
	srvStore, _ := store.Open(filepath.Join(t.TempDir(), "s.db"))
	defer srvStore.Close()
	inner := api.New(srvStore, "tok")
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		inner.ServeHTTP(w, r)
	}))
	defer slow.Close()
	repo, err := cache.Open(filepath.Join(t.TempDir(), "c.db"), client.New(slow.URL, "tok"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	c := store.Client{Name: "Ana"}
	start := time.Now()
	must(t, repo.CreateClient(&c))
	p := store.Project{ClientID: c.ID, Name: "Web", Status: "activo", Currency: "GTQ", BillingType: "parcial"}
	must(t, repo.CreateProject(&p))
	for i := 0; i < 10; i++ { // ~lo que hace un "recargar todo" de la TUI
		_, _ = repo.ListClients()
		_, _ = repo.ListProjects()
		_, _ = repo.ListTasks(store.TaskFilter{Mode: store.FilterAll})
		_, _ = repo.ListPayments()
		_, _ = repo.AllProjectTotals()
		_, _ = repo.TaskProgress()
	}
	if elapsed := time.Since(start); elapsed > 150*time.Millisecond {
		t.Fatalf("lecturas/escrituras tardaron %v con servidor lento: esperan a la red", elapsed)
	}
}
