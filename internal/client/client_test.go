package client_test

import (
	"errors"
	"net/http/httptest"
	"os"
	"testing"

	"tunapm/internal/api"
	"tunapm/internal/client"
	"tunapm/internal/store"
)

// Si TUNA_TEST_URL está definido, los tests corren contra ese servidor (por ejemplo
// el API en PHP sobre una base vacía) para verificar que respeta el mismo contrato.
func setup(t *testing.T) *client.Client {
	t.Helper()
	if url := os.Getenv("TUNA_TEST_URL"); url != "" {
		return client.New(url, os.Getenv("TUNA_TEST_TOKEN"))
	}
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	srv := httptest.NewServer(api.New(st, "secreto"))
	t.Cleanup(srv.Close)
	return client.New(srv.URL, "secreto")
}

func TestRoundTrip(t *testing.T) {
	c := setup(t)

	cl := store.Client{Name: "Ana", Company: "Acme"}
	if err := c.CreateClient(&cl); err != nil || cl.ID == 0 {
		t.Fatalf("CreateClient: %v %+v", err, cl)
	}
	due := "2026-12-31"
	p := store.Project{ClientID: cl.ID, Name: "Web", Status: store.StatusActivo, BudgetCents: 50000,
		Currency: "GTQ", BillingType: store.BillingParcial, DueDate: &due}
	if err := c.CreateProject(&p); err != nil {
		t.Fatal(err)
	}
	got, err := c.GetProject(p.ID)
	if err != nil || got.ClientName != "Ana" || got.DueDate == nil || *got.DueDate != due {
		t.Fatalf("GetProject: %v %+v", err, got)
	}

	task := store.Task{ProjectID: p.ID, Title: "Diseño", Status: store.TaskPendiente, Priority: 2}
	if err := c.CreateTask(&task); err != nil {
		t.Fatal(err)
	}
	task.Status = store.TaskHecho
	if err := c.UpdateTask(&task); err != nil || task.CompletedAt == nil {
		t.Fatalf("UpdateTask: %v %+v", err, task)
	}
	tasks, _ := c.ListTasks(store.TaskFilter{Mode: store.FilterAll, ProjectID: &p.ID})
	if len(tasks) != 1 {
		t.Fatalf("ListTasks = %d", len(tasks))
	}
	prog, _ := c.TaskProgress()
	if prog[p.ID].Done != 1 || prog[p.ID].Total != 1 {
		t.Fatalf("progress = %+v", prog)
	}

	pay := store.Payment{ProjectID: p.ID, AmountCents: 10000, ScheduledDate: "2026-12-01"}
	if err := c.CreatePayment(&pay); err != nil {
		t.Fatal(err)
	}
	if err := c.MarkPaymentPaid(pay.ID, "2026-12-02"); err != nil {
		t.Fatal(err)
	}
	totals, _ := c.AllProjectTotals()
	if totals[p.ID].PaidCents != 10000 {
		t.Fatalf("totals = %+v", totals)
	}
}

func TestErrorsMapToSentinels(t *testing.T) {
	c := setup(t)
	if _, err := c.GetClient(99); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	cl := store.Client{Name: "Ana"}
	_ = c.CreateClient(&cl)
	p := store.Project{ClientID: cl.ID, Name: "X", Status: "activo", Currency: "GTQ", BillingType: "total"}
	_ = c.CreateProject(&p)
	if err := c.DeleteClient(cl.ID); !errors.Is(err, store.ErrClientHasProjects) {
		t.Fatalf("want ErrClientHasProjects, got %v", err)
	}
	bad := store.Payment{ProjectID: p.ID, AmountCents: 0, ScheduledDate: "2026-01-01"}
	if err := c.CreatePayment(&bad); !errors.Is(err, store.ErrInvalidAmount) {
		t.Fatalf("want ErrInvalidAmount, got %v", err)
	}
}

func TestAuth(t *testing.T) {
	url := os.Getenv("TUNA_TEST_URL")
	if url == "" {
		st, _ := store.Open(":memory:")
		defer st.Close()
		srv := httptest.NewServer(api.New(st, "secreto"))
		defer srv.Close()
		url = srv.URL
	}
	if _, err := client.New(url, "malo").ListClients(); err == nil {
		t.Fatal("esperaba error de token")
	}
}

func TestSnapshotAndETag(t *testing.T) {
	c := setup(t)
	cl := store.Client{Name: "Snap"}
	if err := c.CreateClient(&cl); err != nil {
		t.Fatal(err)
	}
	snap, tag, changed, err := c.SnapshotIfChanged("")
	if err != nil || !changed || tag == "" {
		t.Fatalf("primera lectura: %v changed=%v tag=%q", err, changed, tag)
	}
	found := false
	for _, x := range snap.Clients {
		found = found || x.ID == cl.ID
	}
	if !found || snap.Projects == nil || snap.Tasks == nil || snap.Payments == nil {
		t.Fatalf("snapshot incompleto: %+v", snap)
	}
	if _, tag2, changed, err := c.SnapshotIfChanged(tag); err != nil || changed || tag2 != tag {
		t.Fatalf("con el mismo ETag debe responder 304: %v changed=%v", err, changed)
	}
	extra := store.Client{Name: "Otro"}
	_ = c.CreateClient(&extra)
	if _, _, changed, _ := c.SnapshotIfChanged(tag); !changed {
		t.Fatal("tras un cambio el ETag debe cambiar")
	}
}
