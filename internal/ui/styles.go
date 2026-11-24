package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"tunapm/internal/store"
)

func adaptive(light, dark string) lipgloss.TerminalColor {
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

// Colores activos. applyTheme los reemplaza y reconstruye los estilos.
var (
	colorAccent, colorMuted, colorFaint, colorText, colorStrong lipgloss.TerminalColor
	colorRed, colorYellow, colorGreen, colorBlue, colorOrange   lipgloss.TerminalColor
	colorBarBg, colorSelBg, colorOnAcc                          lipgloss.TerminalColor

	tabActiveStyle, tabInactiveStyle, appNameStyle, statusBarStyle, modalStyle lipgloss.Style
	sectionStyle, titleStyle, headStyle, mutedStyle, faintStyle                lipgloss.Style
	errorStyle, successStyle, warnStyle, keyStyle, selectedStyle               lipgloss.Style

	taskStatusColors, projectStatusColors, paymentStatusColors map[string]lipgloss.TerminalColor
	priorityColors                                             []lipgloss.TerminalColor
)

func init() { applyTheme(themes[0]) }

// buildStyles deriva todos los estilos de los colores activos: un solo acento
// para navegación y selección; el resto son colores semánticos.
func buildStyles() {
	tabActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(colorOnAcc).Background(colorAccent).Padding(0, 1)
	tabInactiveStyle = lipgloss.NewStyle().Foreground(colorMuted).Background(colorBarBg).Padding(0, 1)
	appNameStyle = lipgloss.NewStyle().Foreground(colorAccent).Background(colorBarBg).Bold(true).Padding(0, 1)
	statusBarStyle = lipgloss.NewStyle().Background(colorBarBg).Foreground(colorText).Padding(0, 1)
	modalStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorAccent).Padding(1, 2)

	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorStrong)
	headStyle = lipgloss.NewStyle().Foreground(colorMuted)
	mutedStyle = lipgloss.NewStyle().Foreground(colorMuted)
	faintStyle = lipgloss.NewStyle().Foreground(colorFaint)
	errorStyle = lipgloss.NewStyle().Foreground(colorRed)
	successStyle = lipgloss.NewStyle().Foreground(colorGreen)
	warnStyle = lipgloss.NewStyle().Foreground(colorYellow)
	keyStyle = lipgloss.NewStyle().Bold(true).Foreground(colorStrong)
	selectedStyle = lipgloss.NewStyle().Background(colorSelBg).Foreground(colorStrong)

	taskStatusColors = map[string]lipgloss.TerminalColor{
		store.TaskPendiente: colorMuted,
		store.TaskEnCurso:   colorBlue,
		store.TaskHecho:     colorGreen,
	}
	projectStatusColors = map[string]lipgloss.TerminalColor{
		store.StatusProspecto:  colorMuted,
		store.StatusActivo:     colorGreen,
		store.StatusPausado:    colorYellow,
		store.StatusCompletado: colorBlue,
		store.StatusCancelado:  colorRed,
	}
	paymentStatusColors = map[string]lipgloss.TerminalColor{
		store.PaymentPendiente: colorYellow,
		store.PaymentVencido:   colorRed,
		store.PaymentPagado:    colorGreen,
	}
	priorityColors = []lipgloss.TerminalColor{colorMuted, colorAccent, colorOrange, colorRed}
}

// cursorMark es el marcador de la fila seleccionada, igual en todas las listas.
func cursorMark(selected bool) string {
	if selected {
		return lipgloss.NewStyle().Foreground(colorAccent).Render("▌") + " "
	}
	return "  "
}

// renderHints convierte "n nueva · e editar" en pares tecla/acción donde la
// tecla resalta y la acción queda atenuada.
func renderHints(hints string) string {
	var parts []string
	for _, hint := range strings.Split(hints, " · ") {
		key, desc, found := strings.Cut(hint, " ")
		if !found {
			parts = append(parts, keyStyle.Render(key))
			continue
		}
		parts = append(parts, keyStyle.Render(key)+" "+mutedStyle.Render(desc))
	}
	return strings.Join(parts, "  ")
}

// field alinea una etiqueta atenuada con su valor en los paneles de detalle.
func field(label, value string) string {
	return mutedStyle.Render(padRight(label, 14)) + value
}

// vrule es el separador vertical entre lista y detalle.
func vrule(height int) string {
	return faintStyle.Render(strings.TrimRight(strings.Repeat(" │ \n", height), "\n"))
}

var taskStatusLabels = map[string]string{
	store.TaskPendiente: "pendiente",
	store.TaskEnCurso:   "en curso",
	store.TaskHecho:     "hecha",
}

var taskStatusIcons = map[string]string{
	store.TaskPendiente: "○",
	store.TaskEnCurso:   "◐",
	store.TaskHecho:     "●",
}

var projectStatusLabels = map[string]string{
	store.StatusProspecto:  "prospecto",
	store.StatusActivo:     "activo",
	store.StatusPausado:    "pausado",
	store.StatusCompletado: "completado",
	store.StatusCancelado:  "cancelado",
}

var priorityLabels = []string{"baja", "media", "alta", "urgente"}

var paymentStatusLabels = map[string]string{
	store.PaymentPendiente: "pendiente",
	store.PaymentVencido:   "vencido",
	store.PaymentPagado:    "pagado",
}

func statusIcon(status string) string {
	icon, ok := taskStatusIcons[status]
	if !ok {
		icon = "○"
	}
	return lipgloss.NewStyle().Foreground(taskStatusColors[status]).Render(icon)
}

func statusText(status string) string {
	return lipgloss.NewStyle().Foreground(taskStatusColors[status]).Render(taskStatusLabels[status])
}

func priorityText(p int) string {
	if p < 0 || p >= len(priorityLabels) {
		p = 1
	}
	return lipgloss.NewStyle().Foreground(priorityColors[p]).Render(priorityLabels[p])
}

func projectStatusText(status string) string {
	return lipgloss.NewStyle().Foreground(projectStatusColors[status]).Render(projectStatusLabels[status])
}

func paymentStatusText(status string) string {
	return lipgloss.NewStyle().Foreground(paymentStatusColors[status]).Render(paymentStatusLabels[status])
}

// dueStyle resalta vencidas y las que vencen hoy o mañana.
func dueStyle(date string, today string) lipgloss.Style {
	tomorrow, _ := parseDateFor("+1d")
	switch {
	case date < today:
		return lipgloss.NewStyle().Foreground(colorRed)
	case date <= tomorrow:
		return lipgloss.NewStyle().Foreground(colorYellow)
	default:
		return lipgloss.NewStyle().Foreground(colorMuted)
	}
}

func progressBar(done, total, width int) string {
	if total <= 0 {
		return mutedStyle.Render(strings.Repeat("░", width))
	}
	filled := done * width / total
	if filled > width {
		filled = width
	}
	return lipgloss.NewStyle().Foreground(colorGreen).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(colorFaint).Render(strings.Repeat("░", width-filled))
}

func toastColor(err bool) lipgloss.Style {
	if err {
		return errorStyle
	}
	return successStyle
}
