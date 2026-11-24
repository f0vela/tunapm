package main

import (
	"time"

	"tunapm/internal/dates"
	"tunapm/internal/store"
)

// seed llena una base vacía con datos de muestra para probar la TUI.
// Devuelve false si ya había clientes y no tocó nada.
func seed(st *store.Store) (bool, error) {
	if clients, err := st.ListClients(); err != nil {
		return false, err
	} else if len(clients) > 0 {
		return false, nil
	}

	acme := store.Client{Name: "Ana Pérez", Company: "Acme", Email: "ana@acme.example", Phone: "5555-0101",
		Notes: "Prefiere comunicación por correo."}
	lumen := store.Client{Name: "Luis Méndez", Company: "Estudio Lumen", Email: "luis@lumen.example"}
	for _, c := range []*store.Client{&acme, &lumen} {
		if err := st.CreateClient(c); err != nil {
			return false, err
		}
	}

	rel := func(raw string) *string {
		date, err := dates.Parse(raw, time.Now())
		if err != nil {
			return nil
		}
		return &date
	}

	site := store.Project{
		ClientID: acme.ID, Name: "Sitio web Acme", Description: "Rediseño y migración del sitio corporativo.",
		Status: store.StatusActivo, StartDate: rel("-12d"), DueDate: rel("+18d"),
		BudgetCents: 2500000, Currency: store.CurrencyGTQ, BillingType: store.BillingParcial,
	}
	identity := store.Project{
		ClientID: lumen.ID, Name: "Identidad Lumen", Description: "Logotipo, paleta y guía de marca.",
		Status: store.StatusActivo, StartDate: rel("-5d"), DueDate: rel("+25d"),
		BudgetCents: 180000, Currency: store.CurrencyUSD, BillingType: store.BillingTotal,
	}
	prospect := store.Project{
		ClientID: lumen.ID, Name: "App de pedidos", Status: store.StatusProspecto,
		BudgetCents: 950000, Currency: store.CurrencyGTQ, BillingType: store.BillingTotal,
	}
	for _, p := range []*store.Project{&site, &identity, &prospect} {
		if err := st.CreateProject(p); err != nil {
			return false, err
		}
	}

	tasks := []store.Task{
		{ProjectID: site.ID, Title: "Wireframes del home", Status: store.TaskHecho, Priority: 2, DueDate: rel("-4d")},
		{ProjectID: site.ID, Title: "Diseñar logo y encabezado", Status: store.TaskEnCurso, Priority: 3, DueDate: rel("hoy"),
			Notes: "Revisar con Ana antes del viernes."},
		{ProjectID: site.ID, Title: "Migrar contenido del blog", Status: store.TaskPendiente, Priority: 1, DueDate: rel("+6d")},
		{ProjectID: site.ID, Title: "Pruebas en móvil", Status: store.TaskPendiente, Priority: 2, DueDate: rel("+12d")},
		{ProjectID: identity.ID, Title: "Propuesta de logotipo", Status: store.TaskEnCurso, Priority: 2, DueDate: rel("+3d")},
		{ProjectID: identity.ID, Title: "Guía de marca", Status: store.TaskPendiente, Priority: 0, DueDate: rel("+15d")},
		{ProjectID: prospect.ID, Title: "Cotización de la app", Status: store.TaskPendiente, Priority: 1, DueDate: rel("mañana")},
	}
	for i := range tasks {
		if err := st.CreateTask(&tasks[i]); err != nil {
			return false, err
		}
	}

	payments := []store.Payment{
		{ProjectID: site.ID, AmountCents: 1250000, ScheduledDate: *rel("-10d"), PaidDate: rel("-9d"), Notes: "Anticipo 50%"},
		{ProjectID: site.ID, AmountCents: 1250000, ScheduledDate: *rel("+18d"), Notes: "Contra entrega"},
		{ProjectID: identity.ID, AmountCents: 180000, ScheduledDate: *rel("-2d"), Notes: "Pago único"},
	}
	for i := range payments {
		if err := st.CreatePayment(&payments[i]); err != nil {
			return false, err
		}
	}
	return true, nil
}
