// Package web es la web app (HTML renderizado en el servidor) sobre store.Repo.
// Pensada para móvil: sin JavaScript, formularios simples y CSS responsive.
package web

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"tunapm/internal/dates"
	"tunapm/internal/money"
	"tunapm/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

const cookieName = "tuna_session"

type Server struct {
	repo     store.Repo
	password string
	secret   []byte
	secure   bool
	tpl      map[string]*template.Template
	mux      *http.ServeMux
}

type Options struct {
	// Password protege la web app. Vacío = sin login (solo localhost/VPN).
	Password string
	// SecureCookie marca la cookie como Secure (usar detrás de HTTPS).
	SecureCookie bool
}

func New(repo store.Repo, opt Options) (http.Handler, error) {
	s := &Server{
		repo:     repo,
		password: opt.Password,
		secure:   opt.SecureCookie,
		secret:   []byte("tuna-web:" + opt.Password),
		mux:      http.NewServeMux(),
	}
	if err := s.loadTemplates(); err != nil {
		return nil, err
	}
	s.routes()
	return s.logging(s.mux), nil
}

var funcs = template.FuncMap{
	"money": money.Format,
	"human": func(d *string, today string) string {
		if d == nil || *d == "" {
			return ""
		}
		return dates.Human(*d, today)
	},
	"humanS": func(d, today string) string { return dates.Human(d, today) },
	"deref": func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	},
	"amount": func(cents int64) string {
		if cents == 0 {
			return ""
		}
		return fmt.Sprintf("%d.%02d", cents/100, cents%100)
	},
	"pct": func(done, total int) int {
		if total == 0 {
			return 0
		}
		return done * 100 / total
	},
	"statusLabel": func(s string) string { return strings.ReplaceAll(s, "_", " ") },
	"prio":        func(p int) string { return []string{"baja", "normal", "alta", "urgente"}[max(0, min(3, p))] },
	"payStatus":   func(p store.Payment, today string) string { return p.StatusToday(today) },
	"list":        func(v ...string) []string { return v },
	"ctx":         func(items any, today, url string) listCtx { return listCtx{items, today, url} },
	"asset":       asset,
	"join":        strings.Join,
	"localDate":   store.LocalDate,
	"derefID": func(id *int64) int64 {
		if id == nil {
			return 0
		}
		return *id
	},
	// excerpt resume el texto de una nota en una línea para las listas.
	"excerpt": func(body string) string {
		line := strings.Join(strings.Fields(body), " ")
		if r := []rune(line); len(r) > 140 {
			return string(r[:140]) + "…"
		}
		return line
	},
}

var assetVersions sync.Map

// asset devuelve la URL de un estático con la huella de su contenido, para que
// el navegador (sobre todo Safari en iOS) no siga usando una copia vieja.
func asset(name string) string {
	if v, ok := assetVersions.Load(name); ok {
		return v.(string)
	}
	url := "/static/" + name
	if data, err := staticFS.ReadFile("static/" + name); err == nil {
		sum := sha256.Sum256(data)
		url += "?v=" + hex.EncodeToString(sum[:4])
	}
	assetVersions.Store(name, url)
	return url
}

func (s *Server) loadTemplates() error {
	s.tpl = map[string]*template.Template{}
	// Todas las plantillas comparten un solo conjunto para poder reusar parciales.
	all, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return err
	}
	s.tpl["all"] = all
	return nil
}

// listCtx es el contexto de los parciales de filas (tareas, cuotas).
type listCtx struct {
	Items any
	Today string
	URL   string
}

type page struct {
	Title   string
	Nav     string
	Today   string
	Flash   string
	Error   string
	Content template.HTML
	Auth    bool
	URL     string
	Data    any
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name, nav, title string, data any, status int, errMsg string) {
	p := page{Title: title, Nav: nav, Today: dates.Today(), Data: data, Auth: s.password != "", Error: errMsg, URL: r.URL.RequestURI()}
	p.Flash = r.URL.Query().Get("ok")
	t := s.tpl["all"]
	if t.Lookup(name) == nil {
		http.Error(w, "plantilla no encontrada: "+name, 500)
		return
	}
	var body bytes.Buffer
	if err := t.ExecuteTemplate(&body, name, p); err != nil {
		log.Println("template:", name, err)
		http.Error(w, "error interno", 500)
		return
	}
	p.Content = template.HTML(body.String())
	var out bytes.Buffer
	if err := t.ExecuteTemplate(&out, "layout", p); err != nil {
		log.Println("layout:", err)
		http.Error(w, "error interno", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(out.Bytes())
}

func (s *Server) routes() {
	m := s.mux
	static, _ := fs.Sub(staticFS, "static")
	files := http.StripPrefix("/static/", http.FileServerFS(static))
	m.HandleFunc("GET /static/", func(w http.ResponseWriter, r *http.Request) {
		// Con huella (?v=) el archivo nunca cambia; sin ella se revalida siempre.
		if r.URL.Query().Has("v") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
	m.HandleFunc("GET /login", s.loginForm)
	m.HandleFunc("POST /login", s.loginSubmit)
	m.HandleFunc("POST /logout", s.logout)

	g := func(pattern string, h http.HandlerFunc) { m.HandleFunc(pattern, s.guard(h)) }
	g("GET /{$}", s.dashboard)

	g("GET /tasks", s.tasksList)
	g("GET /tasks/new", s.taskNew)
	g("POST /tasks", s.taskSave)
	g("GET /tasks/{id}/edit", s.taskEdit)
	g("POST /tasks/{id}", s.taskSave)
	g("POST /tasks/{id}/toggle", s.taskToggle)
	g("POST /tasks/{id}/delete", s.taskDelete)

	g("GET /projects", s.projectsList)
	g("GET /projects/new", s.projectNew)
	g("POST /projects", s.projectSave)
	g("GET /projects/{id}", s.projectShow)
	g("GET /projects/{id}/edit", s.projectEdit)
	g("POST /projects/{id}", s.projectSave)
	g("POST /projects/{id}/delete", s.projectDelete)

	g("GET /clients", s.clientsList)
	g("GET /clients/new", s.clientNew)
	g("POST /clients", s.clientSave)
	g("GET /clients/{id}", s.clientShow)
	g("GET /clients/{id}/edit", s.clientEdit)
	g("POST /clients/{id}", s.clientSave)
	g("POST /clients/{id}/delete", s.clientDelete)

	g("GET /notes", s.notesList)
	g("GET /notes/new", s.noteNew)
	g("POST /notes", s.noteSave)
	g("GET /notes/{id}/edit", s.noteEdit)
	g("POST /notes/{id}", s.noteSave)
	g("POST /notes/{id}/delete", s.noteDelete)

	g("GET /payments", s.paymentsList)
	g("GET /payments/new", s.paymentNew)
	g("POST /payments", s.paymentSave)
	g("GET /payments/{id}/edit", s.paymentEdit)
	g("POST /payments/{id}", s.paymentSave)
	g("POST /payments/{id}/pay", s.paymentPay)
	g("POST /payments/{id}/unpay", s.paymentUnpay)
	g("POST /payments/{id}/delete", s.paymentDelete)
}

// --- auth ---

func (s *Server) sign() string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte("session-v1"))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *Server) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.password != "" {
			c, err := r.Cookie(cookieName)
			if err != nil || subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.sign())) != 1 {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) loginForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "login", "", "Entrar", nil, 200, "")
}

func (s *Server) loginSubmit(w http.ResponseWriter, r *http.Request) {
	time.Sleep(300 * time.Millisecond) // frena fuerza bruta barata
	if subtle.ConstantTimeCompare([]byte(r.FormValue("password")), []byte(s.password)) != 1 {
		s.render(w, r, "login", "", "Entrar", nil, http.StatusUnauthorized, "Contraseña incorrecta")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: s.sign(), Path: "/", HttpOnly: true, Secure: s.secure,
		SameSite: http.SameSiteStrictMode, MaxAge: 30 * 24 * 3600,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.secure})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(c int) { w.status = c; w.ResponseWriter.WriteHeader(c) }

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		if !strings.HasPrefix(r.URL.Path, "/static/") {
			log.Printf("web %s %s %d", r.Method, r.URL.Path, sw.status)
		}
	})
}

// --- helpers compartidos ---

func pathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

func queryID(r *http.Request, key string) int64 {
	id, _ := strconv.ParseInt(r.URL.Query().Get(key), 10, 64)
	return id
}

func formID(r *http.Request, key string) int64 {
	id, _ := strconv.ParseInt(r.FormValue(key), 10, 64)
	return id
}

func optDate(raw string) *string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return &raw
}

func redirect(w http.ResponseWriter, r *http.Request, to, flash string) {
	if flash != "" {
		sep := "?"
		if strings.Contains(to, "?") {
			sep = "&"
		}
		to += sep + "ok=" + strings.ReplaceAll(flash, " ", "+")
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

// back vuelve a la página indicada en el campo "next" (solo rutas locales).
func back(w http.ResponseWriter, r *http.Request, fallback, flash string) {
	next := r.FormValue("next")
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		next = fallback
	}
	redirect(w, r, next, flash)
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		s.render(w, r, "error", "", "No encontrado", "No encontrado", http.StatusNotFound, "")
		return
	}
	log.Println("web error:", err)
	s.render(w, r, "error", "", "Error", err.Error(), http.StatusInternalServerError, "")
}

func errText(err error) string {
	switch {
	case errors.Is(err, money.ErrInvalidAmount):
		return "Monto inválido"
	case errors.Is(err, store.ErrClientHasProjects):
		return "El cliente tiene proyectos; elimínalos o muévelos primero."
	}
	return err.Error()
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
