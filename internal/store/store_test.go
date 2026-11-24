package store

import (
	"errors"
	"fmt"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func ptr(s string) *string { return &s }
func i64(v int64) *int64   { return &v }

func mustClient(t *testing.T, s *Store, name string) Client {
	t.Helper()
	c := Client{Name: name, Company: "Acme"}
	if err := s.CreateClient(&c); err != nil {
		t.Fatalf("CreateClient: %v", err)
	}
	return c
}

func mustProject(t *testing.T, s *Store, clientID int64, billing string) Project {
	t.Helper()
	p := Project{
		ClientID:    clientID,
		Name:        "Sitio web",
		Status:      StatusActivo,
		BudgetCents: 100000,
		Currency:    CurrencyGTQ,
		BillingType: billing,
	}
	if err := s.CreateProject(&p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	return p
}

func mustTask(t *testing.T, s *Store, projectID int64, title string, priority int, due *string) Task {
	t.Helper()
	tk := Task{ProjectID: projectID, Title: title, Status: TaskPendiente, Priority: priority, DueDate: due}
	if err := s.CreateTask(&tk); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	return tk
}

func TestClientCRUD(t *testing.T) {
	s := newTestStore(t)
	c := mustClient(t, s, "Juan Pérez")

	got, err := s.GetClient(c.ID)
	if err != nil || got.Name != "Juan Pérez" || got.CreatedAt == "" {
		t.Fatalf("GetClient = %+v, %v", got, err)
	}

	got.Phone = "5555-5555"
	if err := s.UpdateClient(&got); err != nil {
		t.Fatalf("UpdateClient: %v", err)
	}
	if got, _ = s.GetClient(c.ID); got.Phone != "5555-5555" {
		t.Fatalf("teléfono no persistió: %+v", got)
	}

	if err := s.DeleteClient(c.ID); err != nil {
		t.Fatalf("DeleteClient: %v", err)
	}
	if _, err := s.GetClient(c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("esperaba ErrNotFound, obtuve %v", err)
	}
}

func TestDeleteClientWithProjectsFails(t *testing.T) {
	s := newTestStore(t)
	c := mustClient(t, s, "Acme")
	mustProject(t, s, c.ID, BillingTotal)

	if err := s.DeleteClient(c.ID); !errors.Is(err, ErrClientHasProjects) {
		t.Fatalf("esperaba ErrClientHasProjects, obtuve %v", err)
	}
}

func TestProjectCascade(t *testing.T) {
	s := newTestStore(t)
	c := mustClient(t, s, "Acme")
	p := mustProject(t, s, c.ID, BillingParcial)
	task := mustTask(t, s, p.ID, "Diseñar", 1, nil)
	payment := Payment{ProjectID: p.ID, AmountCents: 5000, ScheduledDate: "2026-12-01"}
	if err := s.CreatePayment(&payment); err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}

	if err := s.DeleteProject(p.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := s.GetTask(task.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("tarea no se borró en cascada: %v", err)
	}
	if _, err := s.GetPayment(payment.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cuota no se borró en cascada: %v", err)
	}
}

func TestListTasksFilterAndOrder(t *testing.T) {
	s := newTestStore(t)
	c := mustClient(t, s, "Acme")
	p1 := mustProject(t, s, c.ID, BillingTotal)
	p2 := Project{ClientID: c.ID, Name: "App", Status: StatusActivo, BudgetCents: 1, Currency: CurrencyUSD, BillingType: BillingTotal}
	if err := s.CreateProject(&p2); err != nil {
		t.Fatal(err)
	}

	low := mustTask(t, s, p1.ID, "Poca cosa", 0, nil)
	urgent := mustTask(t, s, p1.ID, "Urgente", 3, ptr("2026-12-20"))
	soon := mustTask(t, s, p2.ID, "Pronto", 1, ptr("2026-11-25"))
	done := mustTask(t, s, p2.ID, "Hecha", 2, nil)
	done.Status = TaskHecho
	if err := s.UpdateTask(&done); err != nil {
		t.Fatal(err)
	}

	open, err := s.ListTasks(TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []int64{urgent.ID, soon.ID, low.ID}
	if len(open) != len(wantOrder) {
		t.Fatalf("abiertas = %d, esperaba %d", len(open), len(wantOrder))
	}
	for i, id := range wantOrder {
		if open[i].ID != id {
			t.Fatalf("orden[%d] = %d, esperaba %d", i, open[i].ID, id)
		}
	}
	if open[0].ProjectName == "" || open[0].ClientName != "Acme" {
		t.Fatalf("faltan nombres del JOIN: %+v", open[0])
	}

	all, _ := s.ListTasks(TaskFilter{Mode: FilterAll})
	if len(all) != 4 || all[3].ID != done.ID {
		t.Fatalf("todas = %+v", all)
	}
	onlyDone, _ := s.ListTasks(TaskFilter{Mode: FilterDone})
	if len(onlyDone) != 1 || onlyDone[0].Status != TaskHecho || onlyDone[0].CompletedAt == nil {
		t.Fatalf("hechas = %+v", onlyDone)
	}

	byProject, _ := s.ListTasks(TaskFilter{ProjectID: i64(p2.ID)})
	if len(byProject) != 1 || byProject[0].ID != soon.ID {
		t.Fatalf("por proyecto = %+v", byProject)
	}
	byClient, _ := s.ListTasks(TaskFilter{ClientID: i64(c.ID), Search: "urgente"})
	if len(byClient) != 1 || byClient[0].ID != urgent.ID {
		t.Fatalf("búsqueda = %+v", byClient)
	}
}

func TestTaskCompletionTimestamp(t *testing.T) {
	s := newTestStore(t)
	c := mustClient(t, s, "Acme")
	p := mustProject(t, s, c.ID, BillingTotal)
	task := mustTask(t, s, p.ID, "Diseñar", 1, nil)

	task.Status = TaskHecho
	if err := s.UpdateTask(&task); err != nil {
		t.Fatal(err)
	}
	if task.CompletedAt == nil {
		t.Fatal("completed_at no se fijó al marcar hecha")
	}
	task.Status = TaskEnCurso
	if err := s.UpdateTask(&task); err != nil {
		t.Fatal(err)
	}
	if task.CompletedAt != nil {
		t.Fatal("completed_at no se limpió al reabrir")
	}
}

func TestPaymentRules(t *testing.T) {
	s := newTestStore(t)
	c := mustClient(t, s, "Acme")

	total := mustProject(t, s, c.ID, BillingTotal)
	if err := s.CreatePayment(&Payment{ProjectID: total.ID, AmountCents: 100000, ScheduledDate: "2026-12-01"}); err != nil {
		t.Fatalf("primera cuota: %v", err)
	}
	if err := s.CreatePayment(&Payment{ProjectID: total.ID, AmountCents: 1000, ScheduledDate: "2027-01-01"}); !errors.Is(err, ErrTotalBillingOneQuote) {
		t.Fatalf("esperaba ErrTotalBillingOneQuote, obtuve %v", err)
	}

	partial := mustProject(t, s, c.ID, BillingParcial)
	for i, amount := range []int64{40000, 60000} {
		if err := s.CreatePayment(&Payment{ProjectID: partial.ID, AmountCents: amount, ScheduledDate: fmt.Sprintf("2026-12-0%d", i+1)}); err != nil {
			t.Fatalf("cuota parcial %d: %v", i, err)
		}
	}
	if err := s.CreatePayment(&Payment{ProjectID: partial.ID, AmountCents: 0, ScheduledDate: "2026-12-05"}); !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("esperaba ErrInvalidAmount, obtuve %v", err)
	}

	// Cambiar a cobro total con dos cuotas ya programadas no se permite.
	partial.BillingType = BillingTotal
	if err := s.UpdateProject(&partial); !errors.Is(err, ErrTotalBillingOneQuote) {
		t.Fatalf("esperaba ErrTotalBillingOneQuote al actualizar, obtuve %v", err)
	}
}

func TestAggregates(t *testing.T) {
	s := newTestStore(t)
	c := mustClient(t, s, "Acme")
	p1 := mustProject(t, s, c.ID, BillingParcial)
	p2 := mustProject(t, s, c.ID, BillingTotal)

	done := mustTask(t, s, p1.ID, "Hecha", 1, nil)
	done.Status = TaskHecho
	if err := s.UpdateTask(&done); err != nil {
		t.Fatal(err)
	}
	mustTask(t, s, p1.ID, "Pendiente", 2, nil)
	mustTask(t, s, p2.ID, "Otra", 0, nil)

	progress, err := s.TaskProgress()
	if err != nil {
		t.Fatal(err)
	}
	if got := progress[p1.ID]; got.Done != 1 || got.Total != 2 {
		t.Fatalf("progreso p1 = %+v", got)
	}
	if got := progress[p2.ID]; got.Done != 0 || got.Total != 1 {
		t.Fatalf("progreso p2 = %+v", got)
	}

	pay := Payment{ProjectID: p1.ID, AmountCents: 1000, ScheduledDate: "2026-12-01"}
	if err := s.CreatePayment(&pay); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkPaymentPaid(pay.ID, "2026-12-02"); err != nil {
		t.Fatal(err)
	}
	totals, err := s.AllProjectTotals()
	if err != nil {
		t.Fatal(err)
	}
	if got := totals[p1.ID]; got.ScheduledCents != 1000 || got.PaidCents != 1000 {
		t.Fatalf("totales p1 = %+v", got)
	}
	if _, ok := totals[p2.ID]; ok {
		t.Fatalf("p2 no debería tener totales: %+v", totals)
	}
}

func TestPaymentStatusAndTotals(t *testing.T) {
	s := newTestStore(t)
	c := mustClient(t, s, "Acme")
	p := mustProject(t, s, c.ID, BillingParcial)

	paid := Payment{ProjectID: p.ID, AmountCents: 25000, ScheduledDate: "2026-11-10"}
	if err := s.CreatePayment(&paid); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkPaymentPaid(paid.ID, "2026-11-12"); err != nil {
		t.Fatal(err)
	}
	late := Payment{ProjectID: p.ID, AmountCents: 30000, ScheduledDate: "2026-11-01"}
	if err := s.CreatePayment(&late); err != nil {
		t.Fatal(err)
	}
	upcoming := Payment{ProjectID: p.ID, AmountCents: 45000, ScheduledDate: "2026-12-20"}
	if err := s.CreatePayment(&upcoming); err != nil {
		t.Fatal(err)
	}

	const today = "2026-11-22"
	payments, err := s.PaymentsByProject(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int64]string{paid.ID: PaymentPagado, late.ID: PaymentVencido, upcoming.ID: PaymentPendiente}
	for _, pay := range payments {
		if got := pay.StatusToday(today); got != want[pay.ID] {
			t.Fatalf("cuota %d: estado %q, esperaba %q", pay.ID, got, want[pay.ID])
		}
		if pay.Currency != CurrencyGTQ || pay.ProjectName == "" || pay.ClientName != "Acme" {
			t.Fatalf("faltan datos del JOIN: %+v", pay)
		}
	}

	totals, err := s.ProjectTotals(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if totals.ScheduledCents != 100000 || totals.PaidCents != 25000 {
		t.Fatalf("totales = %+v", totals)
	}
	if got := totals.UnassignedCents(p.BudgetCents); got != 0 {
		t.Fatalf("saldo sin programar = %d, esperaba 0", got)
	}

	all, _ := s.ListPayments()
	if len(all) != 3 || all[0].ID != late.ID {
		t.Fatalf("orden de cuotas = %+v", all)
	}
}

func TestNotes(t *testing.T) {
	s := newTestStore(t)
	c := mustClient(t, s, "Ana")
	p := mustProject(t, s, c.ID, BillingTotal)

	if err := s.CreateNote(&Note{Title: "  "}); !errors.Is(err, ErrNoteTitle) {
		t.Fatalf("título vacío: %v", err)
	}
	loose := Note{Title: "Idea suelta", Body: "texto", Tags: []string{"#Idea", "idea", "Para Revisar", ""}}
	if err := s.CreateNote(&loose); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(loose.Tags); got != "[idea para-revisar]" {
		t.Fatalf("etiquetas normalizadas = %s", got)
	}
	linked := Note{Title: "Acta de reunión", ProjectID: &p.ID, Tags: []string{"reunion", "idea"}}
	if err := s.CreateNote(&linked); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetNote(linked.ID)
	if err != nil || got.ProjectName != p.Name || got.ClientName != "Ana" || got.ProjectID == nil {
		t.Fatalf("GetNote: %v %+v", err, got)
	}
	count := func(f NoteFilter) int {
		notes, err := s.ListNotes(f)
		if err != nil {
			t.Fatal(err)
		}
		return len(notes)
	}
	if count(NoteFilter{}) != 2 || count(NoteFilter{Unassigned: true}) != 1 || count(NoteFilter{ProjectID: &p.ID}) != 1 {
		t.Fatal("filtros por proyecto")
	}
	// "idea" no debe coincidir con una etiqueta que solo la contiene.
	other := Note{Title: "Otra", Tags: []string{"ideas-viejas"}}
	if err := s.CreateNote(&other); err != nil {
		t.Fatal(err)
	}
	if count(NoteFilter{Tag: "idea"}) != 2 || count(NoteFilter{Tag: "#Reunion"}) != 1 || count(NoteFilter{Search: "acta"}) != 1 {
		t.Fatal("filtros por etiqueta y búsqueda")
	}
	tags, err := s.NoteTags()
	if err != nil || len(tags) != 4 || tags[0] != (TagCount{"idea", 2}) {
		t.Fatalf("NoteTags: %v %+v", err, tags)
	}

	// Quitar el proyecto y borrar el proyecto dejan la nota suelta, no la borran.
	linked.Tags = nil
	if err := s.UpdateNote(&linked); err != nil || len(linked.Tags) != 0 {
		t.Fatalf("UpdateNote: %v", err)
	}
	if err := s.DeleteProject(p.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := s.GetNote(linked.ID); err != nil || got.ProjectID != nil || got.ProjectName != "" {
		t.Fatalf("la nota debe sobrevivir sin proyecto: %v %+v", err, got)
	}
	if err := s.DeleteNote(loose.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetNote(loose.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetNote tras borrar: %v", err)
	}
	snap, err := s.Snapshot()
	if err != nil || len(snap.Notes) != 2 {
		t.Fatalf("Snapshot.Notes: %v %d", err, len(snap.Notes))
	}
}
