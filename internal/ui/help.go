package ui

import "strings"

type helpSection struct {
	title string
	keys  [][2]string
}

var helpLeft = []helpSection{
	{"Global", [][2]string{
		{"1-6", "cambiar de vista"},
		{"ctrl+k  :", "paleta de comandos"},
		{"j/k  ↑/↓", "mover la selección"},
		{"esc", "limpiar filtros, cerrar ventanas"},
		{"?  T", "esta ayuda, tema de colores"},
		{"ctrl+c", "salir"},
	}},
	{"Tareas", [][2]string{
		{"o  a  h", "abiertas, todas, hechas"},
		{"n  espacio", "nueva, marcar hecha"},
		{"s  +  -", "estado, subir o bajar prioridad"},
		{"e  d", "editar, borrar"},
		{"enter  g", "ver detalle, ir al proyecto"},
		{"/  f", "buscar, filtrar"},
	}},
	{"Notas", [][2]string{
		{"n  e  d", "nueva, editar, borrar"},
		{"t  f", "filtrar por etiqueta o proyecto"},
		{"/  enter", "buscar, leer completa"},
	}},
}

var helpRight = []helpSection{
	{"Proyectos", [][2]string{
		{"n  e  d", "nuevo, editar, borrar"},
		{"S", "cambiar estado"},
		{"c  x", "agregar cuota, pagar la siguiente"},
		{"enter  o", "ver sus tareas, ver sus notas"},
	}},
	{"Clientes", [][2]string{
		{"n  e  d", "nuevo, editar, borrar"},
		{"enter", "ver sus tareas"},
	}},
	{"Cobros", [][2]string{
		{"x", "marcar la cuota como pagada"},
	}},
	{"Calendario", [][2]string{
		{"←→  ↑↓", "día, semana"},
		{"[ ]  t", "mes anterior o siguiente, hoy"},
		{"v  tab", "vista mes/semana/día, otra entrada"},
		{"n  e  d", "nuevo evento, editar, borrar"},
		{"enter", "abrir la tarea, cobro o proyecto"},
	}},
	{"Creación rápida en la paleta", [][2]string{
		{"t", "Diseñar logo @proyecto !alta ^viernes"},
		{"c", "Juan Pérez #Empresa"},
		{"p", "Sitio web @cliente $5000 usd ^20/12"},
		{"$", "2500 @proyecto ^15/12"},
		{"n", "Idea para el home @proyecto #idea"},
		{"e", "Reunión @proyecto ^viernes 15:30"},
	}},
}

func helpColumn(sections []helpSection, width int) string {
	var lines []string
	for i, section := range sections {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, sectionStyle.Render(section.title))
		for _, k := range section.keys {
			lines = append(lines, truncate(keyStyle.Render(padRight(k[0], 10))+" "+mutedStyle.Render(k[1]), width))
		}
	}
	return strings.Join(lines, "\n")
}

func (a *App) helpView() string {
	width := max(min(a.width-8, 104), 30)
	inner := width - 6
	var body string
	if inner >= 76 {
		leftW := 44
		body = joinColumns(helpColumn(helpLeft, leftW), "   ", helpColumn(helpRight, inner-leftW-3))
	} else {
		body = helpColumn(append(append([]helpSection{}, helpLeft...), helpRight...), inner)
	}
	lines := strings.Split(body, "\n")
	footer := renderHints("esc cerrar")
	// Si no cabe, se desplaza con j/k en vez de cortarse.
	if page := max(a.height-10, 3); len(lines) > page {
		a.helpOff = max(min(a.helpOff, len(lines)-page), 0)
		more := "↓ hay más"
		if a.helpOff == len(lines)-page {
			more = "fin"
		}
		lines = lines[a.helpOff : a.helpOff+page]
		footer = spread(renderHints("j/k desplaza · esc cerrar"), mutedStyle.Render(more), inner)
	} else {
		a.helpOff = 0
	}
	content := titleStyle.Render("Atajos de teclado") + "\n\n" + strings.Join(lines, "\n") + "\n\n" + footer
	return modalStyle.Width(width).Render(content)
}
