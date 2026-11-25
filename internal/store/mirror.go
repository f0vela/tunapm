package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// Snapshot es una copia completa de los datos; el API la sirve en /api/snapshot
// y la caché local la usa para reemplazar su espejo.
type Snapshot struct {
	Clients  []Client  `json:"clients"`
	Projects []Project `json:"projects"`
	Tasks    []Task    `json:"tasks"`
	Payments []Payment `json:"payments"`
	Notes    []Note    `json:"notes"`
	Events   []Event   `json:"events"`
}

// Snapshot lee todas las tablas (tareas hechas incluidas).
func (s *Store) Snapshot() (Snapshot, error) {
	snap := Snapshot{Clients: []Client{}, Projects: []Project{}, Tasks: []Task{}, Payments: []Payment{}}
	var err error
	if snap.Clients, err = s.ListClients(); err != nil {
		return snap, err
	}
	if snap.Projects, err = s.ListProjects(); err != nil {
		return snap, err
	}
	if snap.Tasks, err = s.ListTasks(TaskFilter{Mode: FilterAll}); err != nil {
		return snap, err
	}
	if snap.Payments, err = s.ListPayments(); err != nil {
		return snap, err
	}
	if snap.Notes, err = s.ListNotes(NoteFilter{}); err != nil {
		return snap, err
	}
	if snap.Notes == nil {
		snap.Notes = []Note{}
	}
	if snap.Events, err = s.ListEvents("", ""); err != nil {
		return snap, err
	}
	if snap.Events == nil {
		snap.Events = []Event{}
	}
	if snap.Clients == nil {
		snap.Clients = []Client{}
	}
	if snap.Projects == nil {
		snap.Projects = []Project{}
	}
	if snap.Tasks == nil {
		snap.Tasks = []Task{}
	}
	if snap.Payments == nil {
		snap.Payments = []Payment{}
	}
	return snap, nil
}

// DB expone la conexión para que la caché local guarde sus propias tablas (cola, meta).
func (s *Store) DB() *sql.DB { return s.db }

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func putClient(ex execer, c Client) error {
	_, err := ex.Exec(
		`INSERT INTO clients (id, name, company, email, phone, notes, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name = excluded.name, company = excluded.company, email = excluded.email,
		   phone = excluded.phone, notes = excluded.notes, created_at = excluded.created_at`,
		c.ID, c.Name, c.Company, c.Email, c.Phone, c.Notes, c.CreatedAt)
	return err
}

func putProject(ex execer, p Project) error {
	_, err := ex.Exec(
		`INSERT INTO projects (id, client_id, name, description, status, start_date, due_date, budget_cents,
		   currency, billing_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET client_id = excluded.client_id, name = excluded.name,
		   description = excluded.description, status = excluded.status, start_date = excluded.start_date,
		   due_date = excluded.due_date, budget_cents = excluded.budget_cents, currency = excluded.currency,
		   billing_type = excluded.billing_type, created_at = excluded.created_at, updated_at = excluded.updated_at`,
		p.ID, p.ClientID, p.Name, p.Description, p.Status, nullStr(deref(p.StartDate)), nullStr(deref(p.DueDate)),
		p.BudgetCents, p.Currency, p.BillingType, p.CreatedAt, p.UpdatedAt)
	return err
}

func putTask(ex execer, t Task) error {
	_, err := ex.Exec(
		`INSERT INTO tasks (id, project_id, title, notes, status, priority, due_date, created_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET project_id = excluded.project_id, title = excluded.title,
		   notes = excluded.notes, status = excluded.status, priority = excluded.priority,
		   due_date = excluded.due_date, created_at = excluded.created_at, completed_at = excluded.completed_at`,
		t.ID, t.ProjectID, t.Title, t.Notes, t.Status, t.Priority, nullStr(deref(t.DueDate)), t.CreatedAt, t.CompletedAt)
	return err
}

func putPayment(ex execer, p Payment) error {
	_, err := ex.Exec(
		`INSERT INTO payments (id, project_id, amount_cents, scheduled_date, paid_date, notes)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET project_id = excluded.project_id, amount_cents = excluded.amount_cents,
		   scheduled_date = excluded.scheduled_date, paid_date = excluded.paid_date, notes = excluded.notes`,
		p.ID, p.ProjectID, p.AmountCents, p.ScheduledDate, p.PaidDate, p.Notes)
	return err
}

func putNote(ex execer, n Note) error {
	_, err := ex.Exec(
		`INSERT INTO notes (id, project_id, title, body, tags, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET project_id = excluded.project_id, title = excluded.title, body = excluded.body,
		   tags = excluded.tags, created_at = excluded.created_at, updated_at = excluded.updated_at`,
		n.ID, n.ProjectID, n.Title, n.Body, strings.Join(NormalizeTags(n.Tags), ","), n.CreatedAt, n.UpdatedAt)
	return err
}

func putEvent(ex execer, e Event) error {
	_, err := ex.Exec(
		`INSERT INTO events (id, title, kind, date, time, project_id, client_id, notes, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET title = excluded.title, kind = excluded.kind, date = excluded.date,
		   time = excluded.time, project_id = excluded.project_id, client_id = excluded.client_id,
		   notes = excluded.notes, created_at = excluded.created_at`,
		e.ID, e.Title, e.Kind, e.Date, e.Time, e.ProjectID, e.ClientID, e.Notes, e.CreatedAt)
	return err
}

// ReplaceAll sustituye todo el contenido por snap (IDs incluidos) en una transacción.
func (s *Store) ReplaceAll(snap Snapshot) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("PRAGMA defer_foreign_keys = ON"); err != nil {
		return err
	}
	for _, table := range []string{"events", "notes", "payments", "tasks", "projects", "clients"} {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return err
		}
	}
	for _, c := range snap.Clients {
		if err := putClient(tx, c); err != nil {
			return err
		}
	}
	for _, p := range snap.Projects {
		if err := putProject(tx, p); err != nil {
			return err
		}
	}
	for _, t := range snap.Tasks {
		if err := putTask(tx, t); err != nil {
			return err
		}
	}
	for _, p := range snap.Payments {
		if err := putPayment(tx, p); err != nil {
			return err
		}
	}
	for _, n := range snap.Notes {
		if err := putNote(tx, n); err != nil {
			return err
		}
	}
	for _, e := range snap.Events {
		if err := putEvent(tx, e); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// RemapID cambia el id de una fila (y las referencias de sus hijos). La caché lo usa
// cuando el servidor asigna el id definitivo a algo creado sin conexión.
// kind: "client", "project", "task", "payment", "note" o "event".
func (s *Store) RemapID(kind string, oldID, newID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("PRAGMA defer_foreign_keys = ON"); err != nil {
		return err
	}
	var stmts [][2]string
	switch kind {
	case "client":
		stmts = [][2]string{{"clients", "id"}, {"projects", "client_id"}, {"events", "client_id"}}
	case "project":
		stmts = [][2]string{{"projects", "id"}, {"tasks", "project_id"}, {"payments", "project_id"}, {"notes", "project_id"}, {"events", "project_id"}}
	case "task":
		stmts = [][2]string{{"tasks", "id"}}
	case "payment":
		stmts = [][2]string{{"payments", "id"}}
	case "note":
		stmts = [][2]string{{"notes", "id"}}
	case "event":
		stmts = [][2]string{{"events", "id"}}
	default:
		return fmt.Errorf("tipo desconocido: %s", kind)
	}
	for _, st := range stmts {
		if _, err := tx.Exec(fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s = ?", st[0], st[1], st[1]), newID, oldID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ReserveIDRange hace que las filas creadas localmente reciban ids >= base, para
// distinguirlas de los ids reales del servidor hasta que se sincronicen.
func (s *Store) ReserveIDRange(base int64) error {
	for _, table := range []string{"clients", "projects", "tasks", "payments", "notes", "events"} {
		if _, err := s.db.Exec(
			`INSERT INTO sqlite_sequence (name, seq) SELECT ?, 0 WHERE NOT EXISTS (SELECT 1 FROM sqlite_sequence WHERE name = ?)`,
			table, table); err != nil {
			return err
		}
		if _, err := s.db.Exec("UPDATE sqlite_sequence SET seq = ? WHERE name = ? AND seq < ?", base, table, base); err != nil {
			return err
		}
	}
	return nil
}
