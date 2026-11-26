package ui

import (
	"charm.land/bubbles/v2/key"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"tunapm/internal/dates"
	"tunapm/internal/store"
)

// Notes es la vista de notas: lista a la izquierda, contenido a la derecha.
// Se filtra por etiqueta, por proyecto (o "sin proyecto") y por texto.
type Notes struct {
	st            store.Repo
	width, height int
	notes         []store.Note
	tags          []store.TagCount
	selectedID    int64

	tag           string
	projectFilter *int64
	unassigned    bool
	projectLabel  string
	search        string
	searching     bool
	searchInput   textinput.Model
}

func newNotes(st store.Repo) *Notes {
	ti := newInput("/ ", "buscar en título, texto, etiquetas o proyecto…")
	return &Notes{st: st, width: 100, height: 20, searchInput: ti}
}

func (n *Notes) layout(width, height int) {
	n.width, n.height = width, height
	n.searchInput.SetWidth(max(width-30, 10))
	n.reload()
}

func (n *Notes) filter() store.NoteFilter {
	return store.NoteFilter{ProjectID: n.projectFilter, Unassigned: n.unassigned, Tag: n.tag, Search: n.search}
}

func (n *Notes) reload() {
	notes, err := n.st.ListNotes(n.filter())
	if err != nil {
		n.notes = nil
		return
	}
	n.notes = notes
	n.tags, _ = n.st.NoteTags()
	if n.selected() == nil {
		n.selectedID = 0
		if len(notes) > 0 {
			n.selectedID = notes[0].ID
		}
	}
}

func (n *Notes) selected() *store.Note {
	for i := range n.notes {
		if n.notes[i].ID == n.selectedID {
			return &n.notes[i]
		}
	}
	return nil
}

func (n *Notes) selectedIndex() int {
	for i := range n.notes {
		if n.notes[i].ID == n.selectedID {
			return i
		}
	}
	return 0
}

func (n *Notes) hasFilters() bool {
	return n.tag != "" || n.projectFilter != nil || n.unassigned || n.search != ""
}

func (n *Notes) clearFilters() {
	n.tag, n.projectFilter, n.unassigned, n.projectLabel, n.search = "", nil, false, "", ""
	n.searchInput.SetValue("")
}

// selectByID selecciona una nota; si los filtros la ocultan, los quita.
func (n *Notes) selectByID(id int64) bool {
	n.selectedID = id
	if n.selected() != nil {
		return true
	}
	n.clearFilters()
	n.reload()
	n.selectedID = id
	return n.selected() != nil
}

// showProject deja la vista filtrada por un proyecto (desde Proyectos o la paleta).
func (n *Notes) showProject(p store.Project) {
	n.clearFilters()
	id := p.ID
	n.projectFilter, n.projectLabel = &id, p.Name
	n.reload()
}

func tagText(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return "#" + strings.Join(tags, " #")
}

// noteDate muestra una marca de tiempo (guardada en UTC) como fecha local.
func noteDate(stamp, today string) string {
	return dates.Human(store.LocalDate(stamp), today)
}

// ---------- vista ----------

func (n *Notes) view() string {
	var header string
	if n.searching {
		header = spread(n.searchInput.View(), mutedStyle.Render("enter listo · esc limpiar"), n.width)
	} else {
		var parts []string
		if n.tag != "" {
			parts = append(parts, lipgloss.NewStyle().Foreground(colorAccent).Render("#"+n.tag))
		}
		switch {
		case n.unassigned:
			parts = append(parts, titleStyle.Render("sin proyecto"))
		case n.projectFilter != nil:
			parts = append(parts, mutedStyle.Render("proyecto: ")+titleStyle.Render(truncate(n.projectLabel, 40)))
		}
		if n.search != "" {
			parts = append(parts, mutedStyle.Render("texto: ")+titleStyle.Render(n.search))
		}
		left := titleStyle.Render("Todas las notas")
		if len(parts) > 0 {
			left = strings.Join(parts, mutedStyle.Render("  "))
		}
		header = spread(" "+left, mutedStyle.Render(plural(len(n.notes), "nota", "notas")+" "), n.width)
	}
	bodyH := max(n.height-1, 1)
	listW := max(min(n.width/2, 64), 30)
	return header + "\n" + masterDetail(n.width, bodyH, listW, n.listView, n.detailView)
}

func (n *Notes) emptyHint() string {
	if n.hasFilters() {
		return "  Ninguna nota con este filtro. esc lo limpia."
	}
	return "  Sin notas todavía. n crea una; ctrl+k y \"n Idea #etiqueta\" también."
}

func (n *Notes) listView(width, height int) string {
	const tagsW, dateW = 18, 6
	titleW := max(width-2-tagsW-dateW-2, 8)
	if width < 50 {
		titleW = max(width-2-dateW-1, 8)
	}
	head := padRight("  NOTA", 2+titleW)
	if width >= 50 {
		head += " " + padRight("ETIQUETAS", tagsW)
	}
	lines := []string{headStyle.Render(head + " " + padLeft("EDITADA", dateW))}
	if len(n.notes) == 0 {
		lines = append(lines, "", mutedStyle.Render(truncate(n.emptyHint(), width)))
	}
	today := dates.Today()
	visible := max(height-1, 1)
	start := n.selectedIndex() - visible/2
	if start > len(n.notes)-visible {
		start = len(n.notes) - visible
	}
	if start < 0 {
		start = 0
	}
	for i := start; i < len(n.notes) && i < start+visible; i++ {
		note := n.notes[i]
		selected := note.ID == n.selectedID
		title := titleStyle.Render(truncate(note.Title, titleW))
		if room := titleW - lipglossWidth(truncate(note.Title, titleW)) - 3; room > 3 && note.ProjectName != "" {
			title += faintStyle.Render(" · " + truncate(note.ProjectName, room))
		}
		line := cursorMark(selected) + padRight(title, titleW)
		if width >= 50 {
			line += " " + lipgloss.NewStyle().Foreground(colorAccent).Render(padRight(truncate(tagText(note.Tags), tagsW), tagsW))
		}
		line += " " + mutedStyle.Render(padLeft(noteDate(note.UpdatedAt, today), dateW))
		if selected {
			line = selectedStyle.Width(width).Render(line)
		}
		lines = append(lines, line)
	}
	return fitLines(strings.Join(lines, "\n"), height)
}

func (n *Notes) detailView(width, height int) string {
	note := n.selected()
	if note == nil {
		return mutedStyle.Render("Selecciona una nota")
	}
	today := dates.Today()
	lines := []string{titleStyle.Render(wrapLine(note.Title, width))}
	if note.ProjectName != "" {
		lines = append(lines, mutedStyle.Render(note.ProjectName+" · "+note.ClientName))
	} else {
		lines = append(lines, mutedStyle.Render("Sin proyecto"))
	}
	lines = append(lines, "")
	if len(note.Tags) > 0 {
		lines = append(lines, field("Etiquetas", lipgloss.NewStyle().Foreground(colorAccent).Render(tagText(note.Tags))))
	}
	lines = append(lines, field("Editada", noteDate(note.UpdatedAt, today)+mutedStyle.Render("  creada "+noteDate(note.CreatedAt, today))), "")
	if strings.TrimSpace(note.Body) == "" {
		lines = append(lines, mutedStyle.Render("Sin texto. e la edita."))
	}
	for _, para := range strings.Split(note.Body, "\n") {
		lines = append(lines, wrapLine(para, width))
	}
	return clipLines(lines, width, height)
}

func (n *Notes) info() string {
	parts := []string{"Notas"}
	if n.tag != "" {
		parts = append(parts, "#"+n.tag)
	}
	if n.unassigned {
		parts = append(parts, "sin proyecto")
	} else if n.projectFilter != nil {
		parts = append(parts, truncate(n.projectLabel, 30))
	}
	return strings.Join(parts, " · ")
}

// ---------- teclas ----------

func (n *Notes) move(delta int) {
	if len(n.notes) == 0 {
		return
	}
	idx := max(min(n.selectedIndex()+delta, len(n.notes)-1), 0)
	n.selectedID = n.notes[idx].ID
}

func (n *Notes) handleKey(a *App, msg tea.KeyPressMsg) {
	n.syncKeys()
	if n.searching {
		switch msg.String() {
		case "esc":
			n.searching, n.search = false, ""
			n.searchInput.SetValue("")
			n.searchInput.Blur()
			n.reload()
		case "enter":
			n.searching = false
			n.searchInput.Blur()
		default:
			n.searchInput, _ = n.searchInput.Update(msg)
			n.search = n.searchInput.Value()
			n.reload()
		}
		return
	}

	note := n.selected()
	switch {
	case key.Matches(msg, noteKeys.Down):
		n.move(1)
	case key.Matches(msg, noteKeys.Up):
		n.move(-1)
	case key.Matches(msg, noteKeys.Search):
		n.searching = true
		n.searchInput.Focus()
	case key.Matches(msg, noteKeys.Clear):
		n.clearFilters()
		n.reload()
	case key.Matches(msg, noteKeys.New):
		draft := store.Note{ProjectID: n.projectFilter}
		if n.tag != "" {
			draft.Tags = []string{n.tag}
		}
		a.openNoteForm(draft)
	case key.Matches(msg, noteKeys.Edit):
		if note != nil {
			a.openNoteForm(*note)
		}
	case key.Matches(msg, noteKeys.Read):
		if note != nil {
			a.detail = noteDetail(*note)
		}
	case key.Matches(msg, noteKeys.Tag):
		a.openNoteTagPicker()
	case key.Matches(msg, noteKeys.Project):
		a.openNoteProjectPicker()
	case key.Matches(msg, noteKeys.GoProject):
		if note != nil && note.ProjectID != nil && a.projects.selectByID(*note.ProjectID) {
			a.setTab(tabProjects)
		}
	case key.Matches(msg, noteKeys.Delete):
		if note == nil {
			return
		}
		id, title := note.ID, note.Title
		a.confirmAction(fmt.Sprintf("¿Borrar la nota %q?", title), func(a *App) {
			if err := a.st.DeleteNote(id); err != nil {
				a.errf("No se pudo borrar: %v", err)
				return
			}
			a.toastf("Nota borrada")
			a.reloadAll()
		})
	}
}

func noteDetail(note store.Note) *Reader {
	today := dates.Today()
	project := "Sin proyecto"
	if note.ProjectName != "" {
		project = note.ProjectName + " · " + note.ClientName
	}
	lines := []string{mutedStyle.Render(project)}
	if len(note.Tags) > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(colorAccent).Render(tagText(note.Tags)))
	}
	lines = append(lines, mutedStyle.Render("Editada "+noteDate(note.UpdatedAt, today)), "")
	lines = append(lines, strings.Split(note.Body, "\n")...)
	return newReader(note.Title, lines)
}

func (a *App) openNoteTagPicker() {
	if len(a.notes.tags) == 0 {
		a.errf("Todavía no hay etiquetas: agrégalas al editar una nota")
		return
	}
	items := []pickItem{{Kind: "none", Label: "Todas las etiquetas", Detail: "sin filtro"}}
	for _, t := range a.notes.tags {
		items = append(items, pickItem{Kind: "tag", Value: t.Tag, Label: "#" + t.Tag, Detail: plural(t.Count, "nota", "notas")})
	}
	a.picker = newPicker("Filtrar notas por etiqueta", items, func(a *App, item pickItem) {
		a.notes.tag = item.Value
		a.notes.reload()
	})
	a.picker.layout(a.width, a.height-2)
}

func (a *App) openNoteProjectPicker() {
	items := []pickItem{
		{Kind: "none", Label: "Todas las notas", Detail: "sin filtro"},
		{Kind: "unassigned", Label: "Sin proyecto", Detail: "sueltas"},
	}
	for _, p := range a.projects.projects {
		items = append(items, pickItem{Kind: "project", ID: p.ID, Label: p.Name, Detail: p.ClientName})
	}
	a.picker = newPicker("Filtrar notas por proyecto", items, func(a *App, item pickItem) {
		n := a.notes
		n.projectFilter, n.unassigned, n.projectLabel = nil, false, ""
		switch item.Kind {
		case "unassigned":
			n.unassigned = true
		case "project":
			id := item.ID
			n.projectFilter, n.projectLabel = &id, item.Label
		}
		n.reload()
	})
	a.picker.layout(a.width, a.height-2)
}

// openNoteForm crea o edita una nota. El proyecto es opcional.
func (a *App) openNoteForm(note store.Note) {
	projects, _ := a.st.ListProjects()
	title, body := note.Title, note.Body
	// Etiquetas existentes: se eligen de una lista (sin errores de tipeo); las nuevas se escriben.
	known, _ := a.st.NoteTags()
	inUse := map[string]bool{}
	var tagOpts []huh.Option[string]
	for _, t := range known {
		inUse[t.Tag] = true
		tagOpts = append(tagOpts, huh.NewOption(fmt.Sprintf("#%s  (%d)", t.Tag, t.Count), t.Tag))
	}
	var picked, extra []string
	for _, t := range note.Tags {
		if inUse[t] {
			picked = append(picked, t)
		} else {
			extra = append(extra, t)
		}
	}
	tagsRaw := strings.Join(extra, " ")
	var projectID int64
	if note.ProjectID != nil {
		projectID = *note.ProjectID
	}

	opts := []huh.Option[int64]{huh.NewOption("Sin proyecto", int64(0))}
	for _, p := range projects {
		opts = append(opts, huh.NewOption(truncate(p.Name+" · "+p.ClientName, 60), p.ID))
	}

	fields := []huh.Field{
		huh.NewInput().Title("Título").Value(&title).Validate(huh.ValidateNotEmpty()),
		huh.NewSelect[int64]().Title("Proyecto").Options(opts...).Value(&projectID).Height(5).Description(searchHint(len(opts))),
	}
	newTagsTitle := "Etiquetas"
	if len(tagOpts) > 0 {
		fields = append(fields, huh.NewMultiSelect[string]().Title("Etiquetas").Options(tagOpts...).Value(&picked).
			Filterable(true).Height(min(len(tagOpts)+2, 6)).Description("espacio marca · / busca"))
		newTagsTitle = "Etiquetas nuevas"
	}
	fields = append(fields,
		huh.NewInput().Title(newTagsTitle).Placeholder("idea reunion pendiente-cliente").Value(&tagsRaw).
			Description("Separadas por espacio o coma"),
		huh.NewText().Title("Texto").Value(&body).Lines(8),
	)
	form := huh.NewForm(huh.NewGroup(fields...))

	a.openForm(form, func() error {
		out := note
		out.Title, out.Body = strings.TrimSpace(title), body
		out.Tags = store.NormalizeTags(append(append([]string{}, picked...), store.ParseTags(tagsRaw)...))
		out.ProjectID = nil
		if projectID != 0 {
			out.ProjectID = &projectID
		}
		var err error
		if out.ID == 0 {
			err = a.st.CreateNote(&out)
		} else {
			err = a.st.UpdateNote(&out)
		}
		if err == nil {
			a.notes.selectedID = out.ID
			a.setTab(tabNotes)
		}
		return err
	})
}
