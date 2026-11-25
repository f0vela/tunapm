package web_test

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"tunapm/internal/store"
	"tunapm/internal/web"
)

func TestWebFlow(t *testing.T) {
	st, _ := store.Open(":memory:")
	defer st.Close()
	c := store.Client{Name: "Ana"}
	_ = st.CreateClient(&c)
	p := store.Project{ClientID: c.ID, Name: "Web", Status: "activo", Currency: "GTQ", BillingType: "parcial"}
	_ = st.CreateProject(&p)

	h, err := web.New(st, web.Options{Password: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	cli := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	if r, _ := cli.Get(srv.URL + "/"); r.StatusCode != 303 {
		t.Fatalf("sin login debe redirigir, got %d", r.StatusCode)
	}
	if r, _ := cli.PostForm(srv.URL+"/login", url.Values{"password": {"mal"}}); r.StatusCode != 401 {
		t.Fatalf("login malo = %d", r.StatusCode)
	}
	if r, _ := cli.PostForm(srv.URL+"/login", url.Values{"password": {"pw"}}); r.StatusCode != 303 {
		t.Fatalf("login = %d", r.StatusCode)
	}

	for _, path := range []string{"/", "/tasks", "/projects", "/projects/1", "/clients", "/clients/1", "/payments",
		"/tasks/new", "/projects/new", "/payments/new", "/clients/new", "/static/app.css",
		"/notes", "/notes/new", "/notes?project_id=none&tag=idea&q=x",
		"/calendar", "/calendar?view=semana&date=2026-12-01", "/calendar?view=dia&date=2026-12-01",
		"/calendar?view=mes&date=2027-02-28", "/calendar?date=basura", "/events/new?date=2026-12-01&project_id=1"} {
		r, _ := cli.Get(srv.URL + path)
		if r.StatusCode != 200 {
			t.Errorf("GET %s = %d", path, r.StatusCode)
		}
	}
	if r, _ := cli.Get(srv.URL + "/projects/99"); r.StatusCode != 404 {
		t.Errorf("proyecto inexistente = %d", r.StatusCode)
	}

	r, _ := cli.PostForm(srv.URL+"/tasks", url.Values{"title": {"Hacer cosa"}, "project_id": {"1"}, "status": {"pendiente"}, "priority": {"2"}, "next": {"/tasks"}})
	if r.StatusCode != 303 {
		t.Fatalf("crear tarea = %d", r.StatusCode)
	}
	r, _ = cli.PostForm(srv.URL+"/payments", url.Values{"project_id": {"1"}, "amount": {"1,500.50"}, "scheduled_date": {"2026-12-01"}})
	if r.StatusCode != 303 {
		t.Fatalf("crear cuota = %d", r.StatusCode)
	}
	if r, _ = cli.PostForm(srv.URL+"/payments/1/pay", nil); r.StatusCode != 303 {
		t.Fatalf("pagar = %d", r.StatusCode)
	}
	pay, _ := st.GetPayment(1)
	if pay.AmountCents != 150050 || pay.PaidDate == nil {
		t.Fatalf("cuota = %+v", pay)
	}
	if r, _ = cli.PostForm(srv.URL+"/payments/1/unpay", nil); r.StatusCode != 303 {
		t.Fatalf("unpay = %d", r.StatusCode)
	}
	if pay, _ = st.GetPayment(1); pay.PaidDate != nil || pay.AmountCents != 150050 {
		t.Fatalf("cuota tras deshacer = %+v", pay)
	}
	if r, _ = cli.PostForm(srv.URL+"/tasks/1/toggle", nil); r.StatusCode != 303 {
		t.Fatalf("toggle = %d", r.StatusCode)
	}
	if tk, _ := st.GetTask(1); tk.Status != store.TaskHecho {
		t.Fatalf("tarea = %+v", tk)
	}
	// Notas: con proyecto, sin proyecto, etiquetas normalizadas, edición y borrado.
	r, _ = cli.PostForm(srv.URL+"/notes", url.Values{"title": {"Acta"}, "project_id": {"1"}, "tags": {"Reunión, #Idea idea"}, "body": {"línea 1\r\nlínea 2"}})
	if r.StatusCode != 303 {
		t.Fatalf("crear nota = %d", r.StatusCode)
	}
	note, _ := st.GetNote(1)
	if note.ProjectID == nil || strings.Join(note.Tags, ",") != "reunión,idea" || note.Body != "línea 1\nlínea 2" {
		t.Fatalf("nota = %+v", note)
	}
	if r, _ = cli.PostForm(srv.URL+"/notes/1", url.Values{"title": {"Acta 2"}, "project_id": {""}, "tags": {""}}); r.StatusCode != 303 {
		t.Fatalf("editar nota = %d", r.StatusCode)
	}
	if note, _ = st.GetNote(1); note.ProjectID != nil || len(note.Tags) != 0 || note.Title != "Acta 2" {
		t.Fatalf("nota editada = %+v", note)
	}
	for _, path := range []string{"/notes", "/notes/1/edit", "/projects/1"} {
		if r, _ := cli.Get(srv.URL + path); r.StatusCode != 200 {
			t.Errorf("GET %s = %d", path, r.StatusCode)
		}
	}
	if r, _ = cli.PostForm(srv.URL+"/notes", url.Values{"title": {" "}}); r.StatusCode != 422 {
		t.Fatalf("nota sin título = %d", r.StatusCode)
	}
	if r, _ = cli.PostForm(srv.URL+"/notes/1/delete", nil); r.StatusCode != 303 {
		t.Fatalf("borrar nota = %d", r.StatusCode)
	}
	// Calendario: evento con hora y proyecto; aparece en el día junto a la cuota por cobrar.
	r, _ = cli.PostForm(srv.URL+"/events", url.Values{"title": {"Reunión zzc"}, "kind": {"reunion"}, "date": {"2026-12-01"}, "time": {"9:30"}, "project_id": {"1"}})
	if r.StatusCode != 303 || !strings.Contains(r.Header.Get("Location"), "date=2026-12-01") {
		t.Fatalf("crear evento = %d %s", r.StatusCode, r.Header.Get("Location"))
	}
	ev, _ := st.GetEvent(1)
	if ev.Time == nil || *ev.Time != "09:30" || ev.ProjectID == nil {
		t.Fatalf("evento = %+v", ev)
	}
	for _, view := range []string{"dia", "semana", "mes"} {
		res, _ := cli.Get(srv.URL + "/calendar?view=" + view + "&date=2026-12-01")
		page, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if !strings.Contains(string(page), "Reunión zzc") || !strings.Contains(string(page), "1,500.50") {
			t.Errorf("la vista %s debe mostrar el evento y la cuota por cobrar", view)
		}
	}
	if r, _ = cli.PostForm(srv.URL+"/events", url.Values{"title": {"x"}, "date": {"2026-12-01"}, "time": {"99:99"}}); r.StatusCode != 422 {
		t.Fatalf("hora inválida = %d", r.StatusCode)
	}
	if r, _ = cli.PostForm(srv.URL+"/events/1", url.Values{"title": {"Movida"}, "kind": {"otro"}, "date": {"2026-12-02"}, "time": {""}, "project_id": {""}}); r.StatusCode != 303 {
		t.Fatalf("editar evento = %d", r.StatusCode)
	}
	if ev, _ = st.GetEvent(1); ev.Time != nil || ev.ProjectID != nil || ev.Date != "2026-12-02" {
		t.Fatalf("evento editado = %+v", ev)
	}
	for _, path := range []string{"/events/1/edit", "/projects/1"} {
		if r, _ := cli.Get(srv.URL + path); r.StatusCode != 200 {
			t.Errorf("GET %s = %d", path, r.StatusCode)
		}
	}
	if r, _ = cli.PostForm(srv.URL+"/events/1/delete", nil); r.StatusCode != 303 {
		t.Fatalf("borrar evento = %d", r.StatusCode)
	}
	// Error de validación se muestra en el formulario.
	r, _ = cli.PostForm(srv.URL+"/payments", url.Values{"project_id": {"1"}, "amount": {"abc"}, "scheduled_date": {"2026-12-01"}})
	if r.StatusCode != 422 {
		t.Fatalf("monto inválido = %d", r.StatusCode)
	}
	// Redirección "next" externa se ignora.
	r, _ = cli.PostForm(srv.URL+"/tasks/1/toggle", url.Values{"next": {"//evil.com"}})
	if loc := r.Header.Get("Location"); strings.Contains(loc, "evil") {
		t.Fatalf("open redirect: %s", loc)
	}
}
