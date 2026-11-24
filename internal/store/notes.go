package store

import (
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"
)

var ErrNoteTitle = errors.New("la nota necesita un título")

// Note es una nota libre; ProjectID nil significa que no pertenece a ningún proyecto.
type Note struct {
	ID        int64    `json:"id"`
	ProjectID *int64   `json:"project_id"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Tags      []string `json:"tags"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`

	// Solo lectura, rellenado por las consultas con JOIN (vacíos si no tiene proyecto).
	ProjectName string `json:"project_name"`
	ClientName  string `json:"client_name"`
}

type NoteFilter struct {
	ProjectID  *int64 // notas de este proyecto
	Unassigned bool   // solo notas sin proyecto (ignora ProjectID)
	Tag        string // una etiqueta exacta
	Search     string // en título, cuerpo, etiquetas o nombre del proyecto
}

// TagCount es una etiqueta con el número de notas que la usan.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// NormalizeTag deja una etiqueta en minúsculas, sin "#" ni comas y con guiones en vez de espacios.
func NormalizeTag(raw string) string {
	t := strings.ToLower(strings.TrimSpace(raw))
	t = strings.TrimLeft(t, "#")
	t = strings.ReplaceAll(t, ",", " ")
	return strings.Join(strings.Fields(t), "-")
}

// NormalizeTags normaliza, quita vacías y duplicadas, y conserva el orden.
func NormalizeTags(raw []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, r := range raw {
		t := NormalizeTag(r)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// ParseTags separa un texto escrito por el usuario ("idea, reunión #cliente") en etiquetas.
// Separan la coma y el espacio; para una etiqueta de varias palabras usa guiones.
func ParseTags(text string) []string {
	return NormalizeTags(strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' }))
}

func splitTags(column string) []string {
	if column == "" {
		return []string{}
	}
	return strings.Split(column, ",")
}

func (n *Note) prepare() error {
	n.Title = strings.TrimSpace(n.Title)
	if n.Title == "" {
		return ErrNoteTitle
	}
	n.Tags = NormalizeTags(n.Tags)
	if n.ProjectID != nil && *n.ProjectID == 0 {
		n.ProjectID = nil
	}
	return nil
}

func (s *Store) CreateNote(n *Note) error {
	if err := n.prepare(); err != nil {
		return err
	}
	n.CreatedAt, n.UpdatedAt = now(), now()
	res, err := s.db.Exec(
		`INSERT INTO notes (project_id, title, body, tags, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		n.ProjectID, n.Title, n.Body, strings.Join(n.Tags, ","), n.CreatedAt, n.UpdatedAt,
	)
	if err != nil {
		return err
	}
	n.ID, err = res.LastInsertId()
	return err
}

func (s *Store) UpdateNote(n *Note) error {
	if err := n.prepare(); err != nil {
		return err
	}
	n.UpdatedAt = now()
	res, err := s.db.Exec(
		`UPDATE notes SET project_id = ?, title = ?, body = ?, tags = ?, updated_at = ? WHERE id = ?`,
		n.ProjectID, n.Title, n.Body, strings.Join(n.Tags, ","), n.UpdatedAt, n.ID,
	)
	return affectedOne(res, err)
}

func (s *Store) DeleteNote(id int64) error {
	res, err := s.db.Exec("DELETE FROM notes WHERE id = ?", id)
	return affectedOne(res, err)
}

func (s *Store) GetNote(id int64) (Note, error) {
	n, err := scanNote(s.db.QueryRow(noteQuery+" WHERE n.id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Note{}, ErrNotFound
	}
	return n, err
}

// ListNotes devuelve las notas más recientes primero.
func (s *Store) ListNotes(f NoteFilter) ([]Note, error) {
	var where []string
	var args []any
	switch {
	case f.Unassigned:
		where = append(where, "n.project_id IS NULL")
	case f.ProjectID != nil:
		where = append(where, "n.project_id = ?")
		args = append(args, *f.ProjectID)
	}
	if tag := NormalizeTag(f.Tag); tag != "" {
		where = append(where, "(',' || n.tags || ',') LIKE ?")
		args = append(args, "%,"+tag+",%")
	}
	if search := strings.TrimSpace(f.Search); search != "" {
		where = append(where, "(n.title LIKE ? OR n.body LIKE ? OR n.tags LIKE ? OR COALESCE(p.name, '') LIKE ?)")
		needle := "%" + search + "%"
		args = append(args, needle, needle, needle, needle)
	}
	query := noteQuery
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY n.updated_at DESC, n.id DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Note
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// NoteTags lista las etiquetas en uso, de la más usada a la menos.
func (s *Store) NoteTags() ([]TagCount, error) {
	rows, err := s.db.Query("SELECT tags FROM notes WHERE tags <> ''")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return nil, err
		}
		for _, t := range splitTags(column) {
			counts[t]++
		}
	}
	out := make([]TagCount, 0, len(counts))
	for t, c := range counts {
		out = append(out, TagCount{Tag: t, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
	return out, rows.Err()
}

const noteQuery = `	SELECT n.id, n.project_id, n.title, n.body, n.tags, n.created_at, n.updated_at,
	       COALESCE(p.name, ''), COALESCE(c.name, '')
	FROM notes n
	LEFT JOIN projects p ON p.id = n.project_id
	LEFT JOIN clients c ON c.id = p.client_id`

func scanNote(row scanner) (Note, error) {
	var n Note
	var project sql.NullInt64
	var tags string
	err := row.Scan(&n.ID, &project, &n.Title, &n.Body, &tags, &n.CreatedAt, &n.UpdatedAt, &n.ProjectName, &n.ClientName)
	if project.Valid {
		n.ProjectID = &project.Int64
	}
	n.Tags = splitTags(tags)
	return n, err
}

// LocalDate convierte una marca de tiempo guardada en UTC (RFC 3339) a la fecha
// local YYYY-MM-DD; así una nota escrita de noche no aparece con fecha de mañana.
func LocalDate(stamp string) string {
	if t, err := time.Parse(time.RFC3339, stamp); err == nil {
		return t.Local().Format("2006-01-02")
	}
	if len(stamp) >= 10 {
		return stamp[:10]
	}
	return stamp
}
