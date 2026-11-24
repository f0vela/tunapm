package main

import (
	"database/sql"
	"fmt"
	"math"
	"net/url"
	"strings"

	_ "modernc.org/sqlite"

	"tunapm/internal/store"
)

// importOld copia clientes, proyectos, tareas y cuotas desde la base antigua
// (esquema Laravel) a la actual. Se niega a correr si ya hay clientes.
func importOld(st *store.Store, oldPath string) error {
	if cs, err := st.ListClients(); err != nil {
		return err
	} else if len(cs) > 0 {
		return fmt.Errorf("la base destino ya tiene datos; use otra con -db para importar")
	}

	old, err := sql.Open("sqlite", "file:"+url.PathEscape(oldPath)+"?mode=ro")
	if err != nil {
		return err
	}
	defer old.Close()

	day := func(ns sql.NullString) *string {
		if !ns.Valid || len(ns.String) < 10 {
			return nil
		}
		d := ns.String[:10]
		return &d
	}

	// Clientes
	clientMap := map[int64]int64{}
	rows, err := old.Query(`SELECT id, name, COALESCE(company,''), COALESCE(email,''), COALESCE(phone,''), COALESCE(notes,'') FROM clients ORDER BY id`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var oid int64
		var c store.Client
		if err := rows.Scan(&oid, &c.Name, &c.Company, &c.Email, &c.Phone, &c.Notes); err != nil {
			return err
		}
		if err := st.CreateClient(&c); err != nil {
			return err
		}
		clientMap[oid] = c.ID
	}
	rows.Close()

	// Proyectos
	type pinfo struct {
		id     int64
		budget int64 // centavos
		cur    string
	}
	projMap := map[int64]pinfo{}
	rows, err = old.Query(`
		SELECT p.id, p.client_id, p.name, COALESCE(p.description,''), p.budget, p.billing_mode, p.status, p.currency,
		       COALESCE((SELECT MIN(planned_start) FROM project_phases WHERE project_id = p.id),
		                p.design_start, p.development_start),
		       p.delivery_date
		FROM projects p ORDER BY p.id`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var oid, ocl int64
		var budget float64
		var mode, status, cur string
		var start, due sql.NullString
		p := store.Project{}
		if err := rows.Scan(&oid, &ocl, &p.Name, &p.Description, &budget, &mode, &status, &cur, &start, &due); err != nil {
			return err
		}
		p.ClientID = clientMap[ocl]
		p.StartDate, p.DueDate = day(start), day(due)
		p.BudgetCents = int64(math.Round(budget * 100))
		p.Currency = strings.ToUpper(cur)
		switch status {
		case "planned":
			p.Status = store.StatusProspecto
		case "design", "development", "testing":
			p.Status = store.StatusActivo
		default: // delivered, finished
			p.Status = store.StatusCompletado
		}
		if mode == "single" {
			p.BillingType = store.BillingTotal
		} else {
			p.BillingType = store.BillingParcial
		}
		if err := st.CreateProject(&p); err != nil {
			return fmt.Errorf("proyecto %q: %w", p.Name, err)
		}
		projMap[oid] = pinfo{p.ID, p.BudgetCents, p.Currency}
	}
	rows.Close()

	// Tareas
	rows, err = old.Query(`SELECT project_id, title, COALESCE(description,''), priority, status, estimated_end_at FROM tasks ORDER BY id`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var opid int64
		var prio, status string
		var due sql.NullString
		t := store.Task{}
		if err := rows.Scan(&opid, &t.Title, &t.Notes, &prio, &status, &due); err != nil {
			return err
		}
		t.ProjectID = projMap[opid].id
		t.DueDate = day(due)
		t.Priority = map[string]int{"low": 0, "medium": 1, "high": 2, "urgent": 3}[prio]
		if status == "planned" {
			t.Status = store.TaskPendiente
		} else {
			t.Status = store.TaskHecho
		}
		if err := st.CreateTask(&t); err != nil {
			return fmt.Errorf("tarea %q: %w", t.Title, err)
		}
	}
	rows.Close()

	// Cuotas
	rows, err = old.Query(`SELECT project_id, label, calculation_type, value, due_date, paid_at, status FROM payments ORDER BY project_id, id`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var opid int64
		var label, calc, status string
		var value float64
		var due, paid sql.NullString
		if err := rows.Scan(&opid, &label, &calc, &value, &due, &paid, &status); err != nil {
			return err
		}
		pi := projMap[opid]
		pay := store.Payment{ProjectID: pi.id, Notes: label}
		if calc == "percentage" {
			pay.AmountCents = int64(math.Round(float64(pi.budget) * value / 100))
		} else {
			pay.AmountCents = int64(math.Round(value * 100))
		}
		if d := day(due); d != nil {
			pay.ScheduledDate = *d
		} else if d := day(paid); d != nil {
			pay.ScheduledDate = *d
		}
		// El estado del origen manda: algunas cuotas "pending" traen paid_at.
		if status == "paid" {
			pay.PaidDate = day(paid)
			if pay.PaidDate == nil {
				pay.PaidDate = &pay.ScheduledDate
			}
		} else if d := day(paid); d != nil {
			pay.Notes += " (origen: pendiente, fecha de pago registrada " + *d + ")"
		}
		if err := st.CreatePayment(&pay); err != nil {
			return fmt.Errorf("cuota %q: %w", label, err)
		}
	}
	rows.Close()
	return nil
}
