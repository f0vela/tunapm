// Package client implementa store.Repo sobre el API HTTP de tuna.
package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tunapm/internal/agenda"
	"tunapm/internal/api"
	"tunapm/internal/store"
)

// Errores que la caché usa para decidir si reintenta más tarde o descarta la operación.
var (
	ErrOffline      = errors.New("sin conexión con el servidor") // red caída o 502/503/504/429
	ErrServer       = errors.New("error del servidor")           // 5xx inesperado
	ErrUnauthorized = errors.New("token inválido (revisa TUNA_TOKEN)")
)

type Client struct {
	base  string
	token string
	http  *http.Client
}

var _ store.Repo = (*Client)(nil)

func New(baseURL, token string) *Client {
	return &Client{
		base:  strings.TrimRight(baseURL, "/"),
		token: token,
		http:  &http.Client{Timeout: 15 * time.Second},
	}
}

// Ping verifica conectividad y token.
func (c *Client) Ping() error {
	if _, err := c.ListClients(); err != nil {
		return err
	}
	return nil
}

func (c *Client) do(method, path string, in, out any) error {
	_, _, err := c.doFull(method, path, in, out, nil)
	return err
}

// doFull devuelve además el código y las cabeceras de la respuesta (para ETag/304).
func (c *Client) doFull(method, path string, in, out any, hdr map[string]string) (int, http.Header, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		return 0, nil, err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %v", ErrOffline, err)
	}
	// Leer todo el cuerpo permite reutilizar la conexión (keep-alive).
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode >= 400 {
		var e struct {
			Code    string `json:"code"`
			Message string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&e)
		switch e.Code {
		case api.CodeNotFound:
			return resp.StatusCode, nil, store.ErrNotFound
		case api.CodeClientHasProj:
			return resp.StatusCode, nil, store.ErrClientHasProjects
		case api.CodeTotalOneQuote:
			return resp.StatusCode, nil, store.ErrTotalBillingOneQuote
		case api.CodeInvalidAmount:
			return resp.StatusCode, nil, store.ErrInvalidAmount
		case api.CodeUnauthorized:
			return resp.StatusCode, nil, ErrUnauthorized
		}
		switch resp.StatusCode {
		case 408, 429, 502, 503, 504:
			return resp.StatusCode, nil, fmt.Errorf("%w: el servidor respondió %s", ErrOffline, resp.Status)
		}
		if resp.StatusCode >= 500 {
			return resp.StatusCode, nil, fmt.Errorf("%w: %s", ErrServer, resp.Status)
		}
		if e.Message == "" {
			e.Message = resp.Status
		}
		return resp.StatusCode, nil, errors.New(e.Message)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotModified {
		return resp.StatusCode, resp.Header, nil
	}
	return resp.StatusCode, resp.Header, json.NewDecoder(resp.Body).Decode(out)
}

// Snapshot trae todos los datos en una sola petición.
func (c *Client) Snapshot() (v store.Snapshot, err error) {
	err = c.do("GET", "/api/snapshot", nil, &v)
	return
}

// SnapshotIfChanged es Snapshot condicional: si el ETag no cambió devuelve changed=false.
func (c *Client) SnapshotIfChanged(etag string) (snap store.Snapshot, newTag string, changed bool, err error) {
	hdr := map[string]string{}
	if etag != "" {
		hdr["If-None-Match"] = etag
	}
	status, h, err := c.doFull("GET", "/api/snapshot", nil, &snap, hdr)
	if err != nil {
		return snap, "", false, err
	}
	if status == http.StatusNotModified {
		return snap, etag, false, nil
	}
	return snap, h.Get("ETag"), true, nil
}

func id(n int64) string { return strconv.FormatInt(n, 10) }

// --- clients ---

func (c *Client) CreateClient(v *store.Client) error { return c.do("POST", "/api/clients", v, v) }
func (c *Client) UpdateClient(v *store.Client) error {
	return c.do("PUT", "/api/clients/"+id(v.ID), v, v)
}
func (c *Client) DeleteClient(n int64) error { return c.do("DELETE", "/api/clients/"+id(n), nil, nil) }
func (c *Client) GetClient(n int64) (v store.Client, err error) {
	err = c.do("GET", "/api/clients/"+id(n), nil, &v)
	return
}
func (c *Client) ListClients() (v []store.Client, err error) {
	err = c.do("GET", "/api/clients", nil, &v)
	return
}

// --- projects ---

func (c *Client) CreateProject(v *store.Project) error { return c.do("POST", "/api/projects", v, v) }
func (c *Client) UpdateProject(v *store.Project) error {
	return c.do("PUT", "/api/projects/"+id(v.ID), v, v)
}
func (c *Client) DeleteProject(n int64) error {
	return c.do("DELETE", "/api/projects/"+id(n), nil, nil)
}
func (c *Client) GetProject(n int64) (v store.Project, err error) {
	err = c.do("GET", "/api/projects/"+id(n), nil, &v)
	return
}
func (c *Client) ListProjects() (v []store.Project, err error) {
	err = c.do("GET", "/api/projects", nil, &v)
	return
}
func (c *Client) ProjectsByClient(n int64) (v []store.Project, err error) {
	err = c.do("GET", "/api/clients/"+id(n)+"/projects", nil, &v)
	return
}
func (c *Client) CountProjectsByClient(n int64) (int, error) {
	var out struct {
		Count int `json:"count"`
	}
	err := c.do("GET", "/api/clients/"+id(n)+"/project-count", nil, &out)
	return out.Count, err
}

// --- tasks ---

func (c *Client) CreateTask(v *store.Task) error { return c.do("POST", "/api/tasks", v, v) }
func (c *Client) UpdateTask(v *store.Task) error { return c.do("PUT", "/api/tasks/"+id(v.ID), v, v) }
func (c *Client) DeleteTask(n int64) error       { return c.do("DELETE", "/api/tasks/"+id(n), nil, nil) }
func (c *Client) GetTask(n int64) (v store.Task, err error) {
	err = c.do("GET", "/api/tasks/"+id(n), nil, &v)
	return
}
func (c *Client) ListTasks(f store.TaskFilter) (v []store.Task, err error) {
	q := url.Values{}
	if f.Mode != "" {
		q.Set("mode", f.Mode)
	}
	if f.Search != "" {
		q.Set("search", f.Search)
	}
	if f.ProjectID != nil {
		q.Set("project_id", id(*f.ProjectID))
	}
	if f.ClientID != nil {
		q.Set("client_id", id(*f.ClientID))
	}
	err = c.do("GET", "/api/tasks?"+q.Encode(), nil, &v)
	return
}
func (c *Client) TaskProgress() (v map[int64]store.TaskProgress, err error) {
	err = c.do("GET", "/api/projects/progress", nil, &v)
	return
}

// --- payments ---

func (c *Client) CreatePayment(v *store.Payment) error { return c.do("POST", "/api/payments", v, v) }
func (c *Client) UpdatePayment(v *store.Payment) error {
	return c.do("PUT", "/api/payments/"+id(v.ID), v, v)
}
func (c *Client) MarkPaymentPaid(n int64, date string) error {
	return c.do("POST", "/api/payments/"+id(n)+"/pay", map[string]string{"date": date}, nil)
}
func (c *Client) DeletePayment(n int64) error {
	return c.do("DELETE", "/api/payments/"+id(n), nil, nil)
}
func (c *Client) GetPayment(n int64) (v store.Payment, err error) {
	err = c.do("GET", "/api/payments/"+id(n), nil, &v)
	return
}
func (c *Client) ListPayments() (v []store.Payment, err error) {
	err = c.do("GET", "/api/payments", nil, &v)
	return
}
func (c *Client) PaymentsByProject(n int64) (v []store.Payment, err error) {
	err = c.do("GET", "/api/projects/"+id(n)+"/payments", nil, &v)
	return
}
func (c *Client) ProjectTotals(n int64) (v store.ProjectTotals, err error) {
	err = c.do("GET", "/api/projects/"+id(n)+"/totals", nil, &v)
	return
}
func (c *Client) AllProjectTotals() (v map[int64]store.ProjectTotals, err error) {
	err = c.do("GET", "/api/projects/totals", nil, &v)
	return
}

// --- notes ---

func (c *Client) CreateNote(v *store.Note) error { return c.do("POST", "/api/notes", v, v) }
func (c *Client) UpdateNote(v *store.Note) error { return c.do("PUT", "/api/notes/"+id(v.ID), v, v) }
func (c *Client) DeleteNote(n int64) error       { return c.do("DELETE", "/api/notes/"+id(n), nil, nil) }
func (c *Client) GetNote(n int64) (v store.Note, err error) {
	err = c.do("GET", "/api/notes/"+id(n), nil, &v)
	return
}
func (c *Client) ListNotes(f store.NoteFilter) (v []store.Note, err error) {
	q := url.Values{}
	switch {
	case f.Unassigned:
		q.Set("project_id", "none")
	case f.ProjectID != nil:
		q.Set("project_id", id(*f.ProjectID))
	}
	if f.Tag != "" {
		q.Set("tag", f.Tag)
	}
	if f.Search != "" {
		q.Set("search", f.Search)
	}
	err = c.do("GET", "/api/notes?"+q.Encode(), nil, &v)
	return
}
func (c *Client) NoteTags() (v []store.TagCount, err error) {
	err = c.do("GET", "/api/notes/tags", nil, &v)
	return
}

// --- events ---

func (c *Client) CreateEvent(v *store.Event) error { return c.do("POST", "/api/events", v, v) }
func (c *Client) UpdateEvent(v *store.Event) error {
	return c.do("PUT", "/api/events/"+id(v.ID), v, v)
}
func (c *Client) DeleteEvent(n int64) error { return c.do("DELETE", "/api/events/"+id(n), nil, nil) }
func (c *Client) GetEvent(n int64) (v store.Event, err error) {
	err = c.do("GET", "/api/events/"+id(n), nil, &v)
	return
}
func (c *Client) ListEvents(from, to string) (v []store.Event, err error) {
	q := url.Values{}
	if from != "" {
		q.Set("from", from)
	}
	if to != "" {
		q.Set("to", to)
	}
	err = c.do("GET", "/api/events?"+q.Encode(), nil, &v)
	return
}

// Agenda pide el calendario ya armado por el servidor (la TUI lo arma en local).
func (c *Client) Agenda(from, to string) (v []agenda.Item, err error) {
	q := url.Values{"from": {from}, "to": {to}}
	err = c.do("GET", "/api/agenda?"+q.Encode(), nil, &v)
	return
}
