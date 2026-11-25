package agenda

import (
	"fmt"
	"strings"
	"testing"

	"tunapm/internal/store"
)

func sp(s string) *string { return &s }

func TestBuild(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	c := store.Client{Name: "Ana"}
	must(st.CreateClient(&c))
	p := store.Project{ClientID: c.ID, Name: "Web", Status: store.StatusActivo, Currency: "GTQ", BillingType: store.BillingParcial,
		BudgetCents: 900000, StartDate: sp("2026-11-02"), DueDate: sp("2026-11-20")}
	must(st.CreateProject(&p))
	cancelled := store.Project{ClientID: c.ID, Name: "Cancelado", Status: store.StatusCancelado, Currency: "GTQ",
		BillingType: store.BillingTotal, DueDate: sp("2026-11-20")}
	must(st.CreateProject(&cancelled))

	must(st.CreateTask(&store.Task{ProjectID: p.ID, Title: "Vencida", Status: store.TaskPendiente, DueDate: sp("2026-11-10")}))
	must(st.CreateTask(&store.Task{ProjectID: p.ID, Title: "Hecha", Status: store.TaskHecho, DueDate: sp("2026-11-10")}))
	must(st.CreateTask(&store.Task{ProjectID: p.ID, Title: "Sin fecha", Status: store.TaskPendiente}))
	must(st.CreateTask(&store.Task{ProjectID: p.ID, Title: "Fuera de rango", Status: store.TaskPendiente, DueDate: sp("2026-12-05")}))
	paid := store.Payment{ProjectID: p.ID, AmountCents: 100000, ScheduledDate: "2026-11-20"}
	must(st.CreatePayment(&paid))
	must(st.MarkPaymentPaid(paid.ID, "2026-11-20"))
	must(st.CreatePayment(&store.Payment{ProjectID: p.ID, AmountCents: 250000, ScheduledDate: "2026-11-20", Notes: "Anticipo"}))
	must(st.CreateEvent(&store.Event{Title: "Kickoff", Kind: store.EventReunion, Date: "2026-11-20", Time: sp("9:30"), ProjectID: &p.ID}))
	must(st.CreateEvent(&store.Event{Title: "Lanzamiento", Kind: store.EventLanzamiento, Date: "2026-11-20", ClientID: &c.ID}))
	must(st.CreateEvent(&store.Event{Title: "Tarde", Kind: "inventado", Date: "2026-11-20", Time: sp("15:00")}))

	items, err := Build(st, "2026-11-01", "2026-11-30", "2026-11-15")
	must(err)
	var got []string
	for _, it := range items {
		flags := ""
		if it.Done {
			flags += " hecho"
		}
		if it.Overdue {
			flags += " vencido"
		}
		got = append(got, fmt.Sprintf("%s %s %s: %s%s", it.Date[8:], it.Time, it.Label(), it.Text(), flags))
	}
	want := []string{
		"02  inicio: Web hecho",
		"10  tarea: Vencida vencido",
		"10  tarea: Hecha hecho",
		"20  lanzamiento: Lanzamiento",
		"20  entrega: Web",
		"20  cobro: Q 2,500.00 Anticipo",
		"20 09:30 reunión: Kickoff",
		"20 15:00 otro: Tarde",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("agenda:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if items[6].Detail != "Web · Ana" || items[3].Detail != "Ana" {
		t.Fatalf("detalle: %q / %q", items[6].Detail, items[3].Detail)
	}
}

func TestCalendarMath(t *testing.T) {
	if got := WeekStart("2026-11-25"); got != "2026-11-23" { // miércoles → lunes
		t.Fatalf("WeekStart = %s", got)
	}
	if got := WeekStart("2026-11-29"); got != "2026-11-23" { // domingo cierra la semana
		t.Fatalf("WeekStart domingo = %s", got)
	}
	grid := MonthGrid("2026-11-25")
	if len(grid) != 6 || grid[0][0] != "2026-10-26" || grid[5][6] != "2026-12-06" {
		t.Fatalf("MonthGrid = %d semanas, %s … %s", len(grid), grid[0][0], grid[len(grid)-1][6])
	}
	if got := AddMonths("2027-01-31", 1); got != "2027-02-28" {
		t.Fatalf("AddMonths = %s", got)
	}
	if got := AddMonths("2026-11-25", -11); got != "2025-12-25" {
		t.Fatalf("AddMonths atrás = %s", got)
	}
	if MonthTitle("2026-11-25") != "noviembre 2026" || DayTitle("2026-11-25") != "miércoles 25 de noviembre" {
		t.Fatalf("títulos: %q %q", MonthTitle("2026-11-25"), DayTitle("2026-11-25"))
	}
	if got := WeekTitle("2026-11-30"); got != "30 nov - 6 dic 2026" {
		t.Fatalf("WeekTitle = %q", got)
	}
}
