package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"tunapm/internal/store"
)

type keyMap struct {
	Quit    key.Binding
	Palette key.Binding
	Help    key.Binding
	Tabs    key.Binding
}

var keys = keyMap{
	Quit:    key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "salir")),
	Palette: key.NewBinding(key.WithKeys("ctrl+k", ":"), key.WithHelp("ctrl+k", "paleta")),
	Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "ayuda")),
	Tabs:    key.NewBinding(key.WithKeys("1", "2", "3", "4", "5"), key.WithHelp("1-5", "vistas")),
}

const (
	tabTasks = iota
	tabProjects
	tabClients
	tabPayments
	tabNotes
)

// Syncer lo implementa la caché local (internal/cache): avisa de cambios hechos por la
// sincronización en segundo plano y del estado de la conexión.
type Syncer interface {
	Changes() <-chan struct{}
	SyncStatus() (state string, pending int, detail string)
	TakeNotices() []string
}

type syncMsg struct{}

func waitSync(s Syncer) tea.Cmd {
	return func() tea.Msg {
		<-s.Changes()
		return syncMsg{}
	}
}

type App struct {
	st            store.Repo
	syncer        Syncer
	width, height int
	tab           int

	tasks    *Tasks
	projects *Projects
	clients  *Clients
	payments *Payments
	notes    *Notes

	palette *Palette
	picker  *Picker
	confirm *Confirm
	detail  *Detail
	help    bool

	form     *huh.Form
	formSave func() error
	formInit tea.Cmd

	toast    string
	toastErr bool
}

func NewApp(st store.Repo) *App {
	loadTheme()
	a := &App{st: st, width: 100, height: 32}
	a.tasks = newTasks(st)
	a.projects = newProjects(st)
	a.clients = newClients(st)
	a.payments = newPayments(st)
	a.notes = newNotes(st)
	a.syncer, _ = st.(Syncer)
	a.layout()
	return a
}

func (a *App) Init() tea.Cmd {
	if a.syncer != nil {
		return waitSync(a.syncer)
	}
	return nil
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.layout()
	case syncMsg:
		// La sincronización cambió el espejo o el estado: se relee todo (es local, instantáneo).
		a.reloadAll()
		for _, n := range a.syncer.TakeNotices() {
			a.errf("%s", n)
		}
		return a, waitSync(a.syncer)
	case tea.KeyMsg:
		cmd := a.handleKey(msg)
		if a.formInit != nil {
			init := a.formInit
			a.formInit = nil
			return a, tea.Batch(cmd, init)
		}
		return a, cmd
	default:
		if a.form != nil {
			return a, a.updateForm(msg)
		}
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) tea.Cmd {
	a.toast = ""

	if a.form != nil {
		return a.updateForm(msg)
	}
	if a.confirm != nil {
		switch msg.String() {
		case "y", "Y", "enter":
			yes := a.confirm.onYes
			a.confirm = nil
			yes(a)
		case "n", "N", "esc":
			a.confirm = nil
		}
		return nil
	}
	if a.picker != nil {
		return a.picker.handleKey(a, msg)
	}
	if a.palette != nil {
		return a.palette.handleKey(a, msg)
	}
	if a.detail != nil {
		switch msg.String() {
		case "esc", "enter", "q":
			a.detail = nil
		}
		return nil
	}
	if a.help {
		switch msg.String() {
		case "esc", "?", "q":
			a.help = false
		}
		return nil
	}

	// Mientras se escribe en la búsqueda, las letras no son atajos globales.
	if a.tab == tabTasks && a.tasks.searching && !key.Matches(msg, keys.Quit) {
		a.tasks.handleKey(a, msg)
		return nil
	}
	if a.tab == tabNotes && a.notes.searching && !key.Matches(msg, keys.Quit) {
		a.notes.handleKey(a, msg)
		return nil
	}

	switch {
	case key.Matches(msg, keys.Quit):
		return tea.Quit
	case key.Matches(msg, keys.Palette):
		a.openPalette("")
	case key.Matches(msg, keys.Help):
		a.help = true
	case msg.String() == "T":
		a.openThemePicker()
	case msg.String() == "1":
		a.setTab(tabTasks)
	case msg.String() == "2":
		a.setTab(tabProjects)
	case msg.String() == "3":
		a.setTab(tabClients)
	case msg.String() == "4":
		a.setTab(tabPayments)
	case msg.String() == "5":
		a.setTab(tabNotes)
	default:
		switch a.tab {
		case tabTasks:
			a.tasks.handleKey(a, msg)
		case tabProjects:
			a.projects.handleKey(a, msg)
		case tabClients:
			a.clients.handleKey(a, msg)
		case tabPayments:
			a.payments.handleKey(a, msg)
		case tabNotes:
			a.notes.handleKey(a, msg)
		}
	}
	return nil
}

func (a *App) setTab(tab int) {
	if tab == a.tab {
		return
	}
	a.tab = tab
	a.layout()
}

func (a *App) layout() {
	bodyH := a.height - 2
	if bodyH < 3 {
		bodyH = 3
	}
	a.tasks.layout(a.width, bodyH)
	a.projects.layout(a.width, bodyH)
	a.clients.layout(a.width, bodyH)
	a.payments.layout(a.width, bodyH)
	a.notes.layout(a.width, bodyH)
	if a.palette != nil {
		a.palette.layout(a.width, bodyH)
	}
	if a.picker != nil {
		a.picker.layout(a.width, bodyH)
	}
}

func (a *App) reloadAll() {
	a.tasks.reload()
	a.projects.reload()
	a.clients.reload()
	a.payments.reload()
	a.notes.reload()
}

func (a *App) View() string {
	if a.width <= 0 || a.height <= 0 {
		return ""
	}
	if a.width < minWidth || a.height < minHeight {
		msg := titleStyle.Render("Ventana muy pequeña") + "\n" +
			mutedStyle.Render(fmt.Sprintf("mínimo %d×%d", minWidth, minHeight))
		return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, msg)
	}
	bodyW, bodyH := a.width, a.height-2
	if bodyH < 1 {
		bodyH = 1
	}

	var body string
	switch {
	case a.form != nil:
		body = lipgloss.Place(bodyW, bodyH, lipgloss.Center, lipgloss.Center, modalStyle.Render(a.form.View()))
	case a.confirm != nil:
		body = lipgloss.Place(bodyW, bodyH, lipgloss.Center, lipgloss.Center, a.confirm.view())
	case a.picker != nil:
		body = lipgloss.Place(bodyW, bodyH, lipgloss.Center, lipgloss.Center, a.picker.view())
	case a.palette != nil:
		body = lipgloss.Place(bodyW, bodyH, lipgloss.Center, lipgloss.Top, a.palette.view())
	case a.detail != nil:
		body = lipgloss.Place(bodyW, bodyH, lipgloss.Center, lipgloss.Center, a.detail.view(bodyW, bodyH))
	case a.help:
		body = lipgloss.Place(bodyW, bodyH, lipgloss.Center, lipgloss.Center, a.helpView())
	default:
		body = a.activeView()
	}

	return a.tabsView() + "\n" + body + "\n" + a.statusBar()
}

const minWidth, minHeight = 50, 10

func (a *App) tabsView() string {
	labels := []string{"1 Tareas", "2 Proyectos", "3 Clientes", "4 Cobros", "5 Notas"}
	var b strings.Builder
	for i, label := range labels {
		style := tabInactiveStyle
		if i == a.tab {
			style = tabActiveStyle
		}
		b.WriteString(style.Render(label))
	}
	left := b.String()
	right := a.sourceBadge() + appNameStyle.Render("tunapm")
	gap := max(a.width-lipglossWidth(left)-lipglossWidth(right), 0)
	return left + lipgloss.NewStyle().Background(colorBarBg).Render(strings.Repeat(" ", gap)) + right
}

// Source describe de dónde salen los datos: lo fija main. Remote=true es el API.
var (
	Source string
	Remote bool
)

// sourceBadge muestra en la barra superior de dónde salen los datos y el estado de la sincronización.
func (a *App) sourceBadge() string {
	if Source == "" {
		return ""
	}
	label, color := "● LOCAL", colorYellow
	if Remote {
		label, color = "● API "+Source, colorGreen
		if a.syncer != nil {
			state, pending, _ := a.syncer.SyncStatus()
			switch state {
			case "en línea":
				if pending > 0 {
					label += fmt.Sprintf(" · %d por enviar", pending)
				}
			case "sin conexión":
				label, color = "● SIN CONEXIÓN", colorYellow
				if pending > 0 {
					label += fmt.Sprintf(" · %d pendientes", pending)
				}
			case "error":
				label, color = "● ERROR API", colorRed
				if pending > 0 {
					label += fmt.Sprintf(" · %d pendientes", pending)
				}
			default:
				label, color = "● conectando…", colorMuted
			}
		}
	}
	return lipgloss.NewStyle().Foreground(color).Background(colorBarBg).Bold(true).Render(label) +
		lipgloss.NewStyle().Background(colorBarBg).Render(" ")
}

func (a *App) activeView() string {
	switch a.tab {
	case tabProjects:
		return a.projects.view()
	case tabClients:
		return a.clients.view()
	case tabPayments:
		return a.payments.view()
	case tabNotes:
		return a.notes.view()
	default:
		return a.tasks.view()
	}
}

func (a *App) statusBar() string {
	bg := lipgloss.NewStyle().Background(colorBarBg)
	inner := a.width - 2
	left := a.activeInfo()
	var right string
	if a.toast != "" {
		right = toastColor(a.toastErr).Background(colorBarBg).Render(a.toast)
	} else {
		// Se muestran tantos atajos como quepan; ? ayuda siempre al final.
		hints := strings.Split(a.activeHints(), " · ")
		for n := len(hints); n >= 0; n-- {
			right = barHints(append(append([]string{}, hints[:n]...), "? ayuda"))
			if lipglossWidth(left)+lipglossWidth(right)+2 <= inner {
				break
			}
		}
	}
	left = truncate(left, max(inner-lipglossWidth(right)-2, 0))
	gap := max(inner-lipglossWidth(left)-lipglossWidth(right), 0)
	return bg.Render(" ") + bg.Foreground(colorText).Render(left) + bg.Render(strings.Repeat(" ", gap)) + right + bg.Render(" ")
}

// barHints pinta los atajos sobre el fondo de la barra de estado.
func barHints(hints []string) string {
	bg := lipgloss.NewStyle().Background(colorBarBg)
	var parts []string
	for _, hint := range hints {
		k, desc, _ := strings.Cut(hint, " ")
		parts = append(parts, keyStyle.Background(colorBarBg).Render(k)+bg.Foreground(colorMuted).Render(" "+desc))
	}
	return strings.Join(parts, bg.Render("  "))
}

func (a *App) activeHints() string {
	switch a.tab {
	case tabProjects:
		return a.projects.hints()
	case tabClients:
		return a.clients.hints()
	case tabPayments:
		return a.payments.hints()
	case tabNotes:
		return a.notes.hints()
	default:
		return a.tasks.hints()
	}
}

func (a *App) activeInfo() string {
	switch a.tab {
	case tabProjects:
		return a.projects.info()
	case tabClients:
		return a.clients.info()
	case tabPayments:
		return a.payments.info()
	case tabNotes:
		return a.notes.info()
	default:
		return a.tasks.info()
	}
}

func (a *App) openPalette(prefill string) {
	a.palette = newPalette(a.st, prefill)
	a.palette.layout(a.width, a.height-2)
}

func (a *App) toastf(format string, args ...any) {
	a.toast, a.toastErr = fmt.Sprintf(format, args...), false
}

func (a *App) errf(format string, args ...any) {
	a.toast, a.toastErr = fmt.Sprintf(format, args...), true
}

func (a *App) confirmAction(question string, yes func(*App)) {
	a.confirm = &Confirm{question: question, onYes: yes}
}

// openForm muestra un formulario huh y guarda al completarlo.
func (a *App) openForm(form *huh.Form, save func() error) {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("esc", "ctrl+c"))
	for _, b := range []*key.Binding{&km.Input.Next, &km.Select.Next, &km.Text.Next} {
		b.SetHelp("enter", "siguiente")
	}
	for _, b := range []*key.Binding{&km.Input.Prev, &km.Select.Prev, &km.Text.Prev} {
		b.SetHelp("shift+tab", "anterior")
	}
	for _, b := range []*key.Binding{&km.Input.Submit, &km.Select.Submit, &km.Text.Submit} {
		b.SetHelp("enter", "guardar")
	}
	km.Select.Up.SetHelp("↑", "subir")
	km.Select.Down.SetHelp("↓", "bajar")
	km.Select.Filter.SetHelp("/", "filtrar")
	km.Text.NewLine.SetHelp("alt+enter", "nueva línea")
	km.Text.Editor.SetHelp("ctrl+e", "abrir editor")
	form.WithKeyMap(km).WithShowHelp(true).WithTheme(formTheme())
	if a.width > 20 {
		form.WithWidth(min(a.width-8, 72))
	}
	if a.height > 12 {
		form.WithHeight(a.height - 8)
	}
	a.form, a.formSave = form, save
	a.formInit = form.Init()
}

func (a *App) updateForm(msg tea.Msg) tea.Cmd {
	model, cmd := a.form.Update(msg)
	a.form = model.(*huh.Form)
	switch a.form.State {
	case huh.StateCompleted:
		save := a.formSave
		a.form, a.formSave = nil, nil
		if save != nil {
			if err := save(); err != nil {
				a.errf("%v", err)
			} else {
				a.reloadAll()
			}
		}
	case huh.StateAborted:
		a.form, a.formSave = nil, nil
	}
	return cmd
}

// ---- overlays simples ----

type Confirm struct {
	question string
	onYes    func(*App)
}

func (c *Confirm) view() string {
	return modalStyle.Render(
		titleStyle.Render(c.question) + "\n\n" +
			renderHints("y confirmar · n cancelar"))
}

type Detail struct {
	title string
	lines []string
}

func (d *Detail) view(width, height int) string {
	maxWidth := max(min(width-8, 90), 24)
	body := titleStyle.Render(d.title) + "\n\n"
	for _, line := range d.lines {
		body += wrapLine(line, maxWidth-4) + "\n"
	}
	body += "\n" + renderHints("esc cerrar")
	if height > 6 && lipgloss.Height(body)+4 > height {
		// notas largas: se recorta para no romper el layout
		lines := strings.Split(body, "\n")
		limit := max(height-6, 3)
		if len(lines) > limit {
			lines = append(lines[:limit-1], mutedStyle.Render("…"))
		}
		body = strings.Join(lines, "\n")
	}
	return modalStyle.Width(maxWidth).Render(body)
}

func wrapLine(line string, width int) string {
	words := strings.Fields(line)
	if len(words) == 0 {
		return ""
	}
	var out strings.Builder
	current := 0
	for _, w := range words {
		if current > 0 && current+1+lipglossWidth(w) > width {
			out.WriteString("\n")
			current = 0
		}
		if current > 0 {
			out.WriteString(" ")
			current++
		}
		out.WriteString(w)
		current += lipglossWidth(w)
	}
	return out.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
