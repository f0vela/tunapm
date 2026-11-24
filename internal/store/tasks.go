package store

import (
	"database/sql"
	"errors"
	"strings"
)

const (
	FilterOpen = "abiertas"
	FilterAll  = "todas"
	FilterDone = "hechas"
)

type TaskFilter struct {
	Mode      string // FilterOpen (default), FilterAll o FilterDone
	ProjectID *int64
	ClientID  *int64
	Search    string
}

func (s *Store) CreateTask(t *Task) error {
	t.CreatedAt = now()
	if t.Status == TaskHecho {
		completed := now()
		t.CompletedAt = &completed
	}
	res, err := s.db.Exec(
		`INSERT INTO tasks (project_id, title, notes, status, priority, due_date, created_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ProjectID, t.Title, t.Notes, t.Status, t.Priority, nullStr(deref(t.DueDate)), t.CreatedAt, t.CompletedAt,
	)
	if err != nil {
		return err
	}
	t.ID, err = res.LastInsertId()
	return err
}

func (s *Store) UpdateTask(t *Task) error {
	// completed_at se mantiene coherente con el estado sin depender de la UI.
	if t.Status == TaskHecho && t.CompletedAt == nil {
		completed := now()
		t.CompletedAt = &completed
	} else if t.Status != TaskHecho {
		t.CompletedAt = nil
	}
	res, err := s.db.Exec(
		`UPDATE tasks SET project_id = ?, title = ?, notes = ?, status = ?, priority = ?,
		 due_date = ?, completed_at = ? WHERE id = ?`,
		t.ProjectID, t.Title, t.Notes, t.Status, t.Priority, nullStr(deref(t.DueDate)), t.CompletedAt, t.ID,
	)
	return affectedOne(res, err)
}

func (s *Store) DeleteTask(id int64) error {
	res, err := s.db.Exec("DELETE FROM tasks WHERE id = ?", id)
	return affectedOne(res, err)
}

func (s *Store) GetTask(id int64) (Task, error) {
	t, err := scanTask(s.db.QueryRow(taskQuery+" WHERE t.id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return t, err
}

func (s *Store) ListTasks(f TaskFilter) ([]Task, error) {
	var where []string
	var args []any

	switch f.Mode {
	case FilterOpen, "":
		where = append(where, "t.status <> 'hecho'")
	case FilterDone:
		where = append(where, "t.status = 'hecho'")
	case FilterAll:
	}
	if f.ProjectID != nil {
		where = append(where, "t.project_id = ?")
		args = append(args, *f.ProjectID)
	}
	if f.ClientID != nil {
		where = append(where, "p.client_id = ?")
		args = append(args, *f.ClientID)
	}
	if f.Search != "" {
		where = append(where, "(t.title LIKE ? OR t.notes LIKE ? OR p.name LIKE ? OR c.name LIKE ?)")
		needle := "%" + strings.TrimSpace(f.Search) + "%"
		args = append(args, needle, needle, needle, needle)
	}

	query := taskQuery
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	// Abiertas primero, luego prioridad y fecha límite (sin fecha al final).
	query += `
		ORDER BY CASE WHEN t.status = 'hecho' THEN 1 ELSE 0 END,
		         t.priority DESC,
		         t.due_date IS NULL, t.due_date ASC,
		         t.id`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TaskProgress cuenta tareas hechas y totales por proyecto.
func (s *Store) TaskProgress() (map[int64]TaskProgress, error) {
	rows, err := s.db.Query(
		`SELECT project_id, COUNT(*), SUM(CASE WHEN status = 'hecho' THEN 1 ELSE 0 END)
		 FROM tasks GROUP BY project_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64]TaskProgress)
	for rows.Next() {
		var id int64
		var p TaskProgress
		if err := rows.Scan(&id, &p.Total, &p.Done); err != nil {
			return nil, err
		}
		out[id] = p
	}
	return out, rows.Err()
}

type TaskProgress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

const taskQuery = `	SELECT t.id, t.project_id, t.title, t.notes, t.status, t.priority, t.due_date,
	       t.created_at, t.completed_at, p.name, COALESCE(c.name, '')
	FROM tasks t
	JOIN projects p ON p.id = t.project_id
	JOIN clients c ON c.id = p.client_id`

func scanTask(row scanner) (Task, error) {
	var t Task
	var due, completed sql.NullString
	err := row.Scan(&t.ID, &t.ProjectID, &t.Title, &t.Notes, &t.Status, &t.Priority, &due,
		&t.CreatedAt, &completed, &t.ProjectName, &t.ClientName)
	t.DueDate, t.CompletedAt = strPtr(due), strPtr(completed)
	return t, err
}
