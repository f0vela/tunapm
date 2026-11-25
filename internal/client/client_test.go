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

func TestNotesRoundTrip(t *testing.T) {
	c := setup(t)

	cl := store.Client{Name: "Nora", Company: "Notas SA"}
	if err := c.CreateClient(&cl); err != nil {
		t.Fatal(err)
	}
	p := store.Project{ClientID: cl.ID, Name: "Proyecto con notas", Status: store.StatusActivo,
		Currency: "GTQ", BillingType: store.BillingTotal}
	if err := c.CreateProject(&p); err != nil {
		t.Fatal(err)
	}

	if err := c.CreateNote(&store.Note{Title: " "}); err == nil {
		t.Fatal("una nota sin título debe rechazarse")
	}
	loose := store.Note{Title: "Suelta zzq", Body: "cuerpo", Tags: []string{"#Contrato-ZZQ", "contrato-zzq", "Dos Palabras"}}
	if err := c.CreateNote(&loose); err != nil || loose.ID == 0 {
		t.Fatalf("CreateNote: %v %+v", err, loose)
	}
	if len(loose.Tags) != 2 || loose.Tags[0] != "contrato-zzq" || loose.Tags[1] != "dos-palabras" || loose.ProjectID != nil {
		t.Fatalf("nota creada: %+v", loose)
	}
	linked := store.Note{Title: "Del proyecto zzq", ProjectID: &p.ID, Tags: []string{"contrato-zzq"}}
	if err := c.CreateNote(&linked); err != nil {
		t.Fatal(err)
	}
	got, err := c.GetNote(linked.ID)
	if err != nil || got.ProjectID == nil || *got.ProjectID != p.ID || got.ProjectName != p.Name || got.ClientName != "Nora" {
		t.Fatalf("GetNote: %v %+v", err, got)
	}

	byTag, err := c.ListNotes(store.NoteFilter{Tag: "contrato-zzq"})
	if err != nil || len(byTag) != 2 {
		t.Fatalf("por etiqueta: %v %d", err, len(byTag))
	}
	byProject, _ := c.ListNotes(store.NoteFilter{ProjectID: &p.ID})
	if len(byProject) != 1 || byProject[0].ID != linked.ID {
		t.Fatalf("por proyecto: %+v", byProject)
	}
	unassigned, _ := c.ListNotes(store.NoteFilter{Unassigned: true, Search: "zzq"})
	if len(unassigned) != 1 || unassigned[0].ID != loose.ID {
		t.Fatalf("sin proyecto: %+v", unassigned)
	}
	tags, err := c.NoteTags()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tc := range tags {
		found = found || (tc.Tag == "contrato-zzq" && tc.Count == 2)
	}
	if !found {
		t.Fatalf("NoteTags: %+v", tags)
	}

	// Quitar proyecto y etiquetas con una actualización.
	linked.ProjectID, linked.Tags, linked.Body = nil, []string{}, "editada"
	if err := c.UpdateNote(&linked); err != nil || linked.ProjectID != nil || len(linked.Tags) != 0 || linked.Body != "editada" {
		t.Fatalf("UpdateNote: %v %+v", err, linked)
	}

	// Borrar el proyecto no borra sus notas.
	again := store.Note{Title: "Sobrevive zzq", ProjectID: &p.ID}
	if err := c.CreateNote(&again); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteProject(p.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := c.GetNote(again.ID); err != nil || got.ProjectID != nil {
		t.Fatalf("nota tras borrar el proyecto: %v %+v", err, got)
	}
	snap, err := c.Snapshot()
	if err != nil || len(snap.Notes) < 3 {
		t.Fatalf("Snapshot.Notes: %v %d", err, len(snap.Notes))
	}

	for _, n := range []int64{loose.ID, linked.ID, again.ID} {
		if err := c.DeleteNote(n); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.GetNote(loose.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetNote tras borrar: %v", err)
	}
	_ = c.DeleteClient(cl.ID)
}

func TestEventsAndAgenda(t *testing.T) {
	c := setup(t)
	sp := func(s string) *string { return &s }

	cl := store.Client{Name: "Eva Agenda"}
	if err := c.CreateClient(&cl); err != nil {
		t.Fatal(err)
	}
	// Fechas lejanas para no chocar con datos de otros tests en un servidor compartido.
	p := store.Project{ClientID: cl.ID, Name: "Proyecto agenda", Status: store.StatusActivo, Currency: "GTQ",
		BillingType: store.BillingParcial, BudgetCents: 500000, StartDate: sp("2031-03-02"), DueDate: sp("2031-03-20")}
	if err := c.CreateProject(&p); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []store.Event{
		{Title: " ", Date: "2031-03-20"},
		{Title: "x", Date: "20/03/2031"},
		{Title: "x", Date: "2031-03-20", Time: sp("25:00")},
	} {
		if err := c.CreateEvent(&bad); err == nil {
			t.Fatalf("debió rechazarse: %+v", bad)
		}
	}
	timed := store.Event{Title: "Kickoff", Kind: store.EventReunion, Date: "2031-03-20", Time: sp("9:30"), ProjectID: &p.ID, Notes: "sala 2"}
	if err := c.CreateEvent(&timed); err != nil || timed.ID == 0 {
		t.Fatalf("CreateEvent: %v", err)
	}
	// Los nombres de proyecto y cliente se leen con GET (al crear, el API en Go devuelve lo enviado).
	if got, err := c.GetEvent(timed.ID); err != nil || got.Time == nil || *got.Time != "09:30" || got.ProjectName != p.Name || got.ClientName != "Eva Agenda" {
		t.Fatalf("evento creado: %v %+v", err, got)
	}
	allDay := store.Event{Title: "Lanzamiento", Kind: "inventado", Date: "2031-03-20", ClientID: &cl.ID}
	if err := c.CreateEvent(&allDay); err != nil || allDay.Kind != store.EventOtro || allDay.Time != nil {
		t.Fatalf("evento de todo el día: %v %+v", err, allDay)
	}
	list, err := c.ListEvents("2031-03-01", "2031-03-31")
	if err != nil || len(list) != 2 || list[0].ID != allDay.ID || list[1].ID != timed.ID {
		t.Fatalf("ListEvents: %v %+v", err, list)
	}

	task := store.Task{ProjectID: p.ID, Title: "Tarea agenda", Status: store.TaskPendiente, Priority: 1, DueDate: sp("2031-03-10")}
	if err := c.CreateTask(&task); err != nil {
		t.Fatal(err)
	}
	paid := store.Payment{ProjectID: p.ID, AmountCents: 100000, ScheduledDate: "2031-03-20"}
	if err := c.CreatePayment(&paid); err != nil {
		t.Fatal(err)
	}
	if err := c.MarkPaymentPaid(paid.ID, "2031-03-20"); err != nil {
		t.Fatal(err)
	}
	if err := c.CreatePayment(&store.Payment{ProjectID: p.ID, AmountCents: 250000, ScheduledDate: "2031-03-20", Notes: "Anticipo"}); err != nil {
		t.Fatal(err)
	}

	items, err := c.Agenda("2031-03-01", "2031-03-31")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, it := range items {
		got = append(got, it.Date[8:]+" "+it.Time+" "+it.Label()+": "+it.Text())
	}
	want := []string{
		"02  inicio: Proyecto agenda",
		"10  tarea: Tarea agenda",
		"20  otro: Lanzamiento",
		"20  entrega: Proyecto agenda",
		"20  cobro: Q 2,500.00 Anticipo",
		"20 09:30 reunión: Kickoff",
	}
	if len(got) != len(want) {
		t.Fatalf("agenda = %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("agenda[%d] = %q, want %q (todo: %q)", i, got[i], want[i], got)
		}
	}
	if items[5].Detail != "Proyecto agenda · Eva Agenda" || items[5].ProjectID != p.ID || items[4].AmountCents != 250000 {
		t.Fatalf("detalle de la agenda: %+v / %+v", items[5], items[4])
	}
	if _, err := c.Agenda("", "2031-03-31"); err == nil {
		t.Fatal("la agenda sin rango debe rechazarse")
	}

	// Quitar hora y proyecto con una actualización.
	timed.Time, timed.ProjectID, timed.Title = nil, nil, "Kickoff movido"
	if err := c.UpdateEvent(&timed); err != nil || timed.Time != nil || timed.ProjectID != nil {
		t.Fatalf("UpdateEvent: %v %+v", err, timed)
	}
	// Borrar el proyecto y el cliente conserva los eventos.
	if err := c.DeleteProject(p.ID); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteClient(cl.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := c.GetEvent(allDay.ID); err != nil || got.ClientID != nil {
		t.Fatalf("evento tras borrar el cliente: %v %+v", err, got)
	}
	if snap, err := c.Snapshot(); err != nil || len(snap.Events) < 2 {
		t.Fatalf("Snapshot.Events: %v", err)
	}
	for _, n := range []int64{timed.ID, allDay.ID} {
		if err := c.DeleteEvent(n); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.GetEvent(timed.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetEvent tras borrar: %v", err)
	}
}
