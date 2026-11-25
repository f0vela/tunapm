package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"tunapm/internal/agenda"
	"tunapm/internal/dates"
	"tunapm/internal/store"
)

const (
	calMonth = "mes"
	calWeek  = "semana"
	calDay   = "día"
)

// Calendar muestra una cuadrícula del mes a la izquierda y la agenda a la
// derecha. La agenda cubre el día, la semana o el mes según la vista.
// Todo sale de tareas, cuotas, proyectos y eventos (ver internal/agenda).
type Calendar struct {
	st            store.Repo
	width, height int
	date          string // día seleccionado
	mode          string
	byDate        map[string][]agenda.Item
	item          int // entrada seleccionada dentro del día
}

func newCalendar(st store.Repo) *Calendar {
	return &Calendar{st: st, width: 100, height: 20, date: dates.Today(), mode: calMonth}
}

func (c *Calendar) layout(width, height int) {
	c.width, c.height = width, height
	c.reload()
}

// reload carga las semanas completas que cubren el mes del día seleccionado.
func (c *Calendar) reload() {
	grid := agenda.MonthGrid(c.date)
	items, err := agenda.Build(c.st, grid[0][0], grid[len(grid)-1][6], dates.Today())
	if err != nil {
		c.byDate = nil
		return
	}
	c.byDate = agenda.ByDate(items)
	c.clampItem()
}

func (c *Calendar) clampItem() {
	n := len(c.byDate[c.date])
	if c.item >= n {
		c.item = n - 1
	}
	if c.item < 0 {
		c.item = 0
	}
}

func (c *Calendar) goTo(date string) {
	sameMonth := agenda.SameMonth(date, c.date)
	c.date, c.item = date, 0
	if !sameMonth {
		c.reload()
	}
}

func (c *Calendar) selected() *agenda.Item {
	items := c.byDate[c.date]
	if c.item < 0 || c.item >= len(items) {
		return nil
	}
	return &items[c.item]
}

// visibleDays son los días que lista la agenda en la vista actual.
func (c *Calendar) visibleDays() []string {
	switch c.mode {
	case calDay:
		return []string{c.date}
	case calWeek:
		return agenda.Week(c.date)
	}
	// Mes: solo los días con algo, más el seleccionado para que siempre se vea.
	first, last := agenda.MonthBounds(c.date)
	var days []string
	for d := first; d <= last; d = agenda.Add(d, 1) {
		if len(c.byDate[d]) > 0 || d == c.date {
			days = append(days, d)
		}
	}
	return days
}

// ---------- vista ----------

func (c *Calendar) title() string {
	switch c.mode {
	case calDay:
		return agenda.DayTitle(c.date) + " " + c.date[:4]
	case calWeek:
		return "semana del " + agenda.WeekTitle(c.date)
	}
	return agenda.MonthTitle(c.date)
}

func (c *Calendar) view() string {
	var views []string
	for _, v := range []string{calMonth, calWeek, calDay} {
		if v == c.mode {
			views = append(views, titleStyle.Underline(true).Render(v))
		} else {
			views = append(views, mutedStyle.Render(v))
		}
	}
	header := spread(" "+titleStyle.Render(c.title()),
		keyStyle.Render("v")+" "+strings.Join(views, mutedStyle.Render("  "))+" ", c.width)
	return header + "\n" + masterDetail(c.width, max(c.height-1, 1), 30, c.gridView, c.agendaView)
}

func itemColor(it agenda.Item) lipgloss.TerminalColor {
	switch {
	case it.Overdue:
		return colorRed
	case it.Done:
		return colorMuted
	}
	switch it.Type {
	case agenda.TypeEvent:
		return colorAccent
	case agenda.TypePayment:
		return colorYellow
	case agenda.TypeProjectDue:
		return colorBlue
	case agenda.TypeProjectStart:
		return colorMuted
	}
	return colorText
}

// dayMark resume un día en la cuadrícula: rojo si hay algo vencido, acento si
// hay eventos, atenuado si solo hay cosas ya hechas.
func dayMark(items []agenda.Item) string {
	if len(items) == 0 {
		return " "
	}
	color, pending := colorMuted, false
	for _, it := range items {
		if it.Overdue {
			return lipgloss.NewStyle().Foreground(colorRed).Render("•")
		}
		if !it.Done {
			pending = true
			if it.Type == agenda.TypeEvent {
				color = colorAccent
			}
		}
	}
	if pending && color == colorMuted {
		color = colorText
	}
	return lipgloss.NewStyle().Foreground(color).Render("•")
}

func (c *Calendar) gridView(width, height int) string {
	today := dates.Today()
	pad := strings.Repeat(" ", max((width-28)/2, 0))
	lines := []string{pad + headStyle.Render(" "+strings.Join(agenda.WeekdayShort, "  ")+" ")}
	selectedWeek := agenda.WeekStart(c.date)
	for _, week := range agenda.MonthGrid(c.date) {
		var b strings.Builder
		for _, day := range week {
			num := fmt.Sprintf("%2s", agenda.DayNumber(day))
			style := lipgloss.NewStyle().Foreground(colorText)
			switch {
			case day == c.date:
				style = lipgloss.NewStyle().Bold(true).Foreground(colorOnAcc).Background(colorAccent)
			case day == today:
				style = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(colorAccent)
			case !agenda.SameMonth(day, c.date):
				style = faintStyle
			}
			b.WriteString(" " + style.Render(num) + dayMark(c.byDate[day]))
		}
		row := b.String()
		// En la vista de semana se resalta la fila completa.
		if c.mode == calWeek && week[0] == selectedWeek {
			row = lipgloss.NewStyle().Background(colorSelBg).Render(row)
		}
		lines = append(lines, pad+row)
	}

	counts := map[string]int{}
	first, last := agenda.MonthBounds(c.date)
	for d := first; d <= last; d = agenda.Add(d, 1) {
		for _, it := range c.byDate[d] {
			switch it.Type {
			case agenda.TypeEvent:
				counts["eventos"]++
			case agenda.TypeTask:
				counts["tareas"]++
			case agenda.TypePayment:
				counts["cobros"]++
			default:
				counts["proyectos"]++
			}
		}
	}
	lines = append(lines, "")
	for _, k := range []string{"eventos", "tareas", "cobros", "proyectos"} {
		if counts[k] > 0 {
			lines = append(lines, pad+" "+mutedStyle.Render(padRight(k, 10))+fmt.Sprint(counts[k]))
		}
	}
	lines = append(lines, "",
		pad+" "+renderHints("←→ día · ↑↓ semana"),
		pad+" "+renderHints("[ ] mes · t hoy"),
		pad+" "+renderHints("tab siguiente entrada"),
	)
	for i := range lines {
		lines[i] = truncate(lines[i], width)
	}
	return fitLines(strings.Join(lines, "\n"), height)
}

func (c *Calendar) agendaView(width, height int) string {
	today := dates.Today()
	var lines []string
	focus := 0 // línea que debe quedar visible
	for _, day := range c.visibleDays() {
		items := c.byDate[day]
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		head := agenda.DayTitle(day)
		switch {
		case day == c.date:
			focus = len(lines)
			head = sectionStyle.Render(head)
		default:
			head = titleStyle.Render(head)
		}
		if day == today {
			head += mutedStyle.Render("  hoy")
		}
		lines = append(lines, head)
		if len(items) == 0 {
			empty := "  -"
			if day == c.date {
				empty = "  Nada programado. n agrega un evento."
			}
			lines = append(lines, mutedStyle.Render(empty))
		}
		for i, it := range items {
			selected := day == c.date && i == c.item
			if selected {
				focus = len(lines)
			}
			clock := padRight(it.Time, 5)
			if it.Time == "" {
				clock = faintStyle.Render(padRight("  ·", 5))
			}
			text := it.Text()
			if it.Done {
				text = mutedStyle.Render(text)
			}
			line := cursorMark(selected) + clock + " " +
				lipgloss.NewStyle().Foreground(itemColor(it)).Render(padRight(it.Label(), 12)) + " " + text
			if room := width - lipglossWidth(line) - 3; room > 6 && it.Detail != "" {
				line += faintStyle.Render(" · " + truncate(it.Detail, room))
			}
			line = truncate(line, width)
			if selected {
				line = selectedStyle.Width(width).Render(line)
			}
			lines = append(lines, line)
		}
	}
	// Ventana: el día (o la entrada) seleccionado siempre queda a la vista.
	start := 0
	if len(lines) > height {
		start = max(min(focus-height/3, len(lines)-height), 0)
	}
	end := min(start+height, len(lines))
	return fitLines(strings.Join(lines[start:end], "\n"), height)
}

func (c *Calendar) info() string {
	return "Calendario · " + agenda.DayTitle(c.date)
}

func (c *Calendar) hints() string {
	hints := "n evento · v vista · t hoy"
	if it := c.selected(); it != nil {
		switch it.Type {
		case agenda.TypeEvent:
			hints = "n evento · e editar · d borrar · v vista · t hoy"
		case agenda.TypeTask:
			hints = "enter ver tarea · espacio hecha · n evento · v vista · t hoy"
		case agenda.TypePayment:
			hints = "enter ver cobro · x pagada · n evento · v vista · t hoy"
		default:
			hints = "enter ver proyecto · n evento · v vista · t hoy"
		}
	}
	return hints + " · ctrl+k paleta"
}

// ---------- teclas ----------

func (c *Calendar) handleKey(a *App, msg tea.KeyMsg) {
	it := c.selected()
	switch msg.String() {
	case "h", "left":
		c.goTo(agenda.Add(c.date, -1))
	case "l", "right":
		c.goTo(agenda.Add(c.date, 1))
	case "k", "up":
		c.goTo(agenda.Add(c.date, -7))
	case "j", "down":
		c.goTo(agenda.Add(c.date, 7))
	case "[", "pgup":
		c.goTo(agenda.AddMonths(c.date, -1))
	case "]", "pgdown":
		c.goTo(agenda.AddMonths(c.date, 1))
	case "t":
		c.goTo(dates.Today())
	case "v":
		c.mode = map[string]string{calMonth: calWeek, calWeek: calDay, calDay: calMonth}[c.mode]
	case "tab":
		if n := len(c.byDate[c.date]); n > 0 {
			c.item = (c.item + 1) % n
		}
	case "shift+tab":
		if n := len(c.byDate[c.date]); n > 0 {
			c.item = (c.item + n - 1) % n
		}
	case "n":
		a.openEventForm(store.Event{Date: c.date, Kind: store.EventReunion})
	case "e", "enter":
		if it != nil {
			c.open(a, *it, msg.String() == "e")
		}
	case "d":
		if it == nil || it.Type != agenda.TypeEvent {
			if it != nil {
				a.errf("Solo los eventos se borran aquí; una %s se gestiona en su vista (enter)", it.Label())
			}
			return
		}
		id, title := it.ID, it.Title
		a.confirmAction(fmt.Sprintf("¿Borrar el evento %q?", title), func(a *App) {
			if err := a.st.DeleteEvent(id); err != nil {
				a.errf("No se pudo borrar: %v", err)
				return
			}
			a.toastf("Evento borrado")
			a.reloadAll()
		})
	case " ":
		if it != nil && it.Type == agenda.TypeTask {
			task, err := a.st.GetTask(it.ID)
			if err != nil {
				a.errf("%v", err)
				return
			}
			if task.Status == store.TaskHecho {
				task.Status = store.TaskPendiente
			} else {
				task.Status = store.TaskHecho
			}
			if err := a.st.UpdateTask(&task); err != nil {
				a.errf("%v", err)
				return
			}
			a.toastf("Tarea actualizada")
			a.reloadAll()
		}
	case "x":
		if it != nil && it.Type == agenda.TypePayment {
			if pay, err := a.st.GetPayment(it.ID); err == nil {
				a.openPaidDateForm(pay)
			}
		}
	}
}

// open abre la entrada donde vive: los eventos se editan aquí; lo demás lleva a su vista.
func (c *Calendar) open(a *App, it agenda.Item, edit bool) {
	switch it.Type {
	case agenda.TypeEvent:
		ev, err := a.st.GetEvent(it.ID)
		if err != nil {
			a.errf("%v", err)
			return
		}
		if edit {
			a.openEventForm(ev)
		} else {
			a.detail = eventDetail(ev)
		}
	case agenda.TypeTask:
		if a.tasks.selectByID(it.ID) {
			a.setTab(tabTasks)
		}
	case agenda.TypePayment:
		a.payments.selectedID = it.ID
		a.payments.reload()
		a.setTab(tabPayments)
	default:
		if a.projects.selectByID(it.ID) {
			a.setTab(tabProjects)
		}
	}
}

func eventDetail(ev store.Event) *Detail {
	when := agenda.DayTitle(ev.Date) + " " + ev.Date[:4]
	if ev.Time != nil {
		when += ", " + *ev.Time
	} else {
		when += ", todo el día"
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(colorAccent).Render(store.EventKindLabels[ev.Kind]) + mutedStyle.Render("  "+when),
	}
	if ev.ProjectName != "" || ev.ClientName != "" {
		lines = append(lines, mutedStyle.Render(strings.Trim(ev.ProjectName+" · "+ev.ClientName, " ·")))
	}
	if strings.TrimSpace(ev.Notes) != "" {
		lines = append(lines, "")
		lines = append(lines, strings.Split(ev.Notes, "\n")...)
	}
	return &Detail{title: ev.Title, lines: lines}
}

// openEventForm crea o edita un evento. Hora, proyecto y cliente son opcionales.
func (a *App) openEventForm(ev store.Event) {
	projects, _ := a.st.ListProjects()
	clients, _ := a.st.ListClients()

	title, notes, dateRaw := ev.Title, ev.Notes, ev.Date
	kind := ev.Kind
	if kind == "" {
		kind = store.EventReunion
	}
	timeRaw := strOr(ev.Time, "")
	var projectID, clientID int64
	if ev.ProjectID != nil {
		projectID = *ev.ProjectID
	}
	if ev.ClientID != nil {
		clientID = *ev.ClientID
	}

	kindOpts := make([]huh.Option[string], len(store.EventKinds))
	for i, k := range store.EventKinds {
		kindOpts[i] = huh.NewOption(store.EventKindLabels[k], k)
	}
	projectOpts := []huh.Option[int64]{huh.NewOption("Sin proyecto", int64(0))}
	for _, p := range projects {
		projectOpts = append(projectOpts, huh.NewOption(truncate(p.Name+" · "+p.ClientName, 60), p.ID))
	}
	clientOpts := []huh.Option[int64]{huh.NewOption("Sin cliente (o el del proyecto)", int64(0))}
	for _, c := range clients {
		clientOpts = append(clientOpts, huh.NewOption(truncate(strings.Trim(c.Name+" · "+c.Company, " ·"), 60), c.ID))
	}
	validTime := func(s string) error {
		_, err := store.NormalizeTime(s)
		return err
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Título").Value(&title).Validate(huh.ValidateNotEmpty()),
			huh.NewSelect[string]().Title("Tipo").Options(kindOpts...).Value(&kind).Height(5),
			huh.NewInput().Title("Fecha").Placeholder("hoy, viernes, 15/12, +3d").Value(&dateRaw).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("la fecha es obligatoria")
					}
					return validateDate(s)
				}),
			huh.NewInput().Title("Hora (opcional)").Placeholder("15:30; vacío = todo el día").Value(&timeRaw).Validate(validTime),
		).Title("Evento"),
		huh.NewGroup(
			huh.NewSelect[int64]().Title("Proyecto").Options(projectOpts...).Value(&projectID).Height(6),
			huh.NewSelect[int64]().Title("Cliente").Options(clientOpts...).Value(&clientID).Height(6),
			huh.NewText().Title("Notas").Value(&notes).Lines(3),
		).Title("Relacionado con"),
	)

	a.openForm(form, func() error {
		date, err := parseOptionalDate(dateRaw)
		if err != nil || date == nil {
			return fmt.Errorf("fecha inválida: %s", dateRaw)
		}
		clock, err := store.NormalizeTime(timeRaw)
		if err != nil {
			return err
		}
		out := ev
		out.Title, out.Kind, out.Date, out.Notes = strings.TrimSpace(title), kind, *date, notes
		out.Time, out.ProjectID, out.ClientID = nil, nil, nil
		if clock != "" {
			out.Time = &clock
		}
		if projectID != 0 {
			out.ProjectID = &projectID
		}
		if clientID != 0 {
			out.ClientID = &clientID
		}
		if out.ID == 0 {
			err = a.st.CreateEvent(&out)
		} else {
			err = a.st.UpdateEvent(&out)
		}
		if err == nil {
			a.calendar.date = out.Date
			a.setTab(tabCalendar)
		}
		return err
	})
}
