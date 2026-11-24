package web_test

import (
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
		"/tasks/new", "/projects/new", "/payments/new", "/clients/new", "/static/app.css"} {
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
