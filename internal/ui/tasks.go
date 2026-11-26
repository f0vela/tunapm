package ui

import (
	"charm.land/bubbles/v2/key"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"tunapm/internal/dates"
	"tunapm/internal/store"
)

type Tasks struct {
	st            store.Repo
	width, height int
	mode          string
	search        string
	searching     bool
	searchInput   textinput.Model
	projectFilter *int64
	clientFilter  *int64
	filterLabel   string
	tasks         []store.Task
	cursor        int
	selectedID    int64
}

func newTasks(st store.Repo) *Tasks {
	ti := newInput("/ ", "buscar por título, notas, proyecto o cliente…")
	return &Tasks{st: st, width: 100, height: 20, mode: store.FilterOpen, searchInput: ti}
}

func (t *Tasks) layout(width, height int) {
	t.width, t.height = width, height
	t.searchInput.SetWidth(max(width-30, 10))
	t.reload()
}

func (t *Tasks) reload() {
	prevID := t.selectedID
	if prevID == 0 && t.cursor < len(t.tasks) {
		prevID = t.tasks[t.cursor].ID
	}
	tasks, err := t.st.ListTasks(store.TaskFilter{
		Mode:      t.mode,
		ProjectID: t.projectFilter,
		ClientID:  t.clientFilter,
		Search:    t.search,
	})
	if err != nil {
		t.tasks = nil
		return
	}
	t.tasks = tasks

	prevCursor := t.cursor
	t.cursor = 0
	if prevID != 0 {
		for i, task := range tasks {
			if task.ID == prevID {
				t.cursor = i
				break
			}
		}
	} else {
		t.cursor = prevCursor
	}
	if len(tasks) == 0 {
		t.cursor = 0
	} else if t.cursor >= len(tasks) {
		t.cursor = len(tasks) - 1
	}
	if len(tasks) > 0 {
		t.selectedID = tasks[t.cursor].ID
	} else {
		t.selectedID = 0
	}
}

func (t *Tasks) selected() *store.Task {
	if t.cursor < 0 || t.cursor >= len(t.tasks) {
		return nil
	}
	return &t.tasks[t.cursor]
}

func (t *Tasks) listHeight() int {
	// Dos líneas fijas: filtros y encabezado de columnas.
	return max(t.height-2, 1)
}

func (t *Tasks) view() string {
	var header string
	if t.searching {
		header = spread(t.searchInput.View(), mutedStyle.Render("enter listo · esc limpiar"), t.width)
	} else {
		modes := []struct{ id, label string }{
			{store.FilterOpen, "o Abiertas"},
			{store.FilterAll, "a Todas"},
			{store.FilterDone, "h Hechas"},
		}
		var b strings.Builder
		for i, m := range modes {
			if i > 0 {
				b.WriteString("  ")
			}
			k, name, _ := strings.Cut(m.label, " ")
			if t.mode == m.id {
				b.WriteString(keyStyle.Foreground(colorAccent).Render(k) + " " + titleStyle.Underline(true).Render(name))
			} else {
				b.WriteString(keyStyle.Render(k) + " " + mutedStyle.Render(name))
			}
		}
		left := b.String()
		if t.filterLabel != "" {
			left += mutedStyle.Render("  filtro: ") + titleStyle.Render(t.filterLabel)
		}
		header = spread(" "+left, mutedStyle.Render(plural(len(t.tasks), "tarea", "tareas")+" "), t.width)
	}

	columns := t.columns()
	var tableLines []string
	if len(t.tasks) == 0 {
		tableLines = append(tableLines, "", mutedStyle.Render(t.emptyHint()))
	} else {
		start := t.cursor - (t.listHeight()-1)/2
		if start < 0 {
			start = 0
		}
		if start > len(t.tasks)-t.listHeight() {
			start = max(len(t.tasks)-t.listHeight(), 0)
		}
		end := min(start+t.listHeight(), len(t.tasks))
		for i := start; i < end; i++ {
			tableLines = append(tableLines, t.rowLine(t.tasks[i], columns))
		}
	}

	head := t.headerLine(columns)
	for len(tableLines) < t.listHeight() {
		tableLines = append(tableLines, "")
	}
	return header + "\n" + head + "\n" + strings.Join(tableLines[:t.listHeight()], "\n")
}

type taskColumns struct {
	title   int
	project int
	client  int
}

func (t *Tasks) columns() taskColumns {
	showProject := t.width >= 72
	showClient := t.width >= 96
	fixed := 2 + 1 + 1 + 9 + 1 + 9 // marcador, estado, prioridad, vence + separadores
	if showProject {
		fixed += 20 + 1
	}
	if showClient {
		fixed += 16 + 1
	}
	cols := taskColumns{title: max(t.width-fixed-2, 12)}
	if showProject {
		cols.project = 20
	}
	if showClient {
		cols.client = 16
	}
	return cols
}

func (t *Tasks) headerLine(cols taskColumns) string {
	line := "    " + padRight("PRIORIDAD", 9) + " " + padRight("TAREA", cols.title)
	if cols.project > 0 {
		line += " " + padRight("PROYECTO", cols.project)
	}
	if cols.client > 0 {
		line += " " + padRight("CLIENTE", cols.client)
	}
	line += " " + padLeft("VENCE", 9)
	return headStyle.Render(truncate(line, t.width))
}

func (t *Tasks) rowLine(task store.Task, cols taskColumns) string {
	today := dates.Today()
	// Las hechas se atenúan para que lo pendiente destaque en la vista "Todas".
	done := task.Status == store.TaskHecho
	title, prio := truncate(task.Title, cols.title), priorityText(task.Priority)
	if done {
		title, prio = mutedStyle.Render(title), mutedStyle.Render(priorityLabels[clampPriority(task.Priority)])
	}
	line := cursorMark(task.ID == t.selectedID) + statusIcon(task.Status) + " " + padRight(prio, 9) + " " +
		padRight(title, cols.title)
	if cols.project > 0 {
		line += " " + mutedStyle.Render(padRight(truncate(task.ProjectName, cols.project), cols.project))
	}
	if cols.client > 0 {
		line += " " + mutedStyle.Render(padRight(truncate(task.ClientName, cols.client), cols.client))
	}
	due := mutedStyle.Render("-")
	if task.DueDate != nil {
		due = dueStyle(*task.DueDate, today).Render(dates.Human(*task.DueDate, today))
		if done {
			due = mutedStyle.Render(dates.Human(*task.DueDate, today))
		}
	}
	line += " " + padLeft(due, 9)
	line = truncate(line, t.width)
	if task.ID == t.selectedID {
		return selectedStyle.Width(t.width).Render(line)
	}
	return padRight(line, t.width)
}

func (t *Tasks) info() string {
	parts := []string{"Tareas"}
	switch t.mode {
	case store.FilterAll:
		parts = append(parts, "Todas")
	case store.FilterDone:
		parts = append(parts, "Hechas")
	default:
		parts = append(parts, "Abiertas")
	}
	if t.filterLabel != "" {
		parts = append(parts, "filtro: "+t.filterLabel)
	}
	return strings.Join(parts, " · ")
}

func (t *Tasks) selectByID(id int64) bool {
	for i := range t.tasks {
		if t.tasks[i].ID == id {
			t.cursor, t.selectedID = i, id
			return true
		}
	}
	// Puede estar fuera del filtro actual (por ejemplo, una tarea hecha).
	t.mode, t.search = store.FilterAll, ""
	t.projectFilter, t.clientFilter, t.filterLabel = nil, nil, ""
	t.reload()
	for i := range t.tasks {
		if t.tasks[i].ID == id {
			t.cursor, t.selectedID = i, id
			return true
		}
	}
	return false
}

func (t *Tasks) move(delta int) {
	if len(t.tasks) == 0 {
		return
	}
	t.cursor = min(max(t.cursor+delta, 0), len(t.tasks)-1)
	t.selectedID = t.tasks[t.cursor].ID
}

func (t *Tasks) handleKey(a *App, msg tea.KeyPressMsg) {
	t.syncKeys()
	if t.searching {
		switch msg.String() {
		case "esc":
			t.searching, t.search = false, ""
			t.searchInput.SetValue("")
			t.reload()
		case "enter":
			t.searching = false
			t.searchInput.Blur()
			t.reload()
		default:
			t.searchInput, _ = t.searchInput.Update(msg)
			t.search = t.searchInput.Value()
			t.reload()
		}
		return
	}

	switch {
	case key.Matches(msg, taskKeys.CycleShowMode):
		switch t.mode {
		case store.FilterOpen:
			t.mode = store.FilterAll
		case store.FilterAll:
			t.mode = store.FilterDone
		default:
			t.mode = store.FilterOpen
		}
		t.reload()
	case key.Matches(msg, taskKeys.ShowOpen, taskKeys.ShowAll, taskKeys.ShowDone):
		t.mode = map[string]string{"o": store.FilterOpen, "a": store.FilterAll, "h": store.FilterDone}[msg.String()]
		t.reload()
	case key.Matches(msg, taskKeys.Search):
		t.searching = true
		t.searchInput.Focus()
	case key.Matches(msg, taskKeys.Filter):
		a.openFilterPicker()
	case key.Matches(msg, taskKeys.Clear):
		t.mode, t.search, t.filterLabel = store.FilterOpen, "", ""
		t.projectFilter, t.clientFilter = nil, nil
		t.searchInput.SetValue("")
		t.reload()
	case key.Matches(msg, taskKeys.New):
		var preset int64
		if t.projectFilter != nil {
			preset = *t.projectFilter
		}
		a.openTaskForm(store.Task{}, "", preset)
	case key.Matches(msg, taskKeys.Edit):
		if task := t.selected(); task != nil {
			a.openTaskForm(*task, strOr(task.DueDate, ""), 0)
		}
	case key.Matches(msg, taskKeys.Open):
		if task := t.selected(); task != nil {
			a.detail = taskDetail(*task)
		}
	case key.Matches(msg, taskKeys.Delete):
		if task := t.selected(); task != nil {
			id, title := task.ID, task.Title
			a.confirmAction(fmt.Sprintf("¿Borrar la tarea %q?", title), func(a *App) {
				if err := a.st.DeleteTask(id); err != nil {
					a.errf("No se pudo borrar: %v", err)
					return
				}
				a.toastf("Tarea borrada")
				a.reloadAll()
			})
		}
	case key.Matches(msg, taskKeys.Toggle):
		if task := t.selected(); task != nil {
			if task.Status == store.TaskHecho {
				task.Status = store.TaskPendiente
			} else {
				task.Status = store.TaskHecho
			}
			t.save(a, *task, "Tarea actualizada")
		}
	case key.Matches(msg, taskKeys.Status):
		if task := t.selected(); task != nil {
			task.Status = nextTaskStatus(task.Status)
			t.save(a, *task, "Estado: "+taskStatusLabels[task.Status])
		}
	case key.Matches(msg, taskKeys.PrioUp):
		if task := t.selected(); task != nil && task.Priority < 3 {
			task.Priority++
			t.save(a, *task, "Prioridad: "+priorityLabels[task.Priority])
		}
	case key.Matches(msg, taskKeys.PrioDown):
		if task := t.selected(); task != nil && task.Priority > 0 {
			task.Priority--
			t.save(a, *task, "Prioridad: "+priorityLabels[task.Priority])
		}
	case key.Matches(msg, taskKeys.GoProject):
		if task := t.selected(); task != nil {
			if a.projects.selectByID(task.ProjectID) {
				a.setTab(tabProjects)
			}
		}
	case key.Matches(msg, taskKeys.Down):
		t.move(1)
	case key.Matches(msg, taskKeys.Up):
		t.move(-1)
	case key.Matches(msg, taskKeys.PageDown):
		t.move(t.listHeight() - 1)
	case key.Matches(msg, taskKeys.PageUp):
		t.move(-(t.listHeight() - 1))
	case key.Matches(msg, taskKeys.Top):
		t.move(-len(t.tasks))
	case key.Matches(msg, taskKeys.Bottom):
		t.move(len(t.tasks))
	}
}

func (t *Tasks) save(a *App, task store.Task, message string) {
	if err := t.st.UpdateTask(&task); err != nil {
		t.reload()
		return
	}
	t.selectedID = task.ID
	t.reload()
	a.toastf("%s", message)
}

func nextTaskStatus(status string) string {
	switch status {
	case store.TaskPendiente:
		return store.TaskEnCurso
	case store.TaskEnCurso:
		return store.TaskHecho
	default:
		return store.TaskPendiente
	}
}

func taskDetail(task store.Task) *Reader {
	lines := []string{
		mutedStyle.Render("Estado: ") + statusText(task.Status),
		mutedStyle.Render("Prioridad: ") + priorityText(task.Priority),
		mutedStyle.Render("Proyecto: ") + task.ProjectName,
		mutedStyle.Render("Cliente: ") + task.ClientName,
	}
	if task.DueDate != nil {
		lines = append(lines, mutedStyle.Render("Vence: ")+dates.Human(*task.DueDate, dates.Today()))
	}
	if task.CompletedAt != nil {
		completed := *task.CompletedAt
		if len(completed) >= 10 {
			completed = completed[:10]
		}
		lines = append(lines, mutedStyle.Render("Completada: ")+dates.Human(completed, dates.Today()))
	}
	if task.Notes != "" {
		lines = append(lines, "", sectionStyle.Render("Notas"), task.Notes)
	}
	return newReader(task.Title, lines)
}

func (t *Tasks) emptyHint() string {
	switch {
	case t.mode == store.FilterOpen && t.filterLabel == "":
		return "  Nada pendiente. h muestra las hechas · n crea una · ctrl+k abre la paleta."
	case t.filterLabel != "":
		return "  Sin tareas con este filtro. esc lo limpia."
	default:
		return "  Sin tareas. n crea una, ctrl+k abre la paleta."
	}
}

func clampPriority(p int) int {
	if p < 0 || p >= len(priorityLabels) {
		return 1
	}
	return p
}
