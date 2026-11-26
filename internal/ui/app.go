package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"tunapm/internal/store"
)

const (
	tabTasks = iota
	tabProjects
	tabClients
	tabPayments
	tabNotes
	tabCalendar
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
	calendar *Calendar

	palette *Palette
	picker  *Picker
	detail  *Reader // ventana de lectura (nota, tarea, evento) o la ayuda

	barHelp  help.Model    // atajos de la barra de estado
	spin     spinner.Model // sincronización en curso (modo remoto)
	spinning bool

	form     *huh.Form
	formSave func() error
	formInit tea.Cmd

	toast    string
	toastErr bool
}

func NewApp(st store.Repo) *App {
	loadTheme()
	a := &App{st: st, width: 100, height: 32, barHelp: newBarHelp()}
	a.spin = spinner.New(spinner.WithSpinner(spinner.MiniDot))
	a.tasks = newTasks(st)
	a.projects = newProjects(st)
	a.clients = newClients(st)
	a.payments = newPayments(st)
	a.notes = newNotes(st)
	a.calendar = newCalendar(st)
	a.syncer, _ = st.(Syncer)
	a.restyle()
	a.layout()
	return a
}

func (a *App) Init() tea.Cmd {
	if a.syncer != nil {
		return tea.Batch(waitSync(a.syncer), a.startSpinner())
	}
	return nil
}

// syncBusy: conectando o enviando cambios pendientes.
func (a *App) syncBusy() bool {
	if a.syncer == nil {
		return false
	}
	state, pending, _ := a.syncer.SyncStatus()
	return state == "conectando" || (state == "en línea" && pending > 0)
}

// startSpinner arranca el spinner solo si hay trabajo; se detiene solo al terminar.
func (a *App) startSpinner() tea.Cmd {
	if a.spinning || !a.syncBusy() {
		return nil
	}
	a.spinning = true
	return a.spin.Tick
}

// restyle aplica el tema actual a los componentes que guardan estilos propios.
func (a *App) restyle() {
	a.barHelp = newBarHelp()
	a.spin.Style = lipgloss.NewStyle().Foreground(colorAccent).Background(colorBarBg)
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
		return a, tea.Batch(waitSync(a.syncer), a.startSpinner())
	case spinner.TickMsg:
		if !a.syncBusy() {
			a.spinning = false
			return a, nil
		}
		var cmd tea.Cmd
		a.spin, cmd = a.spin.Update(msg)
		return a, cmd
	case tea.KeyMsg:
		// Una escritura local deja cambios por enviar: el spinner lo muestra hasta que lleguen.
		cmd := tea.Batch(a.handleKey(msg), a.startSpinner())
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
	if a.picker != nil {
		return a.picker.handleKey(a, msg)
	}
	if a.palette != nil {
		return a.palette.handleKey(a, msg)
	}
	if a.detail != nil {
		if a.detail.handleKey(msg) {
			a.detail = nil
		}
		return nil
	}

	// Mientras se escribe en la búsqueda, las letras no son atajos globales.
	if a.tab == tabTasks && a.tasks.searching && !key.Matches(msg, globalKeys.Quit) {
		a.tasks.handleKey(a, msg)
		return nil
	}
	if a.tab == tabNotes && a.notes.searching && !key.Matches(msg, globalKeys.Quit) {
		a.notes.handleKey(a, msg)
		return nil
	}

	switch {
	case key.Matches(msg, globalKeys.Quit):
		return tea.Quit
	case key.Matches(msg, globalKeys.Palette):
		a.openPalette("")
	case key.Matches(msg, globalKeys.Help):
		a.detail = a.helpReader()
	case key.Matches(msg, globalKeys.Theme):
		a.openThemePicker()
	case key.Matches(msg, globalKeys.Tabs):
		a.setTab(int(msg.String()[0] - '1'))
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
		case tabCalendar:
			a.calendar.handleKey(a, msg)
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
	a.calendar.layout(a.width, bodyH)
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
	a.calendar.reload()
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
	case a.picker != nil:
		body = lipgloss.Place(bodyW, bodyH, lipgloss.Center, lipgloss.Center, a.picker.view())
	case a.palette != nil:
		body = lipgloss.Place(bodyW, bodyH, lipgloss.Center, lipgloss.Top, a.palette.view())
	case a.detail != nil:
		body = lipgloss.Place(bodyW, bodyH, lipgloss.Center, lipgloss.Center, a.detail.view(bodyW, bodyH))
	default:
		body = a.activeView()
	}

	return a.tabsView() + "\n" + body + "\n" + a.statusBar()
}

const minWidth, minHeight = 50, 10

func (a *App) tabsView() string {
	labels := []string{"1 Tareas", "2 Proyectos", "3 Clientes", "4 Cobros", "5 Notas", "6 Calendario"}
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
	if a.spinning {
		label = a.spin.View() + lipgloss.NewStyle().Foreground(color).Background(colorBarBg).Bold(true).Render(strings.TrimPrefix(label, "●"))
		return label + lipgloss.NewStyle().Background(colorBarBg).Render(" ")
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
	case tabCalendar:
		return a.calendar.view()
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
		right = toastColor(a.toastErr).Background(colorBarBg).Render(truncate(a.toast, inner))
	} else {
		// "? ayuda" siempre queda; el resto de los atajos se recorta si no cabe.
		helpKey := a.barHelp.ShortHelpView([]key.Binding{globalKeys.Help})
		room := inner - lipglossWidth(helpKey) - min(lipglossWidth(left), inner/3) - 4
		a.barHelp.Width = max(room, 0)
		right = a.barHelp.ShortHelpView(a.activeShortHelp())
		if right != "" {
			right += bg.Render("  ")
		}
		right += helpKey
	}
	left = truncate(left, max(inner-lipglossWidth(right)-2, 0))
	gap := max(inner-lipglossWidth(left)-lipglossWidth(right), 0)
	return bg.Render(" ") + bg.Foreground(colorText).Render(left) + bg.Render(strings.Repeat(" ", gap)) + right + bg.Render(" ")
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
	case tabCalendar:
		return a.calendar.info()
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

// confirmAction pregunta antes de algo destructivo. "Cancelar" viene marcado por
// defecto; y / n responden directo y ←→ cambian de opción.
func (a *App) confirmAction(question string, yes func(*App)) {
	confirmed := false
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title(question).Affirmative("Sí").Negative("Cancelar").Value(&confirmed),
	))
	a.openForm(form, func() error {
		if confirmed {
			yes(a)
		}
		return nil
	})
	a.form.WithShowHelp(false)
	if a.width > 20 {
		a.form.WithWidth(min(max(lipglossWidth(question)+4, 36), a.width-8))
	}
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
	km.Select.Left.SetHelp("←", "anterior")
	km.Select.Right.SetHelp("→", "siguiente")
	km.Text.NewLine.SetHelp("alt+enter", "nueva línea")
	km.Text.Editor.SetHelp("ctrl+e", "abrir editor")
	km.MultiSelect.Toggle.SetHelp("espacio", "marcar")
	km.MultiSelect.Filter.SetHelp("/", "filtrar")
	km.MultiSelect.Next.SetHelp("enter", "siguiente")
	km.MultiSelect.Submit.SetHelp("enter", "guardar")
	km.Confirm.Toggle.SetHelp("←/→", "cambiar")
	km.Confirm.Submit.SetHelp("enter", "aceptar")
	form.WithKeyMap(km).WithShowHelp(true).WithTheme(formTheme())
	if a.width > 20 {
		form.WithWidth(min(a.width-8, 72))
	}
	// Solo se limita la altura si el formulario no cabe; los cortos quedan compactos.
	if limit := a.height - 8; a.height > 12 && lipgloss.Height(form.View()) > limit {
		form.WithHeight(limit)
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
