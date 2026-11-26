package ui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/key"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tunapm/internal/dates"
	"tunapm/internal/money"
	"tunapm/internal/store"
)

type Projects struct {
	st            store.Repo
	width, height int
	projects      []store.Project
	progress      map[int64]store.TaskProgress
	selectedID    int64
	payments      []store.Payment
	totals        store.ProjectTotals
	allTotals     map[int64]store.ProjectTotals
	noteCounts    map[int64]int
}

func newProjects(st store.Repo) *Projects {
	return &Projects{st: st, width: 100, height: 20}
}

func (p *Projects) layout(width, height int) {
	p.width, p.height = width, height
	p.reload()
}

func (p *Projects) reload() {
	projects, err := p.st.ListProjects()
	if err != nil {
		return
	}
	p.projects = projects
	p.progress, _ = p.st.TaskProgress()
	p.allTotals, _ = p.st.AllProjectTotals()
	p.noteCounts = map[int64]int{}
	if notes, err := p.st.ListNotes(store.NoteFilter{}); err == nil {
		for _, n := range notes {
			if n.ProjectID != nil {
				p.noteCounts[*n.ProjectID]++
			}
		}
	}
	if p.selectedID == 0 && len(projects) > 0 {
		p.selectedID = projects[0].ID
	}
	found := false
	for _, proj := range projects {
		if proj.ID == p.selectedID {
			found = true
			break
		}
	}
	if !found {
		p.selectedID = 0
		if len(projects) > 0 {
			p.selectedID = projects[0].ID
		}
	}
	p.loadDetail()
}

func (p *Projects) loadDetail() {
	p.payments, p.totals = nil, store.ProjectTotals{}
	project := p.selectedProject()
	if project == nil {
		return
	}
	p.payments, _ = p.st.PaymentsByProject(project.ID)
	p.totals, _ = p.st.ProjectTotals(project.ID)
}

func (p *Projects) selectedProject() *store.Project {
	for i := range p.projects {
		if p.projects[i].ID == p.selectedID {
			return &p.projects[i]
		}
	}
	return nil
}

func (p *Projects) selectByID(id int64) bool {
	for _, proj := range p.projects {
		if proj.ID == id {
			p.selectedID = id
			p.loadDetail()
			return true
		}
	}
	return false
}

func (p *Projects) selectedIndex() int {
	for i := range p.projects {
		if p.projects[i].ID == p.selectedID {
			return i
		}
	}
	return 0
}

func (p *Projects) listWidth() int {
	return max(min(p.width*3/5, 84), 30)
}

func (p *Projects) view() string {
	return masterDetail(p.width, p.height, p.listWidth(), p.listView, p.detailView)
}

// receivable es lo que falta por cobrar de un proyecto: presupuesto menos lo pagado.
func (p *Projects) receivable(proj store.Project) int64 {
	if proj.Status == store.StatusCancelado {
		return 0
	}
	if owed := proj.BudgetCents - p.allTotals[proj.ID].PaidCents; owed > 0 {
		return owed
	}
	return 0
}

func (p *Projects) summaryLine() string {
	receivable := map[string]int64{}
	pending := 0
	for _, proj := range p.projects {
		receivable[proj.Currency] += p.receivable(proj)
		if proj.Status != store.StatusCancelado {
			prog := p.progress[proj.ID]
			pending += prog.Total - prog.Done
		}
	}
	return mutedStyle.Render("Por cobrar: ") + titleStyle.Render(formatTotals(receivable)) +
		mutedStyle.Render("  ·  Tareas pendientes: ") + titleStyle.Render(fmt.Sprint(pending))
}

func (p *Projects) listView(width, height int) string {
	var lines []string
	const statusW, tasksW, owedW, dueW = 10, 5, 12, 8
	nameW := max(width-2-statusW-tasksW-owedW-dueW-4, 8)
	lines = append(lines, headStyle.Render(padRight("  PROYECTO", 2+nameW)+" "+padRight("ESTADO", statusW)+" "+
		padLeft("PEND.", tasksW)+" "+padLeft("POR COBRAR", owedW)+" "+padLeft("ENTREGA", dueW)))
	if len(p.projects) == 0 {
		lines = append(lines, "", mutedStyle.Render("  Sin proyectos. n crea uno."))
	}
	today := dates.Today()
	visible := max(height-3, 1) // encabezado + separador + resumen
	start := p.selectedIndex() - visible/2
	if start > len(p.projects)-visible {
		start = len(p.projects) - visible
	}
	if start < 0 {
		start = 0
	}
	for i := start; i < len(p.projects) && i < start+visible; i++ {
		proj := p.projects[i]
		prog := p.progress[proj.ID]
		selected := proj.ID == p.selectedID
		closed := proj.Status == store.StatusCompletado || proj.Status == store.StatusCancelado
		marker := cursorMark(selected)
		name := titleStyle.Render(truncate(proj.Name, nameW))
		if closed {
			name = mutedStyle.Render(truncate(proj.Name, nameW))
		}
		if room := nameW - lipglossWidth(truncate(proj.Name, nameW)) - 3; room > 3 && proj.ClientName != "" {
			name += faintStyle.Render(" · " + truncate(proj.ClientName, room))
		}
		tasks := mutedStyle.Render("-")
		if prog.Total > 0 {
			if left := prog.Total - prog.Done; left > 0 {
				tasks = warnStyle.Render(fmt.Sprint(left))
			} else {
				tasks = successStyle.Render("✓")
			}
		}
		owed := mutedStyle.Render("-")
		if cents := p.receivable(proj); cents > 0 {
			owed = money.Format(cents, proj.Currency)
			if closed {
				owed = lipgloss.NewStyle().Foreground(colorRed).Render(owed)
			} else {
				owed = lipgloss.NewStyle().Foreground(colorYellow).Render(owed)
			}
		} else if proj.Status != store.StatusCancelado && proj.BudgetCents > 0 {
			owed = successStyle.Render("al día")
		}
		due := mutedStyle.Render("-")
		if proj.DueDate != nil {
			due = dueStyle(*proj.DueDate, today).Render(dates.Short(*proj.DueDate, today))
			if closed {
				due = mutedStyle.Render(dates.Short(*proj.DueDate, today))
			}
		}
		line := marker + padRight(name, nameW) + " " + padRight(projectStatusText(proj.Status), statusW) + " " +
			padLeft(tasks, tasksW) + " " + padLeft(owed, owedW) + " " + padLeft(due, dueW)
		if selected {
			line = selectedStyle.Width(width).Render(line)
		}
		lines = append(lines, line)
	}
	bodyH := max(height-2, 1)
	for len(lines) < bodyH {
		lines = append(lines, "")
	}
	lines = append(lines[:bodyH], faintStyle.Render(strings.Repeat("─", width)), padRight("  "+p.summaryLine(), width))
	return strings.Join(lines, "\n")
}

func (p *Projects) detailView(width, height int) string {
	proj := p.selectedProject()
	if proj == nil {
		return mutedStyle.Render("Selecciona un proyecto")
	}
	today := dates.Today()
	prog := p.progress[proj.ID]
	cur := proj.Currency

	billing := "un solo pago"
	if proj.BillingType == store.BillingParcial {
		billing = "por cuotas"
	}
	lines := []string{
		titleStyle.Render(wrapLine(proj.Name, width)),
		mutedStyle.Render(proj.ClientName),
		"",
		field("Estado", projectStatusText(proj.Status)),
		field("Fechas", humanOrDash(proj.StartDate, today)+mutedStyle.Render(" → ")+humanOrDash(proj.DueDate, today)),
		field("Presupuesto", money.Format(proj.BudgetCents, cur)+mutedStyle.Render("  "+billing)),
		field("Tareas", taskSummary(prog)),
		field("Notas", noteSummary(p.noteCounts[proj.ID])),
		"",
		sectionStyle.Render("Cobro"),
		field("Cobrado", successStyle.Render(money.Format(p.totals.PaidCents, cur))+"  "+
			progressBar(int(p.totals.PaidCents/100), int(proj.BudgetCents/100), 10)+
			mutedStyle.Render(fmt.Sprintf(" %d%%", percentOf(p.totals.PaidCents, proj.BudgetCents)))),
		field("Por cobrar", titleStyle.Render(money.Format(p.receivable(*proj), cur))),
	}
	if unassigned := p.totals.UnassignedCents(proj.BudgetCents); unassigned > 0 {
		lines = append(lines, field("Sin programar", warnStyle.Render(money.Format(unassigned, cur))+
			mutedStyle.Render("  c agrega cuota")))
	}

	lines = append(lines, "", sectionStyle.Render("Cuotas"))
	if len(p.payments) == 0 {
		lines = append(lines, mutedStyle.Render("Sin cuotas. c agrega una."))
	}
	for _, pay := range p.payments {
		status := pay.StatusToday(today)
		date := padRight(dates.Human(pay.ScheduledDate, today), 7)
		if status == store.PaymentVencido {
			date = errorStyle.Render(date)
		} else {
			date = mutedStyle.Render(date)
		}
		line := date + " " + padLeft(money.Format(pay.AmountCents, cur), 12) + "  " +
			padRight(paymentStatusText(status), 9)
		if pay.Notes != "" {
			line += " " + mutedStyle.Render(pay.Notes)
		}
		lines = append(lines, line)
	}
	if proj.Description != "" {
		lines = append(lines, "", sectionStyle.Render("Descripción"), wrapLine(proj.Description, width))
	}
	return clipLines(lines, width, height)
}

func noteSummary(count int) string {
	if count == 0 {
		return mutedStyle.Render("sin notas  o las abre")
	}
	return plural(count, "nota", "notas") + mutedStyle.Render("  o las abre")
}

func taskSummary(prog store.TaskProgress) string {
	if prog.Total == 0 {
		return mutedStyle.Render("sin tareas")
	}
	return progressBar(prog.Done, prog.Total, 10) +
		mutedStyle.Render(fmt.Sprintf(" %d de %d hechas", prog.Done, prog.Total))
}

func percentOf(part, total int64) int {
	if total <= 0 {
		return 0
	}
	return int(part * 100 / total)
}

func humanOrDash(date *string, today string) string {
	if date == nil {
		return mutedStyle.Render("-")
	}
	return dates.Human(*date, today)
}

func (p *Projects) info() string {
	active := 0
	for _, proj := range p.projects {
		if proj.Status == store.StatusActivo {
			active++
		}
	}
	info := fmt.Sprintf("Proyectos · %s · %d activos", plural(len(p.projects), "proyecto", "proyectos"), active)
	return info
}

func (p *Projects) handleKey(a *App, msg tea.KeyMsg) {
	p.syncKeys()
	proj := p.selectedProject()
	switch {
	case key.Matches(msg, projectKeys.Down):
		if len(p.projects) > 0 {
			idx := min(p.selectedIndex()+1, len(p.projects)-1)
			p.selectByID(p.projects[idx].ID)
		}
	case key.Matches(msg, projectKeys.Up):
		if len(p.projects) > 0 {
			idx := max(p.selectedIndex()-1, 0)
			p.selectByID(p.projects[idx].ID)
		}
	case key.Matches(msg, projectKeys.New):
		a.openProjectForm(store.Project{}, "", 0)
	case key.Matches(msg, projectKeys.Edit):
		if proj != nil {
			a.openProjectForm(*proj, "", 0)
		}
	case key.Matches(msg, projectKeys.Status):
		if proj != nil {
			a.openProjectStatusPicker(proj.ID)
		}
	case key.Matches(msg, projectKeys.AddPayment):
		if proj != nil {
			a.openPaymentForm(store.Payment{ProjectID: proj.ID}, "", proj.ID)
		}
	case key.Matches(msg, projectKeys.Pay):
		if proj == nil {
			return
		}
		for _, pay := range p.payments {
			if pay.PaidDate == nil {
				a.openPaidDateForm(pay)
				return
			}
		}
		a.toastf("No hay cuotas pendientes en %s", proj.Name)
	case key.Matches(msg, projectKeys.Notes):
		if proj != nil {
			a.notes.showProject(*proj)
			a.setTab(tabNotes)
		}
	case key.Matches(msg, projectKeys.Tasks):
		if proj == nil {
			return
		}
		id := proj.ID
		a.tasks.mode, a.tasks.search = store.FilterOpen, ""
		a.tasks.projectFilter, a.tasks.clientFilter = &id, nil
		a.tasks.filterLabel = "proyecto: " + proj.Name
		a.tasks.reload()
		a.setTab(tabTasks)
	case key.Matches(msg, projectKeys.Delete):
		if proj == nil {
			return
		}
		id, name := proj.ID, proj.Name
		a.confirmAction(fmt.Sprintf("¿Borrar %q y todas sus tareas y cuotas?", name), func(a *App) {
			if err := a.st.DeleteProject(id); err != nil {
				a.errf("No se pudo borrar: %v", err)
				return
			}
			a.toastf("Proyecto borrado")
			a.reloadAll()
		})
	}
}
