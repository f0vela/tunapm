package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"
)

// La ayuda (?) se arma con los mismos key.Binding que usan las vistas, así no
// puede quedar desactualizada respecto de las teclas reales.

var helpLeft = []helpGroup{globalHelp, taskHelp, noteHelp, clientHelp, paymentHelp}

var quickAddHelp = helpGroup{"Creación rápida en la paleta", []key.Binding{
	bind(nil, "t", "Diseñar logo @proyecto !alta ^viernes"),
	bind(nil, "c", "Juan Pérez #Empresa"),
	bind(nil, "p", "Sitio web @cliente $5000 usd ^20/12"),
	bind(nil, "$", "2500 @proyecto ^15/12"),
	bind(nil, "n", "Idea para el home @proyecto #idea"),
	bind(nil, "e", "Reunión @proyecto ^viernes 15:30"),
}}

var helpRight = []helpGroup{projectHelp, calendarHelp, quickAddHelp}

func helpColumn(groups []helpGroup, width int) string {
	var lines []string
	for i, g := range groups {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, sectionStyle.Render(g.title))
		for _, b := range g.bindings {
			h := b.Help()
			lines = append(lines, truncate(keyStyle.Render(padRight(h.Key, 10))+" "+mutedStyle.Render(h.Desc), width))
		}
	}
	return strings.Join(lines, "\n")
}

// helpReader abre la ayuda en una ventana desplazable; en terminales anchas va en dos columnas.
func (a *App) helpReader() *Reader {
	title := "Atajos de teclado"
	if Version != "" {
		title += "  " + mutedStyle.Render("· tuna "+Version)
	}
	r := newReader(title, nil)
	r.render = func(width int) string {
		if width >= 76 {
			left := helpColumn(helpLeft, width/2)
			leftW := 0
			for _, line := range strings.Split(left, "\n") {
				leftW = max(leftW, lipglossWidth(line))
			}
			return joinColumns(left, "    ", helpColumn(helpRight, width-leftW-4))
		}
		return helpColumn(append(append([]helpGroup{}, helpLeft...), helpRight...), width)
	}
	return r
}
