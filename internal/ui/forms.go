package ui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/huh"

	"tunapm/internal/agenda"
	"tunapm/internal/dates"
	"tunapm/internal/money"
	"tunapm/internal/quickadd"
	"tunapm/internal/store"
)

func (a *App) openTaskForm(t store.Task, dueRaw string, presetProject int64) {
	projects, err := a.st.ListProjects()
	if err != nil || len(projects) == 0 {
		a.errf("Primero crea un cliente y un proyecto")
		return
	}

	title := t.Title
	notes := t.Notes
	status := t.Status
	if status == "" {
		status = store.TaskPendiente
	}
	priority := t.Priority
	if t.ID == 0 && priority == 0 {
		priority = 1
	}
	projectID := t.ProjectID
	if projectID == 0 {
		projectID = presetProject
	}
	if projectID == 0 {
		projectID = projects[0].ID
	}
	if dueRaw == "" && t.DueDate != nil {
		dueRaw = *t.DueDate
	}

	projectOpts := make([]huh.Option[int64], len(projects))
	for i, p := range projects {
		projectOpts[i] = huh.NewOption(truncate(p.Name+" · "+p.ClientName, 60), p.ID)
	}
	statusOpts := make([]huh.Option[string], len(store.TaskStatuses))
	for i, s := range store.TaskStatuses {
		statusOpts[i] = huh.NewOption(taskStatusLabels[s], s)
	}
	priorityOpts := make([]huh.Option[int], len(priorityLabels))
	for i, label := range priorityLabels {
		priorityOpts[i] = huh.NewOption(label, i)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Título").Value(&title).Validate(huh.ValidateNotEmpty()),
			huh.NewSelect[int64]().Title("Proyecto").Options(projectOpts...).Height(6).Value(&projectID).
				Description(searchHint(len(projectOpts))),
			huh.NewSelect[string]().Title("Estado").Options(statusOpts...).Value(&status).Inline(true),
			huh.NewSelect[int]().Title("Prioridad").Options(priorityOpts...).Value(&priority).Inline(true),
			huh.NewInput().Title("Vence").Placeholder("2026-12-15, mañana, viernes, +3d").Value(&dueRaw).
				DescriptionFunc(datePreviewFunc(&dueRaw, "sin fecha límite"), &dueRaw).
				Validate(validateDate),
			huh.NewText().Title("Notas").Value(&notes).Lines(4),
		),
	)

	a.openForm(form, func() error {
		due, err := parseOptionalDate(dueRaw)
		if err != nil {
			return fmt.Errorf("fecha inválida: %s", dueRaw)
		}
		task := t
		task.Title, task.Notes, task.Status = strings.TrimSpace(title), notes, status
		task.Priority, task.ProjectID, task.DueDate = priority, projectID, due
		if task.ID == 0 {
			if err := a.st.CreateTask(&task); err != nil {
				return err
			}
			a.tasks.mode = store.FilterOpen
			return nil
		}
		return a.st.UpdateTask(&task)
	})
}

func (a *App) openClientForm(c store.Client) {
	name, company, email, phone, notes := c.Name, c.Company, c.Email, c.Phone, c.Notes

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Nombre").Value(&name).Validate(huh.ValidateNotEmpty()),
			huh.NewInput().Title("Empresa").Value(&company),
			huh.NewInput().Title("Email").Value(&email),
			huh.NewInput().Title("Teléfono").Value(&phone),
			huh.NewText().Title("Notas").Value(&notes).Lines(3),
		),
	)

	a.openForm(form, func() error {
		client := c
		client.Name, client.Company = strings.TrimSpace(name), strings.TrimSpace(company)
		client.Email, client.Phone, client.Notes = strings.TrimSpace(email), strings.TrimSpace(phone), notes
		if client.ID == 0 {
			return a.st.CreateClient(&client)
		}
		return a.st.UpdateClient(&client)
	})
}

func (a *App) openProjectForm(p store.Project, dueRaw string, presetClient int64) {
	clients, err := a.st.ListClients()
	if err != nil || len(clients) == 0 {
		a.errf("Primero crea un cliente")
		return
	}

	name, description := p.Name, p.Description
	status := p.Status
	if status == "" {
		status = store.StatusProspecto
	}
	currency := p.Currency
	if currency == "" {
		currency = store.CurrencyGTQ
	}
	billing := p.BillingType
	if billing == "" {
		billing = store.BillingTotal
	}
	clientID := p.ClientID
	if clientID == 0 {
		clientID = presetClient
	}
	if clientID == 0 {
		clientID = clients[0].ID
	}
	startRaw := strOr(p.StartDate, "")
	if dueRaw == "" {
		dueRaw = strOr(p.DueDate, "")
	}
	budgetRaw := ""
	if p.BudgetCents > 0 {
		budgetRaw = money.Format(p.BudgetCents, currency)
	}

	clientOpts := make([]huh.Option[int64], len(clients))
	for i, c := range clients {
		label := c.Name
		if c.Company != "" {
			label += " · " + c.Company
		}
		clientOpts[i] = huh.NewOption(label, c.ID)
	}
	statusOpts := make([]huh.Option[string], len(store.ProjectStatuses))
	for i, s := range store.ProjectStatuses {
		statusOpts[i] = huh.NewOption(projectStatusLabels[s], s)
	}
	currencyOpts := []huh.Option[string]{
		huh.NewOption("GTQ (Q)", store.CurrencyGTQ),
		huh.NewOption("USD ($)", store.CurrencyUSD),
	}
	billingOpts := []huh.Option[string]{
		huh.NewOption("Total (una sola cuota)", store.BillingTotal),
		huh.NewOption("Parcial (varias cuotas)", store.BillingParcial),
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Nombre").Value(&name).Validate(huh.ValidateNotEmpty()),
			huh.NewSelect[int64]().Title("Cliente").Options(clientOpts...).Height(6).Value(&clientID).
				Description(searchHint(len(clientOpts))),
			huh.NewSelect[string]().Title("Estado").Options(statusOpts...).Value(&status).Inline(true),
			huh.NewText().Title("Descripción").Value(&description).Lines(3),
		).Title("Datos del proyecto"),
		huh.NewGroup(
			huh.NewInput().Title("Inicio").Placeholder("hoy, 15/12, +3d").Value(&startRaw).Validate(validateDate).
				DescriptionFunc(datePreviewFunc(&startRaw, "sin fecha de inicio"), &startRaw),
			huh.NewInput().Title("Entrega").Placeholder("2026-12-20").Value(&dueRaw).Validate(validateDate).
				DescriptionFunc(datePreviewFunc(&dueRaw, "sin fecha de entrega"), &dueRaw),
			huh.NewInput().Title("Presupuesto").Placeholder("5000").Value(&budgetRaw).Validate(validateMoney),
			huh.NewSelect[string]().Title("Moneda").Options(currencyOpts...).Value(&currency).Inline(true),
			huh.NewSelect[string]().Title("Cobro").Options(billingOpts...).Value(&billing).Inline(true),
		).Title("Fechas y cobro"),
	)

	a.openForm(form, func() error {
		start, err := parseOptionalDate(startRaw)
		if err != nil {
			return fmt.Errorf("fecha de inicio inválida: %s", startRaw)
		}
		due, err := parseOptionalDate(dueRaw)
		if err != nil {
			return fmt.Errorf("fecha de entrega inválida: %s", dueRaw)
		}
		budget, err := parseOptionalMoney(budgetRaw)
		if err != nil {
			return fmt.Errorf("presupuesto inválido: %s", budgetRaw)
		}
		project := p
		project.ClientID, project.Name = clientID, strings.TrimSpace(name)
		project.Description, project.Status = description, status
		project.StartDate, project.DueDate = start, due
		project.BudgetCents, project.Currency, project.BillingType = budget, currency, billing
		if project.ID == 0 {
			return a.st.CreateProject(&project)
		}
		return a.st.UpdateProject(&project)
	})
}

func (a *App) openPaymentForm(p store.Payment, scheduledRaw string, presetProject int64) {
	projects, err := a.st.ListProjects()
	if err != nil || len(projects) == 0 {
		a.errf("Primero crea un proyecto")
		return
	}

	projectID := p.ProjectID
	if projectID == 0 {
		projectID = presetProject
	}
	if projectID == 0 {
		projectID = projects[0].ID
	}
	amountRaw := ""
	if p.AmountCents > 0 {
		amountRaw = money.Format(p.AmountCents, p.Currency)
	}
	notes := p.Notes
	if scheduledRaw == "" {
		scheduledRaw = p.ScheduledDate
	}
	if scheduledRaw == "" {
		scheduledRaw = dates.Today()
	}

	projectOpts := make([]huh.Option[int64], len(projects))
	for i, proj := range projects {
		projectOpts[i] = huh.NewOption(truncate(proj.Name+" · "+proj.ClientName, 60), proj.ID)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int64]().Title("Proyecto").Options(projectOpts...).Height(6).Value(&projectID).
				Description(searchHint(len(projectOpts))),
			huh.NewInput().Title("Monto").Placeholder("2500").Value(&amountRaw).
				Validate(validateMoney).Validate(huh.ValidateNotEmpty()),
			huh.NewInput().Title("Fecha programada").Value(&scheduledRaw).Validate(validateDate).
				DescriptionFunc(datePreviewFunc(&scheduledRaw, "hoy"), &scheduledRaw),
			huh.NewInput().Title("Notas").Value(&notes),
		),
	)

	a.openForm(form, func() error {
		amount, err := money.Parse(amountRaw)
		if err != nil || amount <= 0 {
			return errors.New("monto inválido")
		}
		scheduled, err := dates.Parse(scheduledRaw, nowTime())
		if err != nil {
			return fmt.Errorf("fecha inválida: %s", scheduledRaw)
		}
		payment := p
		payment.ProjectID, payment.AmountCents = projectID, amount
		payment.ScheduledDate, payment.Notes = scheduled, strings.TrimSpace(notes)
		if payment.ID == 0 {
			return a.st.CreatePayment(&payment)
		}
		return a.st.UpdatePayment(&payment)
	})
}

func (a *App) openPaidDateForm(p store.Payment) {
	date := dates.Today()
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Fecha de pago").Value(&date).Validate(validateDate).
				DescriptionFunc(datePreviewFunc(&date, "hoy"), &date),
		),
	)
	a.openForm(form, func() error {
		parsed, err := dates.Parse(date, nowTime())
		if err != nil {
			return fmt.Errorf("fecha inválida: %s", date)
		}
		if err := a.st.MarkPaymentPaid(p.ID, parsed); err != nil {
			return err
		}
		a.toastf("Cuota marcada como pagada")
		return nil
	})
}

// openFormFromDraft completa con huh lo que la sintaxis rápida no alcanzó.
func (a *App) openFormFromDraft(d quickadd.Draft) {
	switch d.Kind {
	case quickadd.KindTask:
		priority := d.Priority
		if priority < 0 {
			priority = 1
		}
		task := store.Task{Title: d.Title, Priority: priority}
		projectID := a.contextProjectID()
		if proj := resolveProject(a.projects.projects, d.Target); proj != nil {
			projectID = proj.ID
		}
		a.openTaskForm(task, d.Date, projectID)
	case quickadd.KindClient:
		a.openClientForm(store.Client{Name: d.Title, Company: d.Company})
	case quickadd.KindProject:
		project := store.Project{Name: d.Title, BudgetCents: d.AmountCents, Currency: d.Currency}
		clientID := int64(0)
		if client := resolveClient(a.clients.clients, d.Target); client != nil {
			clientID = client.ID
		}
		a.openProjectForm(project, d.Date, clientID)
	case quickadd.KindEvent:
		ev, _ := eventFromDraft(a, d)
		ev.ProjectID = a.contextProjectRef()
		if proj := resolveProject(a.projects.projects, d.Target); proj != nil {
			ev.ProjectID = &proj.ID
		}
		a.openEventForm(ev)
	case quickadd.KindNote:
		note := store.Note{Title: d.Title, Tags: store.ParseTags(d.Tags), ProjectID: a.contextProjectRef()}
		if proj := resolveProject(a.projects.projects, d.Target); proj != nil {
			note.ProjectID = &proj.ID
		}
		a.openNoteForm(note)
	case quickadd.KindPayment:
		payment := store.Payment{AmountCents: d.AmountCents}
		projectID := a.contextProjectID()
		if proj := resolveProject(a.projects.projects, d.Target); proj != nil {
			projectID = proj.ID
		}
		a.openPaymentForm(payment, d.Date, projectID)
	}
}

func (a *App) contextProjectID() int64 {
	if a.tab == tabProjects {
		if p := a.projects.selectedProject(); p != nil {
			return p.ID
		}
	}
	if a.tasks.projectFilter != nil {
		return *a.tasks.projectFilter
	}
	return 0
}

// contextProjectRef es contextProjectID como referencia opcional (nil = sin proyecto).
func (a *App) contextProjectRef() *int64 {
	if a.tab == tabNotes {
		return a.notes.projectFilter
	}
	if id := a.contextProjectID(); id != 0 {
		return &id
	}
	return nil
}

func validateDate(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	if _, err := dates.Parse(s, nowTime()); err != nil {
		return errors.New("usa 2026-12-15, hoy, mañana, viernes o +3d")
	}
	return nil
}

func validateMoney(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	if _, err := money.Parse(s); err != nil {
		return errors.New("monto inválido, por ejemplo 5000 o 5,000.50")
	}
	return nil
}

func parseOptionalMoney(raw string) (int64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	return money.Parse(raw)
}

func nowTime() time.Time { return time.Now() }

// datePreviewFunc muestra, mientras se escribe, cómo se interpreta una fecha:
// "viernes" → "viernes 27 de noviembre 2026". empty describe el campo vacío.
func datePreviewFunc(raw *string, empty string) func() string {
	return func() string {
		text := strings.TrimSpace(*raw)
		if text == "" {
			return "Vacío: " + empty
		}
		date, err := parseDateFor(text)
		if err != nil {
			return "No reconozco la fecha. Prueba: hoy, mañana, viernes, +3d, 15/12"
		}
		return "→ " + agenda.DayTitle(date) + " " + date[:4]
	}
}

// searchHint recuerda que las listas largas se filtran escribiendo "/".
func searchHint(options int) string {
	if options > 6 {
		return "/ para buscar"
	}
	return ""
}
