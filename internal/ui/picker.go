package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tunapm/internal/store"
)

type pickItem struct {
	Kind   string // project, client, status, none
	ID     int64
	Label  string
	Detail string
	Value  string
}

type Picker struct {
	title    string
	items    []pickItem
	filtered []pickItem
	input    textinput.Model
	cursor   int
	width    int
	height   int
	onSelect func(*App, pickItem)
	onMove   func(*App, pickItem) // vista previa al mover el cursor
	onCancel func(*App)
}

func newPicker(title string, items []pickItem, onSelect func(*App, pickItem)) *Picker {
	ti := textinput.New()
	ti.Placeholder = "buscar…"
	ti.Focus()
	p := &Picker{title: title, items: items, input: ti, onSelect: onSelect, width: 60, height: 20}
	p.filter()
	return p
}

func (p *Picker) layout(width, height int) {
	p.width, p.height = width, height
	p.input.Width = max(min(width-12, 66), 10)
}

func (p *Picker) listHeight() int {
	return max(min(p.height-10, 12), 3)
}

func (p *Picker) filter() {
	labels := make([]string, len(p.items))
	for i, item := range p.items {
		labels[i] = item.Label + " " + item.Detail
	}
	idx := fuzzyIndexes(p.input.Value(), labels)
	p.filtered = make([]pickItem, len(idx))
	for i, j := range idx {
		p.filtered[i] = p.items[j]
	}
	if p.cursor >= len(p.filtered) {
		p.cursor = max(len(p.filtered)-1, 0)
	}
}

func (p *Picker) handleKey(a *App, msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		a.picker = nil
		if p.onCancel != nil {
			p.onCancel(a)
		}
	case "up", "ctrl+p":
		if p.cursor > 0 {
			p.cursor--
			p.moved(a)
		}
	case "down", "ctrl+n":
		if p.cursor < len(p.filtered)-1 {
			p.cursor++
			p.moved(a)
		}
	case "enter":
		if len(p.filtered) == 0 {
			return nil
		}
		item := p.filtered[p.cursor]
		a.picker = nil
		if p.onSelect != nil {
			p.onSelect(a, item)
		}
	default:
		p.input, _ = p.input.Update(msg)
		p.filter()
		p.moved(a)
	}
	return nil
}

func (p *Picker) moved(a *App) {
	if p.onMove != nil && p.cursor < len(p.filtered) {
		p.onMove(a, p.filtered[p.cursor])
	}
}

func (p *Picker) view() string {
	contentWidth := max(min(p.width-12, 66), 20)
	var lines []string
	lines = append(lines, titleStyle.Render(p.title), p.input.View(), "")

	start := 0
	if p.cursor >= p.listHeight() {
		start = p.cursor - p.listHeight() + 1
	}
	for i := start; i < len(p.filtered) && i < start+p.listHeight(); i++ {
		item := p.filtered[i]
		line := padRight(" "+truncate(item.Label, contentWidth-12), contentWidth-10) +
			mutedStyle.Render(truncate(item.Detail, 10))
		if i == p.cursor {
			lines = append(lines, selectedStyle.Render(line))
		} else {
			lines = append(lines, line)
		}
	}
	if len(p.filtered) == 0 {
		lines = append(lines, mutedStyle.Render("  sin coincidencias"))
	}
	lines = append(lines, "", renderHints("enter elegir · esc cerrar"))
	return modalStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// openFilterPicker filtra la vista de tareas por proyecto o cliente.
func (a *App) openFilterPicker() {
	items := []pickItem{{Kind: "none", Label: "Todas las tareas", Detail: "sin filtro"}}
	for _, p := range a.projects.projects {
		items = append(items, pickItem{Kind: "project", ID: p.ID, Label: p.Name, Detail: p.ClientName})
	}
	for _, c := range a.clients.clients {
		items = append(items, pickItem{Kind: "client", ID: c.ID, Label: c.Name, Detail: c.Company})
	}
	a.picker = newPicker("Filtrar tareas por proyecto o cliente", items, func(a *App, item pickItem) {
		a.tasks.mode = store.FilterOpen
		switch item.Kind {
		case "none":
			a.tasks.projectFilter, a.tasks.clientFilter, a.tasks.filterLabel = nil, nil, ""
		case "project":
			id := item.ID
			a.tasks.projectFilter, a.tasks.clientFilter = &id, nil
			a.tasks.filterLabel = "proyecto: " + item.Label
		case "client":
			id := item.ID
			a.tasks.projectFilter, a.tasks.clientFilter = nil, &id
			a.tasks.filterLabel = "cliente: " + item.Label
		}
		a.tasks.reload()
		a.setTab(tabTasks)
	})
	a.picker.layout(a.width, a.height-2)
}

// openProjectStatusPicker cambia el estado del proyecto dado.
func (a *App) openProjectStatusPicker(projectID int64) {
	var items []pickItem
	for _, status := range store.ProjectStatuses {
		items = append(items, pickItem{Kind: "status", Value: status, Label: projectStatusLabels[status]})
	}
	a.picker = newPicker("Nuevo estado del proyecto", items, func(a *App, item pickItem) {
		project, err := a.st.GetProject(projectID)
		if err != nil {
			a.errf("No se encontró el proyecto")
			return
		}
		project.Status = item.Value
		if err := a.st.UpdateProject(&project); err != nil {
			a.errf("%v", err)
			return
		}
		a.projects.reload()
		a.toastf("Estado: %s", projectStatusLabels[project.Status])
	})
	a.picker.layout(a.width, a.height-2)
}

// openProjectPicker deja elegir un proyecto para una acción posterior.
func (a *App) openProjectPicker(title string, onSelect func(*App, store.Project)) {
	items := make([]pickItem, 0, len(a.projects.projects))
	for _, p := range a.projects.projects {
		items = append(items, pickItem{Kind: "project", ID: p.ID, Label: p.Name, Detail: p.ClientName})
	}
	a.picker = newPicker(title, items, func(a *App, item pickItem) {
		project, err := a.st.GetProject(item.ID)
		if err != nil {
			a.errf("No se encontró el proyecto")
			return
		}
		onSelect(a, project)
	})
	a.picker.layout(a.width, a.height-2)
}
