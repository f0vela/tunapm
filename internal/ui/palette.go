package ui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tunapm/internal/agenda"
	"tunapm/internal/dates"
	"tunapm/internal/money"
	"tunapm/internal/quickadd"
	"tunapm/internal/store"
)

type paletteItem struct {
	kind   string // cmd, client, project, task
	id     int64
	label  string
	detail string
	run    func(*App)
}

type Palette struct {
	st       store.Repo
	input    textinput.Model
	all      []paletteItem
	filtered []paletteItem
	cursor   int
	width    int
	height   int

	quick    bool
	draft    quickadd.Draft
	parseErr error
	preview  []string
}

func newPalette(st store.Repo, prefill string) *Palette {
	ti := textinput.New()
	ti.Placeholder = "comando, entidad o t tarea · c cliente · p proyecto · $ cuota · n nota · e evento"
	ti.Prompt = "› "
	ti.SetValue(prefill)
	ti.CursorEnd()
	ti.Focus()

	p := &Palette{st: st, input: ti, width: 80, height: 24}
	p.all = p.buildItems()
	p.filter()
	return p
}

func (p *Palette) buildItems() []paletteItem {
	items := []paletteItem{
		{kind: "cmd", label: "Nueva tarea", detail: "t", run: func(a *App) {
			a.openTaskForm(store.Task{}, "", a.contextProjectID())
		}},
		{kind: "cmd", label: "Nuevo cliente", detail: "c", run: func(a *App) {
			a.openClientForm(store.Client{})
		}},
		{kind: "cmd", label: "Nuevo proyecto", detail: "p", run: func(a *App) {
			a.openProjectForm(store.Project{}, "", 0)
		}},
		{kind: "cmd", label: "Registrar cobro (cuota)", detail: "$", run: func(a *App) {
			a.openPaymentForm(store.Payment{}, "", a.contextProjectID())
		}},
		{kind: "cmd", label: "Nueva nota", detail: "n", run: func(a *App) {
			a.openNoteForm(store.Note{ProjectID: a.contextProjectRef()})
		}},
		{kind: "cmd", label: "Nuevo evento de calendario", detail: "e", run: func(a *App) {
			date := dates.Today()
			if a.tab == tabCalendar {
				date = a.calendar.date
			}
			a.openEventForm(store.Event{Date: date, Kind: store.EventReunion, ProjectID: a.contextProjectRef()})
		}},
		{kind: "cmd", label: "Cambiar estado de proyecto", detail: "S", run: func(a *App) {
			a.openProjectPicker("Proyecto a cambiar de estado", func(a *App, project store.Project) {
				a.openProjectStatusPicker(project.ID)
			})
		}},
		{kind: "cmd", label: "Ir a Tareas", detail: "1", run: func(a *App) { a.setTab(tabTasks) }},
		{kind: "cmd", label: "Ir a Proyectos", detail: "2", run: func(a *App) { a.setTab(tabProjects) }},
		{kind: "cmd", label: "Ir a Clientes", detail: "3", run: func(a *App) { a.setTab(tabClients) }},
		{kind: "cmd", label: "Ir a Cobros", detail: "4", run: func(a *App) { a.setTab(tabPayments) }},
		{kind: "cmd", label: "Ir a Notas", detail: "5", run: func(a *App) { a.setTab(tabNotes) }},
		{kind: "cmd", label: "Ir a Calendario", detail: "6", run: func(a *App) { a.setTab(tabCalendar) }},
		{kind: "cmd", label: "Filtrar tareas por proyecto o cliente", detail: "f", run: func(a *App) {
			a.openFilterPicker()
		}},
		{kind: "cmd", label: "Cambiar tema de colores", detail: "T", run: func(a *App) { a.openThemePicker() }},
		{kind: "cmd", label: "Ayuda y atajos", detail: "?", run: func(a *App) { a.detail = a.helpReader() }},
	}

	clients, _ := p.st.ListClients()
	for _, c := range clients {
		items = append(items, paletteItem{
			kind: "client", id: c.ID, label: "Cliente · " + c.Name, detail: c.Company,
			run: func(a *App) {
				if a.clients.selectByID(c.ID) {
					a.setTab(tabClients)
				}
			},
		})
	}
	projects, _ := p.st.ListProjects()
	for _, proj := range projects {
		items = append(items, paletteItem{
			kind: "project", id: proj.ID, label: "Proyecto · " + proj.Name,
			detail: proj.ClientName + " · " + projectStatusLabels[proj.Status],
			run: func(a *App) {
				if a.projects.selectByID(proj.ID) {
					a.setTab(tabProjects)
				}
			},
		})
	}
	tasks, _ := p.st.ListTasks(store.TaskFilter{Mode: store.FilterAll})
	for _, task := range tasks {
		items = append(items, paletteItem{
			kind: "task", id: task.ID, label: "Tarea · " + task.Title,
			detail: task.ProjectName + " · " + taskStatusLabels[task.Status],
			run: func(a *App) {
				if a.tasks.selectByID(task.ID) {
					a.setTab(tabTasks)
				}
			},
		})
	}
	notes, _ := p.st.ListNotes(store.NoteFilter{})
	for _, note := range notes {
		items = append(items, paletteItem{
			kind: "note", id: note.ID, label: "Nota · " + note.Title,
			detail: strings.TrimSpace(note.ProjectName + " " + tagText(note.Tags)),
			run: func(a *App) {
				if a.notes.selectByID(note.ID) {
					a.setTab(tabNotes)
				}
			},
		})
	}
	return items
}

func (p *Palette) layout(width, height int) {
	p.width, p.height = width, height
	p.input.Width = max(min(width-14, 70), 20)
}

func (p *Palette) listHeight() int {
	return max(min(p.height-12, 11), 3)
}

func (p *Palette) filter() {
	labels := make([]string, len(p.all))
	for i, item := range p.all {
		labels[i] = item.label + " " + item.detail
	}
	idx := fuzzyIndexes(p.input.Value(), labels)
	p.filtered = make([]paletteItem, len(idx))
	for i, j := range idx {
		p.filtered[i] = p.all[j]
	}
	p.cursor = 0
}

func (p *Palette) refresh(a *App) {
	draft, err := quickadd.Parse(p.input.Value())
	if errors.Is(err, quickadd.ErrNotQuickAdd) {
		p.quick, p.preview, p.parseErr = false, nil, nil
		p.filter()
		return
	}
	p.quick, p.draft, p.parseErr = true, draft, err
	p.preview = previewLines(a, draft, err)
}

func (p *Palette) handleKey(a *App, msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		a.palette = nil
	case "up", "ctrl+p":
		if p.cursor > 0 {
			p.cursor--
		}
	case "down", "ctrl+n":
		if p.cursor < len(p.filtered)-1 {
			p.cursor++
		}
	case "tab":
		if p.quick {
			a.palette = nil
			a.openFormFromDraft(p.draft)
		}
	case "enter":
		p.execute(a)
	default:
		p.input, _ = p.input.Update(msg)
		p.refresh(a)
	}
	return nil
}

func (p *Palette) execute(a *App) {
	if p.quick {
		if p.parseErr != nil {
			if errors.Is(p.parseErr, quickadd.ErrMissingTitle) ||
				errors.Is(p.parseErr, quickadd.ErrMissingAmount) {
				a.palette = nil
				a.openFormFromDraft(p.draft)
				return
			}
			a.errf("%v", p.parseErr)
			return
		}
		a.executeDraft(p.draft)
		return
	}
	if len(p.filtered) == 0 {
		return
	}
	item := p.filtered[p.cursor]
	a.palette = nil
	if item.run != nil {
		item.run(a)
	}
}

// executeDraft crea directamente si están los datos obligatorios; si falta
// algo, abre el formulario correspondiente con lo ya escrito.
func (a *App) executeDraft(d quickadd.Draft) {
	switch d.Kind {
	case quickadd.KindTask:
		proj := resolveProject(a.projects.projects, d.Target)
		if d.Target != "" && proj == nil {
			a.errf("No encontré el proyecto %q", d.Target)
			return
		}
		due, err := parseOptionalDate(d.Date)
		if err != nil {
			a.errf("Fecha inválida: %s", d.Date)
			return
		}
		projectID := a.contextProjectID()
		if proj != nil {
			projectID = proj.ID
		}
		priority := d.Priority
		if priority < 0 {
			priority = 1
		}
		if projectID == 0 {
			a.palette = nil
			a.openTaskForm(store.Task{Title: d.Title, Priority: priority}, d.Date, 0)
			return
		}
		task := store.Task{
			ProjectID: projectID,
			Title:     d.Title,
			Status:    store.TaskPendiente,
			Priority:  priority,
			DueDate:   due,
		}
		if err := a.st.CreateTask(&task); err != nil {
			a.palette = nil
			a.errf("%v", err)
			return
		}
		a.palette = nil
		a.tasks.mode = store.FilterOpen
		a.toastf("Tarea creada")
		a.reloadAll()

	case quickadd.KindClient:
		client := store.Client{Name: d.Title, Company: d.Company}
		if err := a.st.CreateClient(&client); err != nil {
			a.palette = nil
			a.errf("%v", err)
			return
		}
		a.palette = nil
		a.toastf("Cliente creado")
		a.reloadAll()

	case quickadd.KindProject:
		client := resolveClient(a.clients.clients, d.Target)
		if d.Target != "" && client == nil {
			a.errf("No encontré el cliente %q", d.Target)
			return
		}
		due, err := parseOptionalDate(d.Date)
		if err != nil {
			a.errf("Fecha inválida: %s", d.Date)
			return
		}
		if client == nil {
			a.palette = nil
			a.openProjectForm(store.Project{
				Name:        d.Title,
				BudgetCents: d.AmountCents,
				Currency:    d.Currency,
			}, d.Date, 0)
			return
		}
		project := store.Project{
			ClientID:    client.ID,
			Name:        d.Title,
			Status:      store.StatusProspecto,
			BudgetCents: d.AmountCents,
			Currency:    d.Currency,
			BillingType: store.BillingTotal,
			DueDate:     due,
		}
		if err := a.st.CreateProject(&project); err != nil {
			a.palette = nil
			a.errf("%v", err)
			return
		}
		a.palette = nil
		a.toastf("Proyecto creado")
		a.reloadAll()

	case quickadd.KindEvent:
		proj := resolveProject(a.projects.projects, d.Target)
		if d.Target != "" && proj == nil {
			a.errf("No encontré el proyecto %q", d.Target)
			return
		}
		ev, err := eventFromDraft(a, d)
		if err != nil {
			a.errf("%v", err)
			return
		}
		if proj != nil {
			ev.ProjectID = &proj.ID
		}
		if err := a.st.CreateEvent(&ev); err != nil {
			a.palette = nil
			a.errf("%v", err)
			return
		}
		a.palette = nil
		a.toastf("Evento creado")
		a.calendar.date = ev.Date
		a.reloadAll()
		a.setTab(tabCalendar)

	case quickadd.KindNote:
		proj := resolveProject(a.projects.projects, d.Target)
		if d.Target != "" && proj == nil {
			a.errf("No encontré el proyecto %q", d.Target)
			return
		}
		note := store.Note{Title: d.Title, Tags: store.ParseTags(d.Tags)}
		if proj != nil {
			note.ProjectID = &proj.ID
		}
		if err := a.st.CreateNote(&note); err != nil {
			a.palette = nil
			a.errf("%v", err)
			return
		}
		a.palette = nil
		a.toastf("Nota creada · e para escribir el texto")
		a.reloadAll()
		a.notes.selectByID(note.ID)
		a.setTab(tabNotes)

	case quickadd.KindPayment:
		proj := resolveProject(a.projects.projects, d.Target)
		if d.Target != "" && proj == nil {
			a.errf("No encontré el proyecto %q", d.Target)
			return
		}
		scheduled := dates.Today()
		if d.Date != "" {
			parsed, err := dates.Parse(d.Date, time.Now())
			if err != nil {
				a.errf("Fecha inválida: %s", d.Date)
				return
			}
			scheduled = parsed
		}
		projectID := a.contextProjectID()
		if proj != nil {
			projectID = proj.ID
		}
		if projectID == 0 {
			a.palette = nil
			a.openPaymentForm(store.Payment{AmountCents: d.AmountCents}, d.Date, 0)
			return
		}
		payment := store.Payment{ProjectID: projectID, AmountCents: d.AmountCents, ScheduledDate: scheduled}
		if err := a.st.CreatePayment(&payment); err != nil {
			a.palette = nil
			a.errf("%v", err)
			return
		}
		a.palette = nil
		a.toastf("Cuota programada")
		a.reloadAll()
	}
}

func (p *Palette) view() string {
	contentWidth := max(min(p.width-14, 70), 24)
	var lines []string
	lines = append(lines, titleStyle.Render("Paleta de comandos"), p.input.View(), "")

	if p.quick {
		lines = append(lines, p.preview...)
		lines = append(lines, "", renderHints("enter crear · tab abrir formulario · esc cerrar"))
	} else {
		start := 0
		if p.cursor >= p.listHeight() {
			start = p.cursor - p.listHeight() + 1
		}
		for i := start; i < len(p.filtered) && i < start+p.listHeight(); i++ {
			item := p.filtered[i]
			line := padRight(" "+truncate(item.label, contentWidth-12), contentWidth-8) +
				mutedStyle.Render(truncate(item.detail, 8))
			if i == p.cursor {
				lines = append(lines, selectedStyle.Render(line))
			} else {
				lines = append(lines, line)
			}
		}
		if len(p.filtered) == 0 {
			lines = append(lines, mutedStyle.Render("  sin coincidencias"))
		}
		lines = append(lines, "", renderHints("enter ejecutar · esc cerrar · t/c/p/$/n/e crear rápido"))
	}
	return modalStyle.Width(contentWidth + 8).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// previewLines describe en vivo lo que creará la sintaxis rápida.
func previewLines(a *App, d quickadd.Draft, parseErr error) []string {
	var lines []string
	switch d.Kind {
	case quickadd.KindTask:
		lines = append(lines, sectionStyle.Render("Nueva tarea"))
		lines = append(lines, kv("Título", orDefault(d.Title, "—")))
		lines = append(lines, kv("Proyecto", targetOrMissing(a.projects.projects, d.Target)))
		priority := "media (por defecto)"
		if d.Priority >= 0 {
			priority = priorityLabels[d.Priority]
		}
		lines = append(lines, kv("Prioridad", priority))
		lines = append(lines, kv("Vence", datePreview(d.Date)))
	case quickadd.KindClient:
		lines = append(lines, sectionStyle.Render("Nuevo cliente"))
		lines = append(lines, kv("Nombre", orDefault(d.Title, "—")))
		lines = append(lines, kv("Empresa", orDefault(d.Company, "—")))
	case quickadd.KindProject:
		lines = append(lines, sectionStyle.Render("Nuevo proyecto"))
		lines = append(lines, kv("Nombre", orDefault(d.Title, "—")))
		lines = append(lines, kv("Cliente", targetOrMissingClients(a.clients.clients, d.Target)))
		budget := "sin presupuesto"
		if d.AmountCents > 0 {
			budget = money.Format(d.AmountCents, d.Currency)
		}
		lines = append(lines, kv("Presupuesto", budget))
		lines = append(lines, kv("Entrega", datePreview(d.Date)))
	case quickadd.KindEvent:
		lines = append(lines, sectionStyle.Render("Nuevo evento"))
		lines = append(lines, kv("Título", orDefault(d.Title, "-")))
		lines = append(lines, kv("Tipo", store.EventKindLabels[eventKindFor(d)]))
		date := "hoy (por defecto)"
		if a.tab == tabCalendar {
			date = agenda.DayTitle(a.calendar.date) + " (día seleccionado)"
		}
		if d.Date != "" {
			date = datePreview(d.Date)
		}
		lines = append(lines, kv("Fecha", date))
		clock := mutedStyle.Render("todo el día")
		if d.Time != "" {
			clock = d.Time
			if _, err := store.NormalizeTime(d.Time); err != nil {
				clock = errorStyle.Render("inválida: " + d.Time)
			}
		}
		lines = append(lines, kv("Hora", clock))
		project := mutedStyle.Render("sin proyecto")
		if d.Target != "" {
			project = targetOrMissing(a.projects.projects, d.Target)
		}
		lines = append(lines, kv("Proyecto", project))
	case quickadd.KindNote:
		lines = append(lines, sectionStyle.Render("Nueva nota"))
		lines = append(lines, kv("Título", orDefault(d.Title, "-")))
		project := mutedStyle.Render("sin proyecto")
		if d.Target != "" {
			project = targetOrMissing(a.projects.projects, d.Target)
		}
		lines = append(lines, kv("Proyecto", project))
		lines = append(lines, kv("Etiquetas", orDefault(tagText(store.ParseTags(d.Tags)), mutedStyle.Render("ninguna"))))
	case quickadd.KindPayment:
		lines = append(lines, sectionStyle.Render("Nueva cuota"))
		lines = append(lines, kv("Proyecto", targetOrMissing(a.projects.projects, d.Target)))
		lines = append(lines, kv("Monto", money.Format(d.AmountCents, d.Currency)))
		date := d.Date
		if date == "" {
			date = "hoy (por defecto)"
		} else {
			date = datePreview(date)
		}
		lines = append(lines, kv("Programada", date))
	}

	if parseErr != nil {
		lines = append(lines, "", errorStyle.Render("⚠ "+parseErr.Error()))
		if errors.Is(parseErr, quickadd.ErrMissingTitle) || errors.Is(parseErr, quickadd.ErrMissingAmount) {
			lines = append(lines, renderHints("enter abre el formulario para completarlo"))
		}
	}
	return lines
}

func kv(label, value string) string {
	return mutedStyle.Render(fmt.Sprintf("%-12s", label)) + value
}

func orDefault(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func targetOrMissing(projects []store.Project, query string) string {
	if strings.TrimSpace(query) == "" {
		return mutedStyle.Render("sin definir")
	}
	if proj := resolveProject(projects, query); proj != nil {
		return proj.Name + mutedStyle.Render(" · "+proj.ClientName)
	}
	return errorStyle.Render("no encontrado: " + query)
}

func targetOrMissingClients(clients []store.Client, query string) string {
	if strings.TrimSpace(query) == "" {
		return mutedStyle.Render("sin definir")
	}
	if c := resolveClient(clients, query); c != nil {
		return c.Name + mutedStyle.Render(" · "+c.Company)
	}
	return errorStyle.Render("no encontrado: " + query)
}

func datePreview(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return mutedStyle.Render("sin fecha")
	}
	date, err := dates.Parse(raw, time.Now())
	if err != nil {
		return errorStyle.Render("inválida: " + raw)
	}
	return dates.Human(date, dates.Today()) + mutedStyle.Render(" ("+date+")")
}

// eventKindFor decide el tipo de un evento rápido: el #tipo escrito o, si no,
// lo que sugiere el título; la vista previa lo muestra antes de crear.
func eventKindFor(d quickadd.Draft) string {
	guess := func(text string) string {
		t := strings.ToLower(text)
		switch {
		case strings.Contains(t, "present") || strings.Contains(t, "demo"):
			return store.EventPresentacion
		case strings.Contains(t, "lanz") || strings.Contains(t, "launch"):
			return store.EventLanzamiento
		case strings.Contains(t, "llam") || strings.Contains(t, "call"):
			return store.EventLlamada
		case strings.Contains(t, "reuni") || strings.Contains(t, "junta") || strings.Contains(t, "meeting"):
			return store.EventReunion
		}
		return ""
	}
	if d.Tags != "" {
		if k := guess(d.Tags); k != "" {
			return k
		}
		return store.EventOtro
	}
	if k := guess(d.Title); k != "" {
		return k
	}
	return store.EventOtro
}

func eventFromDraft(a *App, d quickadd.Draft) (store.Event, error) {
	ev := store.Event{Title: d.Title, Kind: eventKindFor(d), Date: dates.Today()}
	if a.tab == tabCalendar {
		ev.Date = a.calendar.date
	}
	if d.Date != "" {
		date, err := parseOptionalDate(d.Date)
		if err != nil || date == nil {
			return ev, fmt.Errorf("Fecha inválida: %s", d.Date)
		}
		ev.Date = *date
	}
	if d.Time != "" {
		clock, err := store.NormalizeTime(d.Time)
		if err != nil {
			return ev, err
		}
		ev.Time = &clock
	}
	return ev, nil
}
