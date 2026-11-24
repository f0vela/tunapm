package web

import (
	"net/http"
	"strings"

	"tunapm/internal/dates"
	"tunapm/internal/money"
	"tunapm/internal/store"
)

// ---------- dashboard ----------

type dashData struct {
	Tasks    []store.Task
	Payments []store.Payment
	Overdue  int
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.repo.ListTasks(store.TaskFilter{Mode: store.FilterOpen})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	pays, err := s.repo.ListPayments()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	today := dates.Today()
	d := dashData{}
	for _, t := range tasks {
		if len(d.Tasks) < 8 {
			d.Tasks = append(d.Tasks, t)
		}
	}
	for _, p := range pays {
		if p.PaidDate != nil {
			continue
		}
		if p.StatusToday(today) == store.PaymentVencido {
			d.Overdue++
		}
		if len(d.Payments) < 6 {
			d.Payments = append(d.Payments, p)
		}
	}
	s.render(w, r, "dashboard", "inicio", "Inicio", d, 200, "")
}

// ---------- tasks ----------

type tasksData struct {
	Tasks    []store.Task
	Mode     string
	Q        string
	Projects []store.Project
	Project  int64
}

func (s *Server) tasksList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.TaskFilter{Mode: q.Get("mode"), Search: q.Get("q")}
	if f.Mode == "" {
		f.Mode = store.FilterOpen
	}
	pid := queryID(r, "project_id")
	if pid > 0 {
		f.ProjectID = &pid
	}
	tasks, err := s.repo.ListTasks(f)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	projects, _ := s.repo.ListProjects()
	s.render(w, r, "tasks", "tareas", "Tareas", tasksData{tasks, f.Mode, f.Search, projects, pid}, 200, "")
}

type taskForm struct {
	Task     store.Task
	Projects []store.Project
	Next     string
}

func (s *Server) taskFormPage(w http.ResponseWriter, r *http.Request, t store.Task, status int, errMsg string) {
	projects, err := s.repo.ListProjects()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	title := "Nueva tarea"
	if t.ID != 0 {
		title = "Editar tarea"
	}
	s.render(w, r, "task_form", "tareas", title, taskForm{t, projects, r.FormValue("next")}, status, errMsg)
}

func (s *Server) taskNew(w http.ResponseWriter, r *http.Request) {
	s.taskFormPage(w, r, store.Task{ProjectID: queryID(r, "project_id"), Status: store.TaskPendiente, Priority: 1}, 200, "")
}

func (s *Server) taskEdit(w http.ResponseWriter, r *http.Request) {
	t, err := s.repo.GetTask(pathID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.taskFormPage(w, r, t, 200, "")
}

func (s *Server) taskSave(w http.ResponseWriter, r *http.Request) {
	t := store.Task{
		ID:        pathID(r),
		ProjectID: formID(r, "project_id"),
		Title:     strings.TrimSpace(r.FormValue("title")),
		Notes:     strings.TrimSpace(r.FormValue("notes")),
		Status:    r.FormValue("status"),
		DueDate:   optDate(r.FormValue("due_date")),
	}
	t.Priority = int(formID(r, "priority"))
	var err error
	switch {
	case t.Title == "":
		s.taskFormPage(w, r, t, 422, "El título es obligatorio")
		return
	case t.ProjectID == 0:
		s.taskFormPage(w, r, t, 422, "Elige un proyecto")
		return
	}
	if t.ID == 0 {
		err = s.repo.CreateTask(&t)
	} else {
		var old store.Task
		if old, err = s.repo.GetTask(t.ID); err == nil {
			t.CompletedAt = old.CompletedAt
			err = s.repo.UpdateTask(&t)
		}
	}
	if err != nil {
		s.taskFormPage(w, r, t, 422, errText(err))
		return
	}
	back(w, r, "/tasks", "Tarea guardada")
}

func (s *Server) taskToggle(w http.ResponseWriter, r *http.Request) {
	t, err := s.repo.GetTask(pathID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if t.Status == store.TaskHecho {
		t.Status = store.TaskPendiente
	} else {
		t.Status = store.TaskHecho
	}
	if err := s.repo.UpdateTask(&t); err != nil {
		s.fail(w, r, err)
		return
	}
	back(w, r, "/tasks", "")
}

func (s *Server) taskDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.DeleteTask(pathID(r)); err != nil {
		s.fail(w, r, err)
		return
	}
	back(w, r, "/tasks", "Tarea eliminada")
}

// ---------- projects ----------

type projectRow struct {
	store.Project
	Progress store.TaskProgress
	Totals   store.ProjectTotals
}

func (s *Server) projectsList(w http.ResponseWriter, r *http.Request) {
	projects, err := s.repo.ListProjects()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	progress, _ := s.repo.TaskProgress()
	totals, _ := s.repo.AllProjectTotals()
	rows := make([]projectRow, len(projects))
	for i, p := range projects {
		rows[i] = projectRow{p, progress[p.ID], totals[p.ID]}
	}
	s.render(w, r, "projects", "proyectos", "Proyectos", rows, 200, "")
}

type projectDetail struct {
	Project  store.Project
	Tasks    []store.Task
	Payments []store.Payment
	Totals   store.ProjectTotals
	Notes    []store.Note
}

func (s *Server) projectShow(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	p, err := s.repo.GetProject(id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	tasks, _ := s.repo.ListTasks(store.TaskFilter{Mode: store.FilterAll, ProjectID: &id})
	pays, _ := s.repo.PaymentsByProject(id)
	totals, _ := s.repo.ProjectTotals(id)
	notes, _ := s.repo.ListNotes(store.NoteFilter{ProjectID: &id})
	s.render(w, r, "project", "proyectos", p.Name, projectDetail{p, tasks, pays, totals, notes}, 200, "")
}

type projectForm struct {
	Project store.Project
	Clients []store.Client
}

func (s *Server) projectFormPage(w http.ResponseWriter, r *http.Request, p store.Project, status int, errMsg string) {
	clients, err := s.repo.ListClients()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	title := "Nuevo proyecto"
	if p.ID != 0 {
		title = "Editar proyecto"
	}
	s.render(w, r, "project_form", "proyectos", title, projectForm{p, clients}, status, errMsg)
}

func (s *Server) projectNew(w http.ResponseWriter, r *http.Request) {
	s.projectFormPage(w, r, store.Project{ClientID: queryID(r, "client_id"), Status: store.StatusActivo,
		Currency: store.CurrencyGTQ, BillingType: store.BillingTotal}, 200, "")
}

func (s *Server) projectEdit(w http.ResponseWriter, r *http.Request) {
	p, err := s.repo.GetProject(pathID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.projectFormPage(w, r, p, 200, "")
}

func (s *Server) projectSave(w http.ResponseWriter, r *http.Request) {
	p := store.Project{
		ID:          pathID(r),
		ClientID:    formID(r, "client_id"),
		Name:        strings.TrimSpace(r.FormValue("name")),
		Description: strings.TrimSpace(r.FormValue("description")),
		Status:      r.FormValue("status"),
		StartDate:   optDate(r.FormValue("start_date")),
		DueDate:     optDate(r.FormValue("due_date")),
		Currency:    r.FormValue("currency"),
		BillingType: r.FormValue("billing_type"),
	}
	if raw := strings.TrimSpace(r.FormValue("budget")); raw != "" {
		cents, err := money.Parse(raw)
		if err != nil {
			s.projectFormPage(w, r, p, 422, errText(err))
			return
		}
		p.BudgetCents = cents
	}
	if p.Name == "" || p.ClientID == 0 {
		s.projectFormPage(w, r, p, 422, "Nombre y cliente son obligatorios")
		return
	}
	var err error
	if p.ID == 0 {
		err = s.repo.CreateProject(&p)
	} else {
		err = s.repo.UpdateProject(&p)
	}
	if err != nil {
		s.projectFormPage(w, r, p, 422, errText(err))
		return
	}
	redirect(w, r, "/projects/"+itoa(p.ID), "Proyecto guardado")
}

func (s *Server) projectDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.DeleteProject(pathID(r)); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/projects", "Proyecto eliminado")
}

// ---------- clients ----------

func (s *Server) clientsList(w http.ResponseWriter, r *http.Request) {
	clients, err := s.repo.ListClients()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, "clients", "clientes", "Clientes", clients, 200, "")
}

type clientDetail struct {
	Client   store.Client
	Projects []store.Project
}

func (s *Server) clientShow(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	c, err := s.repo.GetClient(id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	projects, _ := s.repo.ProjectsByClient(id)
	s.render(w, r, "client", "clientes", c.Name, clientDetail{c, projects}, 200, "")
}

func (s *Server) clientFormPage(w http.ResponseWriter, r *http.Request, c store.Client, status int, errMsg string) {
	title := "Nuevo cliente"
	if c.ID != 0 {
		title = "Editar cliente"
	}
	s.render(w, r, "client_form", "clientes", title, c, status, errMsg)
}

func (s *Server) clientNew(w http.ResponseWriter, r *http.Request) {
	s.clientFormPage(w, r, store.Client{}, 200, "")
}

func (s *Server) clientEdit(w http.ResponseWriter, r *http.Request) {
	c, err := s.repo.GetClient(pathID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.clientFormPage(w, r, c, 200, "")
}

func (s *Server) clientSave(w http.ResponseWriter, r *http.Request) {
	c := store.Client{
		ID:      pathID(r),
		Name:    strings.TrimSpace(r.FormValue("name")),
		Company: strings.TrimSpace(r.FormValue("company")),
		Email:   strings.TrimSpace(r.FormValue("email")),
		Phone:   strings.TrimSpace(r.FormValue("phone")),
		Notes:   strings.TrimSpace(r.FormValue("notes")),
	}
	if c.Name == "" {
		s.clientFormPage(w, r, c, 422, "El nombre es obligatorio")
		return
	}
	var err error
	if c.ID == 0 {
		err = s.repo.CreateClient(&c)
	} else {
		err = s.repo.UpdateClient(&c)
	}
	if err != nil {
		s.clientFormPage(w, r, c, 422, errText(err))
		return
	}
	redirect(w, r, "/clients/"+itoa(c.ID), "Cliente guardado")
}

func (s *Server) clientDelete(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if err := s.repo.DeleteClient(id); err != nil {
		c, _ := s.repo.GetClient(id)
		projects, _ := s.repo.ProjectsByClient(id)
		s.render(w, r, "client", "clientes", c.Name, clientDetail{c, projects}, 409, errText(err))
		return
	}
	redirect(w, r, "/clients", "Cliente eliminado")
}

// ---------- payments ----------

func (s *Server) paymentsList(w http.ResponseWriter, r *http.Request) {
	pays, err := s.repo.ListPayments()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, "payments", "cuotas", "Cuotas", pays, 200, "")
}

type paymentForm struct {
	Payment  store.Payment
	Projects []store.Project
	Next     string
}

func (s *Server) paymentFormPage(w http.ResponseWriter, r *http.Request, p store.Payment, status int, errMsg string) {
	projects, err := s.repo.ListProjects()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	title := "Nueva cuota"
	if p.ID != 0 {
		title = "Editar cuota"
	}
	s.render(w, r, "payment_form", "cuotas", title, paymentForm{p, projects, r.FormValue("next")}, status, errMsg)
}

func (s *Server) paymentNew(w http.ResponseWriter, r *http.Request) {
	s.paymentFormPage(w, r, store.Payment{ProjectID: queryID(r, "project_id"), ScheduledDate: dates.Today()}, 200, "")
}

func (s *Server) paymentEdit(w http.ResponseWriter, r *http.Request) {
	p, err := s.repo.GetPayment(pathID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.paymentFormPage(w, r, p, 200, "")
}

func (s *Server) paymentSave(w http.ResponseWriter, r *http.Request) {
	p := store.Payment{
		ID:            pathID(r),
		ProjectID:     formID(r, "project_id"),
		ScheduledDate: strings.TrimSpace(r.FormValue("scheduled_date")),
		PaidDate:      optDate(r.FormValue("paid_date")),
		Notes:         strings.TrimSpace(r.FormValue("notes")),
	}
	cents, err := money.Parse(r.FormValue("amount"))
	if err != nil {
		s.paymentFormPage(w, r, p, 422, errText(err))
		return
	}
	p.AmountCents = cents
	if p.ScheduledDate == "" || p.ProjectID == 0 {
		s.paymentFormPage(w, r, p, 422, "Proyecto y fecha son obligatorios")
		return
	}
	if p.ID == 0 {
		err = s.repo.CreatePayment(&p)
	} else {
		err = s.repo.UpdatePayment(&p)
	}
	if err != nil {
		s.paymentFormPage(w, r, p, 422, errText(err))
		return
	}
	back(w, r, "/payments", "Cuota guardada")
}

func (s *Server) paymentPay(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.MarkPaymentPaid(pathID(r), dates.Today()); err != nil {
		s.fail(w, r, err)
		return
	}
	back(w, r, "/payments", "Cuota marcada como pagada")
}

func (s *Server) paymentUnpay(w http.ResponseWriter, r *http.Request) {
	pay, err := s.repo.GetPayment(pathID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	pay.PaidDate = nil
	if err := s.repo.UpdatePayment(&pay); err != nil {
		s.fail(w, r, err)
		return
	}
	back(w, r, "/payments", "Cuota marcada como no pagada")
}

func (s *Server) paymentDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.DeletePayment(pathID(r)); err != nil {
		s.fail(w, r, err)
		return
	}
	back(w, r, "/payments", "Cuota eliminada")
}

// ---------- notes ----------

type notesData struct {
	Notes    []store.Note
	Tags     []store.TagCount
	Projects []store.Project
	Q        string
	Tag      string
	Project  string // "", "none" o el id del proyecto
}

func (s *Server) notesList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	d := notesData{Q: q.Get("q"), Tag: store.NormalizeTag(q.Get("tag")), Project: q.Get("project_id")}
	f := store.NoteFilter{Search: d.Q, Tag: d.Tag}
	if d.Project == "none" {
		f.Unassigned = true
	} else if pid := queryID(r, "project_id"); pid > 0 {
		f.ProjectID = &pid
	} else {
		d.Project = ""
	}
	var err error
	if d.Notes, err = s.repo.ListNotes(f); err != nil {
		s.fail(w, r, err)
		return
	}
	d.Tags, _ = s.repo.NoteTags()
	d.Projects, _ = s.repo.ListProjects()
	s.render(w, r, "notes", "notas", "Notas", d, 200, "")
}

type noteForm struct {
	Note     store.Note
	Projects []store.Project
	Tags     []store.TagCount
	Next     string
}

func (s *Server) noteFormPage(w http.ResponseWriter, r *http.Request, n store.Note, status int, errMsg string) {
	projects, err := s.repo.ListProjects()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	tags, _ := s.repo.NoteTags()
	title := "Nueva nota"
	if n.ID != 0 {
		title = "Editar nota"
	}
	s.render(w, r, "note_form", "notas", title, noteForm{n, projects, tags, r.FormValue("next")}, status, errMsg)
}

func (s *Server) noteNew(w http.ResponseWriter, r *http.Request) {
	n := store.Note{Tags: store.ParseTags(r.URL.Query().Get("tag"))}
	if pid := queryID(r, "project_id"); pid > 0 {
		n.ProjectID = &pid
	}
	s.noteFormPage(w, r, n, 200, "")
}

func (s *Server) noteEdit(w http.ResponseWriter, r *http.Request) {
	n, err := s.repo.GetNote(pathID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.noteFormPage(w, r, n, 200, "")
}

func (s *Server) noteSave(w http.ResponseWriter, r *http.Request) {
	n := store.Note{
		ID:    pathID(r),
		Title: strings.TrimSpace(r.FormValue("title")),
		Body:  strings.ReplaceAll(strings.TrimSpace(r.FormValue("body")), "\r\n", "\n"),
		Tags:  store.ParseTags(r.FormValue("tags")),
	}
	if pid := formID(r, "project_id"); pid > 0 {
		n.ProjectID = &pid
	}
	if n.Title == "" {
		s.noteFormPage(w, r, n, 422, "El título es obligatorio")
		return
	}
	var err error
	if n.ID == 0 {
		err = s.repo.CreateNote(&n)
	} else {
		err = s.repo.UpdateNote(&n)
	}
	if err != nil {
		s.noteFormPage(w, r, n, 422, errText(err))
		return
	}
	back(w, r, "/notes", "Nota guardada")
}

func (s *Server) noteDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.DeleteNote(pathID(r)); err != nil {
		s.fail(w, r, err)
		return
	}
	back(w, r, "/notes", "Nota eliminada")
}
