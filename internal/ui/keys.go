package ui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"tunapm/internal/agenda"
)

// Atajos de teclado. Cada binding es la única fuente de verdad: con él se
// reconoce la tecla (key.Matches), se pinta la barra de estado (help.Model) y
// se arma la ayuda (?). Los que no aplican en el contexto actual se
// deshabilitan con SetEnabled y desaparecen de la barra.

func bind(keys []string, label, desc string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(label, desc))
}

func k(keys ...string) []string { return keys }

// helpGroup es una sección de la ayuda (?).
type helpGroup struct {
	title    string
	bindings []key.Binding
}

// ---------- globales ----------

type globalKeyMap struct {
	Quit, Palette, Help, Theme, Tabs key.Binding
}

var globalKeys = globalKeyMap{
	Quit:    bind(k("ctrl+c"), "ctrl+c", "salir"),
	Palette: bind(k("ctrl+k", ":"), "ctrl+k", "paleta"),
	Help:    bind(k("?"), "?", "ayuda"),
	Theme:   bind(k("T"), "T", "tema de colores"),
	Tabs:    bind(k("1", "2", "3", "4", "5", "6"), "1-6", "cambiar de vista"),
}

// ---------- tareas ----------

type taskKeyMap struct {
	Up, Down, PageUp, PageDown, Top, Bottom    key.Binding
	New, Toggle, Status, PrioUp, PrioDown      key.Binding
	Edit, Open, Delete, GoProject              key.Binding
	Search, Filter, Clear                      key.Binding
	ShowOpen, ShowAll, ShowDone, CycleShowMode key.Binding
}

var taskKeys = taskKeyMap{
	Up:            bind(k("k", "up"), "↑/k", "subir"),
	Down:          bind(k("j", "down"), "↓/j", "bajar"),
	PageUp:        bind(k("pgup"), "re pág", "página arriba"),
	PageDown:      bind(k("pgdown"), "av pág", "página abajo"),
	Top:           bind(k("home"), "inicio", "primera"),
	Bottom:        bind(k("end"), "fin", "última"),
	New:           bind(k("n"), "n", "nueva"),
	Toggle:        bind(k(" "), "espacio", "hecha"),
	Status:        bind(k("s"), "s", "estado"),
	PrioUp:        bind(k("+", "="), "+", "subir prioridad"),
	PrioDown:      bind(k("-"), "-", "bajar prioridad"),
	Edit:          bind(k("e"), "e", "editar"),
	Open:          bind(k("enter"), "enter", "ver"),
	Delete:        bind(k("d"), "d", "borrar"),
	GoProject:     bind(k("g"), "g", "ir al proyecto"),
	Search:        bind(k("/"), "/", "buscar"),
	Filter:        bind(k("f"), "f", "filtrar"),
	Clear:         bind(k("esc"), "esc", "quitar filtros"),
	ShowOpen:      bind(k("o"), "o", "abiertas"),
	ShowAll:       bind(k("a"), "a", "todas"),
	ShowDone:      bind(k("h"), "h", "hechas"),
	CycleShowMode: bind(k("tab"), "tab", "alternar vista"),
}

func (t *Tasks) syncKeys() {
	has := t.selected() != nil
	for _, b := range []*key.Binding{&taskKeys.Toggle, &taskKeys.Status, &taskKeys.PrioUp, &taskKeys.PrioDown,
		&taskKeys.Edit, &taskKeys.Open, &taskKeys.Delete, &taskKeys.GoProject} {
		b.SetEnabled(has)
	}
	taskKeys.Clear.SetEnabled(t.filterLabel != "" || t.search != "")
}

func (t *Tasks) shortHelp() []key.Binding {
	t.syncKeys()
	m := taskKeys
	return []key.Binding{m.New, m.Toggle, m.Edit, m.Open, m.Search, m.Filter, m.Clear, globalKeys.Palette}
}

var taskHelp = helpGroup{"Tareas", []key.Binding{
	taskKeys.ShowOpen, taskKeys.ShowAll, taskKeys.ShowDone, taskKeys.CycleShowMode, taskKeys.New, taskKeys.Toggle,
	taskKeys.Status, taskKeys.PrioUp, taskKeys.PrioDown, taskKeys.Edit, taskKeys.Open, taskKeys.Delete,
	taskKeys.GoProject, taskKeys.Search, taskKeys.Filter, taskKeys.Clear,
}}

// ---------- proyectos ----------

type projectKeyMap struct {
	Up, Down, New, Edit, Status, AddPayment, Pay, Notes, Tasks, Delete key.Binding
}

var projectKeys = projectKeyMap{
	Up:         bind(k("k", "up"), "↑/k", "subir"),
	Down:       bind(k("j", "down"), "↓/j", "bajar"),
	New:        bind(k("n"), "n", "nuevo"),
	Edit:       bind(k("e"), "e", "editar"),
	Status:     bind(k("S"), "S", "estado"),
	AddPayment: bind(k("c"), "c", "cuota"),
	Pay:        bind(k("x"), "x", "pagar"),
	Notes:      bind(k("o"), "o", "notas"),
	Tasks:      bind(k("enter"), "enter", "tareas"),
	Delete:     bind(k("d"), "d", "borrar"),
}

func (p *Projects) syncKeys() {
	has := p.selectedProject() != nil
	m := &projectKeys
	for _, b := range []*key.Binding{&m.Edit, &m.Status, &m.AddPayment, &m.Notes, &m.Tasks, &m.Delete} {
		b.SetEnabled(has)
	}
	pending := false
	for _, pay := range p.payments {
		pending = pending || pay.PaidDate == nil
	}
	m.Pay.SetEnabled(has && pending)
}

func (p *Projects) shortHelp() []key.Binding {
	p.syncKeys()
	m := projectKeys
	return []key.Binding{m.New, m.Edit, m.Status, m.AddPayment, m.Pay, m.Tasks, m.Notes, globalKeys.Palette}
}

var projectHelp = helpGroup{"Proyectos", []key.Binding{
	projectKeys.New, projectKeys.Edit, projectKeys.Delete, projectKeys.Status, projectKeys.AddPayment,
	projectKeys.Pay, projectKeys.Tasks, projectKeys.Notes,
}}

// ---------- clientes ----------

type clientKeyMap struct {
	Up, Down, New, Edit, Delete, Tasks key.Binding
}

var clientKeys = clientKeyMap{
	Up:     bind(k("k", "up"), "↑/k", "subir"),
	Down:   bind(k("j", "down"), "↓/j", "bajar"),
	New:    bind(k("n"), "n", "nuevo"),
	Edit:   bind(k("e"), "e", "editar"),
	Delete: bind(k("d"), "d", "borrar"),
	Tasks:  bind(k("enter"), "enter", "tareas"),
}

func (c *Clients) shortHelp() []key.Binding {
	has := c.selectedClient() != nil
	m := &clientKeys
	for _, b := range []*key.Binding{&m.Edit, &m.Delete, &m.Tasks} {
		b.SetEnabled(has)
	}
	return []key.Binding{m.New, m.Edit, m.Delete, m.Tasks, globalKeys.Palette}
}

var clientHelp = helpGroup{"Clientes", []key.Binding{clientKeys.New, clientKeys.Edit, clientKeys.Delete, clientKeys.Tasks}}

// ---------- cobros ----------

type paymentKeyMap struct {
	Up, Down, Pay, Unpay key.Binding
}

var paymentKeys = paymentKeyMap{
	Up:    bind(k("k", "up"), "↑/k", "subir"),
	Down:  bind(k("j", "down"), "↓/j", "bajar"),
	Pay:   bind(k("x"), "x", "marcar pagada"),
	Unpay: bind(k("u"), "u", "deshacer pago"),
}

func (p *Payments) shortHelp() []key.Binding {
	pay := p.selected()
	paymentKeys.Pay.SetEnabled(pay != nil && pay.PaidDate == nil)
	paymentKeys.Unpay.SetEnabled(pay != nil && pay.PaidDate != nil)
	return []key.Binding{paymentKeys.Up, paymentKeys.Down, paymentKeys.Pay, paymentKeys.Unpay, globalKeys.Palette}
}

var paymentHelp = helpGroup{"Cobros", []key.Binding{paymentKeys.Pay, paymentKeys.Unpay}}

// ---------- notas ----------

type noteKeyMap struct {
	Up, Down, New, Edit, Read, Tag, Project, Search, GoProject, Delete, Clear key.Binding
}

var noteKeys = noteKeyMap{
	Up:        bind(k("k", "up"), "↑/k", "subir"),
	Down:      bind(k("j", "down"), "↓/j", "bajar"),
	New:       bind(k("n"), "n", "nueva"),
	Edit:      bind(k("e"), "e", "editar"),
	Read:      bind(k("enter"), "enter", "leer"),
	Tag:       bind(k("t"), "t", "etiqueta"),
	Project:   bind(k("f"), "f", "proyecto"),
	Search:    bind(k("/"), "/", "buscar"),
	GoProject: bind(k("g"), "g", "ir al proyecto"),
	Delete:    bind(k("d"), "d", "borrar"),
	Clear:     bind(k("esc"), "esc", "quitar filtros"),
}

func (n *Notes) syncKeys() {
	note := n.selected()
	m := &noteKeys
	for _, b := range []*key.Binding{&m.Edit, &m.Read, &m.Delete} {
		b.SetEnabled(note != nil)
	}
	m.GoProject.SetEnabled(note != nil && note.ProjectID != nil)
	m.Tag.SetEnabled(len(n.tags) > 0)
	m.Clear.SetEnabled(n.hasFilters())
}

func (n *Notes) shortHelp() []key.Binding {
	n.syncKeys()
	m := noteKeys
	return []key.Binding{m.New, m.Read, m.Edit, m.Tag, m.Project, m.Search, m.Clear, m.Delete}
}

var noteHelp = helpGroup{"Notas", []key.Binding{
	noteKeys.New, noteKeys.Edit, noteKeys.Read, noteKeys.Delete, noteKeys.Tag, noteKeys.Project,
	noteKeys.Search, noteKeys.GoProject, noteKeys.Clear,
}}

// ---------- calendario ----------

type calendarKeyMap struct {
	PrevDay, NextDay, PrevWeek, NextWeek, PrevMonth, NextMonth, Today, View key.Binding
	NextItem, PrevItem, New, Edit, Open, Delete, Toggle, Pay                key.Binding
}

var calendarKeys = calendarKeyMap{
	PrevDay:   bind(k("h", "left"), "←/h", "día anterior"),
	NextDay:   bind(k("l", "right"), "→/l", "día siguiente"),
	PrevWeek:  bind(k("k", "up"), "↑/k", "semana anterior"),
	NextWeek:  bind(k("j", "down"), "↓/j", "semana siguiente"),
	PrevMonth: bind(k("[", "pgup"), "[", "mes anterior"),
	NextMonth: bind(k("]", "pgdown"), "]", "mes siguiente"),
	Today:     bind(k("t"), "t", "hoy"),
	View:      bind(k("v"), "v", "mes/semana/día"),
	NextItem:  bind(k("tab"), "tab", "siguiente entrada"),
	PrevItem:  bind(k("shift+tab"), "shift+tab", "entrada anterior"),
	New:       bind(k("n"), "n", "evento"),
	Edit:      bind(k("e"), "e", "editar"),
	Open:      bind(k("enter"), "enter", "abrir"),
	Delete:    bind(k("d"), "d", "borrar"),
	Toggle:    bind(k(" "), "espacio", "hecha"),
	Pay:       bind(k("x"), "x", "pagada"),
}

func (c *Calendar) syncKeys() {
	it := c.selected()
	m := &calendarKeys
	isEvent := it != nil && it.Type == agenda.TypeEvent
	m.Edit.SetEnabled(isEvent)
	m.Delete.SetEnabled(isEvent)
	m.Open.SetEnabled(it != nil)
	m.Toggle.SetEnabled(it != nil && it.Type == agenda.TypeTask)
	m.Pay.SetEnabled(it != nil && it.Type == agenda.TypePayment)
	many := len(c.byDate[c.date]) > 1
	m.NextItem.SetEnabled(many)
	m.PrevItem.SetEnabled(many)
	m.Open.SetHelp("enter", "abrir")
	if it != nil {
		m.Open.SetHelp("enter", "ver "+it.Label())
	}
}

func (c *Calendar) shortHelp() []key.Binding {
	c.syncKeys()
	m := calendarKeys
	return []key.Binding{m.New, m.Open, m.Edit, m.Toggle, m.Pay, m.Delete, m.NextItem, m.View, m.Today, globalKeys.Palette}
}

var calendarHelp = helpGroup{"Calendario", []key.Binding{
	calendarKeys.PrevDay, calendarKeys.NextDay, calendarKeys.PrevWeek, calendarKeys.NextWeek,
	calendarKeys.PrevMonth, calendarKeys.NextMonth, calendarKeys.Today, calendarKeys.View,
	calendarKeys.NextItem, calendarKeys.New, calendarKeys.Edit, calendarKeys.Delete,
	bind(nil, "enter", "abrir tarea, cobro o proyecto"), calendarKeys.Toggle, calendarKeys.Pay,
}}

var globalHelp = helpGroup{"Global", []key.Binding{
	globalKeys.Tabs, globalKeys.Palette, globalKeys.Help, globalKeys.Theme,
	bind(nil, "esc", "cerrar ventanas"), globalKeys.Quit,
}}

// ---------- barra de estado ----------

// newBarHelp es el help.Model de la barra de estado: teclas resaltadas y
// descripción atenuada sobre el fondo de la barra; si no caben, recorta con "…".
func newBarHelp() help.Model {
	h := help.New()
	bg := lipgloss.NewStyle().Background(colorBarBg)
	h.ShortSeparator = "  "
	h.Ellipsis = " …"
	h.Styles.ShortKey = keyStyle.Background(colorBarBg)
	h.Styles.ShortDesc = bg.Foreground(colorMuted)
	h.Styles.ShortSeparator = bg
	h.Styles.Ellipsis = bg.Foreground(colorMuted)
	return h
}

func (a *App) activeShortHelp() []key.Binding {
	switch a.tab {
	case tabProjects:
		return a.projects.shortHelp()
	case tabClients:
		return a.clients.shortHelp()
	case tabPayments:
		return a.payments.shortHelp()
	case tabNotes:
		return a.notes.shortHelp()
	case tabCalendar:
		return a.calendar.shortHelp()
	default:
		return a.tasks.shortHelp()
	}
}
