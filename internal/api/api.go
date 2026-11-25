// Package api expone el store de SQLite como API REST JSON.
package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tunapm/internal/agenda"
	"tunapm/internal/dates"
	"tunapm/internal/store"
)

// Códigos de error estables que el cliente traduce a errores del store.
const (
	CodeNotFound      = "not_found"
	CodeClientHasProj = "client_has_projects"
	CodeInvalidAmount = "invalid_amount"
	CodeTotalOneQuote = "total_one_quote"
	CodeBadRequest    = "bad_request"
	CodeUnauthorized  = "unauthorized"
	CodeInternal      = "internal"
	maxBody           = 1 << 20
)

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"error"`
}

type Server struct {
	repo  store.Repo
	token string
	mux   *http.ServeMux
}

// New crea el handler. Si token está vacío, el API queda sin autenticación
// (solo recomendable en localhost).
func New(repo store.Repo, token string) http.Handler {
	s := &Server{repo: repo, token: token, mux: http.NewServeMux()}
	s.routes()
	return s.logging(s.auth(s.mux))
}

func (s *Server) routes() {
	m := s.mux
	m.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })

	m.HandleFunc("GET /api/snapshot", s.snapshot)

	m.HandleFunc("GET /api/clients", list(s.repo.ListClients))
	m.HandleFunc("POST /api/clients", create(s.repo.CreateClient))
	m.HandleFunc("GET /api/clients/{id}", get(s.repo.GetClient))
	m.HandleFunc("PUT /api/clients/{id}", update(s.repo.UpdateClient, func(c *store.Client, id int64) { c.ID = id }))
	m.HandleFunc("DELETE /api/clients/{id}", del(s.repo.DeleteClient))
	m.HandleFunc("GET /api/clients/{id}/projects", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		out, err := s.repo.ProjectsByClient(id)
		respond(w, 200, nonNil(out), err)
	})
	m.HandleFunc("GET /api/clients/{id}/project-count", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		n, err := s.repo.CountProjectsByClient(id)
		respond(w, 200, map[string]int{"count": n}, err)
	})

	m.HandleFunc("GET /api/projects", list(s.repo.ListProjects))
	m.HandleFunc("POST /api/projects", create(s.repo.CreateProject))
	m.HandleFunc("GET /api/projects/totals", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.repo.AllProjectTotals()
		respond(w, 200, out, err)
	})
	m.HandleFunc("GET /api/projects/progress", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.repo.TaskProgress()
		respond(w, 200, out, err)
	})
	m.HandleFunc("GET /api/projects/{id}", get(s.repo.GetProject))
	m.HandleFunc("PUT /api/projects/{id}", update(s.repo.UpdateProject, func(p *store.Project, id int64) { p.ID = id }))
	m.HandleFunc("DELETE /api/projects/{id}", del(s.repo.DeleteProject))
	m.HandleFunc("GET /api/projects/{id}/payments", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		out, err := s.repo.PaymentsByProject(id)
		respond(w, 200, nonNil(out), err)
	})
	m.HandleFunc("GET /api/projects/{id}/totals", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		out, err := s.repo.ProjectTotals(id)
		respond(w, 200, out, err)
	})

	m.HandleFunc("GET /api/tasks", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f := store.TaskFilter{Mode: q.Get("mode"), Search: q.Get("search")}
		if v, err := strconv.ParseInt(q.Get("project_id"), 10, 64); err == nil {
			f.ProjectID = &v
		}
		if v, err := strconv.ParseInt(q.Get("client_id"), 10, 64); err == nil {
			f.ClientID = &v
		}
		out, err := s.repo.ListTasks(f)
		respond(w, 200, nonNil(out), err)
	})
	m.HandleFunc("POST /api/tasks", create(s.repo.CreateTask))
	m.HandleFunc("GET /api/tasks/{id}", get(s.repo.GetTask))
	m.HandleFunc("PUT /api/tasks/{id}", update(s.repo.UpdateTask, func(t *store.Task, id int64) { t.ID = id }))
	m.HandleFunc("DELETE /api/tasks/{id}", del(s.repo.DeleteTask))

	m.HandleFunc("GET /api/notes", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f := store.NoteFilter{Tag: q.Get("tag"), Search: q.Get("search")}
		// project_id=none pide las notas sin proyecto.
		if q.Get("project_id") == "none" {
			f.Unassigned = true
		} else if v, err := strconv.ParseInt(q.Get("project_id"), 10, 64); err == nil {
			f.ProjectID = &v
		}
		out, err := s.repo.ListNotes(f)
		respond(w, 200, nonNil(out), err)
	})
	m.HandleFunc("GET /api/notes/tags", list(s.repo.NoteTags))
	m.HandleFunc("POST /api/notes", create(s.repo.CreateNote))
	m.HandleFunc("GET /api/notes/{id}", get(s.repo.GetNote))
	m.HandleFunc("PUT /api/notes/{id}", update(s.repo.UpdateNote, func(n *store.Note, id int64) { n.ID = id }))
	m.HandleFunc("DELETE /api/notes/{id}", del(s.repo.DeleteNote))

	m.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.repo.ListEvents(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
		respond(w, 200, nonNil(out), err)
	})
	m.HandleFunc("POST /api/events", create(s.repo.CreateEvent))
	m.HandleFunc("GET /api/events/{id}", get(s.repo.GetEvent))
	m.HandleFunc("PUT /api/events/{id}", update(s.repo.UpdateEvent, func(e *store.Event, id int64) { e.ID = id }))
	m.HandleFunc("DELETE /api/events/{id}", del(s.repo.DeleteEvent))

	// Calendario: tareas, cuotas por cobrar, fechas de proyectos y eventos en un rango.
	m.HandleFunc("GET /api/agenda", func(w http.ResponseWriter, r *http.Request) {
		from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
		if !validDate(from) || !validDate(to) {
			writeErr(w, 400, CodeBadRequest, "from y to son obligatorios (YYYY-MM-DD)")
			return
		}
		out, err := agenda.Build(s.repo, from, to, dates.Today())
		respond(w, 200, out, err)
	})

	m.HandleFunc("GET /api/payments", list(s.repo.ListPayments))
	m.HandleFunc("POST /api/payments", create(s.repo.CreatePayment))
	m.HandleFunc("GET /api/payments/{id}", get(s.repo.GetPayment))
	m.HandleFunc("PUT /api/payments/{id}", update(s.repo.UpdatePayment, func(p *store.Payment, id int64) { p.ID = id }))
	m.HandleFunc("DELETE /api/payments/{id}", del(s.repo.DeletePayment))
	m.HandleFunc("POST /api/payments/{id}/pay", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		var body struct {
			Date string `json:"date"`
		}
		if !decode(w, r, &body) {
			return
		}
		if body.Date == "" {
			writeErr(w, 400, CodeBadRequest, "falta date (YYYY-MM-DD)")
			return
		}
		if err := s.repo.MarkPaymentPaid(id, body.Date); err != nil {
			fail(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// snapshot sirve todos los datos con ETag: la caché de la TUI pregunta barato si hubo cambios.
func (s *Server) snapshot(w http.ResponseWriter, r *http.Request) {
	snap, err := s.repo.Snapshot()
	if err != nil {
		fail(w, err)
		return
	}
	body, err := json.Marshal(snap)
	if err != nil {
		fail(w, err)
		return
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	if strings.TrimPrefix(r.Header.Get("If-None-Match"), "W/") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(body)
}

// --- generic handler builders ---

func list[T any](fn func() ([]T, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := fn()
		respond(w, 200, nonNil(out), err)
	}
}

func get[T any](fn func(int64) (T, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		out, err := fn(id)
		respond(w, 200, out, err)
	}
}

func create[T any](fn func(*T) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var v T
		if !decode(w, r, &v) {
			return
		}
		if err := fn(&v); err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, v)
	}
}

func update[T any](fn func(*T) error, setID func(*T, int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		var v T
		if !decode(w, r, &v) {
			return
		}
		setID(&v, id)
		if err := fn(&v); err != nil {
			fail(w, err)
			return
		}
		writeJSON(w, 200, v)
	}
}

func del(fn func(int64) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		if err := fn(id); err != nil {
			fail(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- helpers ---

func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, CodeBadRequest, "id inválido")
		return 0, false
	}
	return id, true
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, 400, CodeBadRequest, "JSON inválido: "+err.Error())
		return false
	}
	return true
}

func respond(w http.ResponseWriter, status int, v any, err error) {
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, status, v)
}

func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, 404, CodeNotFound, err.Error())
	case errors.Is(err, store.ErrClientHasProjects):
		writeErr(w, 409, CodeClientHasProj, err.Error())
	case errors.Is(err, store.ErrTotalBillingOneQuote):
		writeErr(w, 409, CodeTotalOneQuote, err.Error())
	case errors.Is(err, store.ErrInvalidAmount):
		writeErr(w, 422, CodeInvalidAmount, err.Error())
	case errors.Is(err, store.ErrNoteTitle), errors.Is(err, store.ErrEventTitle),
		errors.Is(err, store.ErrEventDate), errors.Is(err, store.ErrEventTime):
		writeErr(w, 422, CodeBadRequest, err.Error())
	case strings.Contains(err.Error(), "constraint failed"):
		// CHECK / FOREIGN KEY / NOT NULL de SQLite: datos inválidos del cliente.
		writeErr(w, 422, CodeBadRequest, err.Error())
	default:
		log.Println("api error:", err)
		writeErr(w, 500, CodeInternal, "error interno")
	}
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, errorBody{Code: code, Message: msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// --- middleware ---

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token != "" && r.URL.Path != "/api/health" {
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
				writeErr(w, 401, CodeUnauthorized, "token inválido")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d", r.Method, r.URL.Path, sw.status)
	})
}
