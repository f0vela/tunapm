package store

import (
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"
)

const (
	EventReunion      = "reunion"
	EventLlamada      = "llamada"
	EventPresentacion = "presentacion"
	EventLanzamiento  = "lanzamiento"
	EventOtro         = "otro"
)

// EventKinds alimenta los selectores de la UI, en el orden en que se muestran.
var EventKinds = []string{EventReunion, EventLlamada, EventPresentacion, EventLanzamiento, EventOtro}

var EventKindLabels = map[string]string{
	EventReunion:      "reunión",
	EventLlamada:      "llamada",
	EventPresentacion: "presentación",
	EventLanzamiento:  "lanzamiento",
	EventOtro:         "otro",
}

var (
	ErrEventTitle = errors.New("el evento necesita un título")
	ErrEventDate  = errors.New("fecha del evento inválida (usa YYYY-MM-DD)")
	ErrEventTime  = errors.New("hora del evento inválida (usa HH:MM, 24 horas)")
)

// Event es un evento de calendario. Time nil = todo el día.
type Event struct {
	ID        int64   `json:"id"`
	Title     string  `json:"title"`
	Kind      string  `json:"kind"`
	Date      string  `json:"date"` // YYYY-MM-DD
	Time      *string `json:"time"` // HH:MM
	ProjectID *int64  `json:"project_id"`
	ClientID  *int64  `json:"client_id"`
	Notes     string  `json:"notes"`
	CreatedAt string  `json:"created_at"`

	// Solo lectura. ClientName es el cliente del evento o, si no tiene, el del proyecto.
	ProjectName string `json:"project_name"`
	ClientName  string `json:"client_name"`
}

var eventTimeRE = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// NormalizeTime acepta "9:30", "09:30" o "0930" y devuelve "09:30"; "" si no hay hora.
func NormalizeTime(raw string) (string, error) {
	t := strings.TrimSpace(raw)
	if t == "" {
		return "", nil
	}
	if !strings.Contains(t, ":") && len(t) >= 3 && len(t) <= 4 {
		t = t[:len(t)-2] + ":" + t[len(t)-2:]
	}
	if len(t) == 4 {
		t = "0" + t
	}
	if !eventTimeRE.MatchString(t) {
		return "", ErrEventTime
	}
	return t, nil
}

func (e *Event) prepare() error {
	e.Title = strings.TrimSpace(e.Title)
	if e.Title == "" {
		return ErrEventTitle
	}
	if _, err := time.Parse("2006-01-02", e.Date); err != nil {
		return ErrEventDate
	}
	if _, ok := EventKindLabels[e.Kind]; !ok {
		e.Kind = EventOtro
	}
	if e.Time != nil {
		t, err := NormalizeTime(*e.Time)
		if err != nil {
			return err
		}
		e.Time = nil
		if t != "" {
			e.Time = &t
		}
	}
	if e.ProjectID != nil && *e.ProjectID == 0 {
		e.ProjectID = nil
	}
	if e.ClientID != nil && *e.ClientID == 0 {
		e.ClientID = nil
	}
	return nil
}

func (s *Store) CreateEvent(e *Event) error {
	if err := e.prepare(); err != nil {
		return err
	}
	e.CreatedAt = now()
	res, err := s.db.Exec(
		`INSERT INTO events (title, kind, date, time, project_id, client_id, notes, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Title, e.Kind, e.Date, e.Time, e.ProjectID, e.ClientID, e.Notes, e.CreatedAt,
	)
	if err != nil {
		return err
	}
	e.ID, err = res.LastInsertId()
	return err
}

func (s *Store) UpdateEvent(e *Event) error {
	if err := e.prepare(); err != nil {
		return err
	}
	res, err := s.db.Exec(
		`UPDATE events SET title = ?, kind = ?, date = ?, time = ?, project_id = ?, client_id = ?, notes = ? WHERE id = ?`,
		e.Title, e.Kind, e.Date, e.Time, e.ProjectID, e.ClientID, e.Notes, e.ID,
	)
	return affectedOne(res, err)
}

func (s *Store) DeleteEvent(id int64) error {
	res, err := s.db.Exec("DELETE FROM events WHERE id = ?", id)
	return affectedOne(res, err)
}

func (s *Store) GetEvent(id int64) (Event, error) {
	e, err := scanEvent(s.db.QueryRow(eventQuery+" WHERE e.id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	return e, err
}

// ListEvents devuelve los eventos entre from y to (YYYY-MM-DD, inclusive).
// Un límite vacío no acota por ese lado.
func (s *Store) ListEvents(from, to string) ([]Event, error) {
	var where []string
	var args []any
	if from != "" {
		where = append(where, "e.date >= ?")
		args = append(args, from)
	}
	if to != "" {
		where = append(where, "e.date <= ?")
		args = append(args, to)
	}
	query := eventQuery
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	// Dentro de un día: primero los de todo el día, luego por hora.
	query += " ORDER BY e.date, e.time IS NOT NULL, e.time, e.id"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

const eventQuery = `	SELECT e.id, e.title, e.kind, e.date, e.time, e.project_id, e.client_id, e.notes, e.created_at,
	       COALESCE(p.name, ''), COALESCE(c.name, pc.name, '')
	FROM events e
	LEFT JOIN projects p ON p.id = e.project_id
	LEFT JOIN clients c ON c.id = e.client_id
	LEFT JOIN clients pc ON pc.id = p.client_id`

func scanEvent(row scanner) (Event, error) {
	var e Event
	var clock sql.NullString
	var project, client sql.NullInt64
	err := row.Scan(&e.ID, &e.Title, &e.Kind, &e.Date, &clock, &project, &client, &e.Notes, &e.CreatedAt,
		&e.ProjectName, &e.ClientName)
	e.Time = strPtr(clock)
	if project.Valid {
		e.ProjectID = &project.Int64
	}
	if client.Valid {
		e.ClientID = &client.Int64
	}
	return e, err
}
