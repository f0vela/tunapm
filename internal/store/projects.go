package store

import (
	"database/sql"
	"errors"
)

func (s *Store) CreateProject(p *Project) error {
	p.CreatedAt, p.UpdatedAt = now(), now()
	res, err := s.db.Exec(
		`INSERT INTO projects
		 (client_id, name, description, status, start_date, due_date, budget_cents, currency, billing_type, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ClientID, p.Name, p.Description, p.Status, nullStr(deref(p.StartDate)), nullStr(deref(p.DueDate)),
		p.BudgetCents, p.Currency, p.BillingType, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return err
	}
	p.ID, err = res.LastInsertId()
	return err
}

func (s *Store) UpdateProject(p *Project) error {
	if p.BillingType == BillingTotal {
		var n int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM payments WHERE project_id = ?", p.ID).Scan(&n); err != nil {
			return err
		}
		if n > 1 {
			return ErrTotalBillingOneQuote
		}
	}
	p.UpdatedAt = now()
	res, err := s.db.Exec(
		`UPDATE projects SET client_id = ?, name = ?, description = ?, status = ?, start_date = ?,
		 due_date = ?, budget_cents = ?, currency = ?, billing_type = ?, updated_at = ?
		 WHERE id = ?`,
		p.ClientID, p.Name, p.Description, p.Status, nullStr(deref(p.StartDate)), nullStr(deref(p.DueDate)),
		p.BudgetCents, p.Currency, p.BillingType, p.UpdatedAt, p.ID,
	)
	return affectedOne(res, err)
}

func (s *Store) DeleteProject(id int64) error {
	res, err := s.db.Exec("DELETE FROM projects WHERE id = ?", id)
	return affectedOne(res, err)
}

func (s *Store) GetProject(id int64) (Project, error) {
	p, err := scanProject(s.db.QueryRow(projectQuery+" WHERE p.id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return p, err
}

func (s *Store) ListProjects() ([]Project, error) {
	return s.queryProjects(projectQuery + projectOrder)
}

func (s *Store) ProjectsByClient(clientID int64) ([]Project, error) {
	return s.queryProjects(projectQuery+" WHERE p.client_id = ?"+projectOrder, clientID)
}

func (s *Store) CountProjectsByClient(clientID int64) (int, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM projects WHERE client_id = ?", clientID).Scan(&n)
	return n, err
}

const projectQuery = `
	SELECT p.id, p.client_id, p.name, p.description, p.status, p.start_date, p.due_date,
	       p.budget_cents, p.currency, p.billing_type, p.created_at, p.updated_at,
	       COALESCE(c.name, '')
	FROM projects p
	JOIN clients c ON c.id = p.client_id`

const projectOrder = `
	ORDER BY CASE p.status
	           WHEN 'activo' THEN 0 WHEN 'prospecto' THEN 1 WHEN 'pausado' THEN 2
	           WHEN 'completado' THEN 3 ELSE 4 END,
	         p.name COLLATE NOCASE`

func (s *Store) queryProjects(query string, args ...any) ([]Project, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanProject(row scanner) (Project, error) {
	var p Project
	var start, due sql.NullString
	err := row.Scan(&p.ID, &p.ClientID, &p.Name, &p.Description, &p.Status, &start, &due,
		&p.BudgetCents, &p.Currency, &p.BillingType, &p.CreatedAt, &p.UpdatedAt, &p.ClientName)
	p.StartDate, p.DueDate = strPtr(start), strPtr(due)
	return p, err
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
