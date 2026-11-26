package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Selector de vistas (Tareas, Proyectos, ...). Hay dos presentaciones:
//
//   - pestañas con borde (3 líneas): la activa se abre por abajo y se funde con
//     una regla que cruza la pantalla; solo si hay espacio de sobra;
//   - barra plana (1 línea): la activa en bloque de color, como siempre.
//
// La regla de elección depende solo del tamaño de la ventana (no del contenido
// de la insignia de sincronización) para que la altura no salte entre cuadros.

var tabLabels = []string{"1 Tareas", "2 Proyectos", "3 Clientes", "4 Cobros", "5 Notas", "6 Calendario"}

// Con borde, las seis pestañas ocupan 80 columnas; el resto queda para la insignia.
const (
	borderTabsMinWidth  = 100
	borderTabsMinHeight = 28
)

func (a *App) useBorderTabs() bool {
	return a.width >= borderTabsMinWidth && a.height >= borderTabsMinHeight
}

// headerHeight son las líneas que ocupa el selector de vistas.
func (a *App) headerHeight() int {
	if a.useBorderTabs() {
		return 3
	}
	return 1
}

// bodyHeight es la altura del contenido: ventana menos selector y barra de estado.
func (a *App) bodyHeight() int { return a.height - a.headerHeight() - 1 }

func (a *App) tabsView() string {
	if a.useBorderTabs() {
		return a.borderTabsView()
	}
	return a.flatTabsView()
}

func (a *App) flatTabsView() string {
	var b strings.Builder
	for i, label := range tabLabels {
		style := tabInactiveStyle
		if i == a.tab {
			style = tabActiveStyle
		}
		b.WriteString(style.Render(label))
	}
	left := b.String()
	right := a.sourceBadge(true) + appNameStyle.Render("tunapm")
	gap := max(a.width-lipglossWidth(left)-lipglossWidth(right), 0)
	return left + lipgloss.NewStyle().Background(colorBarBg).Render(strings.Repeat(" ", gap)) + right
}

// Bordes de las pestañas: la inactiva cierra por abajo con una unión "┴" a la
// regla; la activa deja el fondo abierto ("┘ └") para fundirse con ella.
func tabBorders() (active, inactive lipgloss.Border) {
	inactive = lipgloss.RoundedBorder()
	inactive.BottomLeft, inactive.Bottom, inactive.BottomRight = "┴", "─", "┴"
	active = lipgloss.RoundedBorder()
	active.BottomLeft, active.Bottom, active.BottomRight = "┘", " ", "└"
	return active, inactive
}

func (a *App) borderTabsView() string {
	activeBorder, inactiveBorder := tabBorders()
	rule := lipgloss.NewStyle().Foreground(colorFaint)

	var tabs []string
	for i, label := range tabLabels {
		style := lipgloss.NewStyle().Padding(0, 1)
		active, inactive := activeBorder, inactiveBorder
		if i == 0 { // la regla empieza en esta pestaña: no hay nada a su izquierda con qué unirse
			active.BottomLeft, inactive.BottomLeft = "│", "├"
		}
		if i == a.tab {
			style = style.Border(active).BorderForeground(colorAccent).Bold(true).Foreground(colorStrong)
		} else {
			style = style.Border(inactive).BorderForeground(colorFaint).Foreground(colorMuted)
		}
		tabs = append(tabs, style.Render(label))
	}
	block := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)
	rows := strings.Split(block, "\n") // [borde superior, etiquetas, regla]
	used := lipglossWidth(rows[0])

	// Derecha: insignia y nombre sobre la fila de etiquetas; la regla sigue por debajo.
	right := a.sourceBadge(false) + lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render("tunapm") + " "
	room := max(a.width-used, 0)
	right = truncate(right, room)
	gap := max(room-lipglossWidth(right), 0)

	top := rows[0] + strings.Repeat(" ", room)
	mid := rows[1] + strings.Repeat(" ", gap) + right
	bottom := rows[2] + rule.Render(strings.Repeat("─", room))
	return top + "\n" + mid + "\n" + bottom
}
