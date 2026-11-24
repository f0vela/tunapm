package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"

	"tunapm/internal/dates"
	"tunapm/internal/store"
)

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}

func padRight(s string, width int) string {
	w := lipglossWidth(s)
	if w >= width {
		return truncate(s, width)
	}
	return s + strings.Repeat(" ", width-w)
}

func lipglossWidth(s string) int { return ansi.StringWidth(s) }

func spread(left, right string, width int) string {
	if width <= 0 {
		return ""
	}
	lw, rw := lipglossWidth(left), lipglossWidth(right)
	if lw+rw+1 > width {
		// La información de la izquierda manda; se recorta la ayuda de la derecha.
		avail := width - lw - 1
		if avail <= 0 {
			return truncate(left, width)
		}
		right = truncate(right, avail)
		rw = lipglossWidth(right)
	}
	gap := width - lw - rw
	if gap < 0 {
		gap = 0
	}
	return left + strings.Repeat(" ", gap) + right
}

// fuzzyIndexes aplica búsqueda difusa y devuelve los índices en orden de
// coincidencia; sin consulta devuelve todos.
func fuzzyIndexes(query string, labels []string) []int {
	if strings.TrimSpace(query) == "" {
		all := make([]int, len(labels))
		for i := range labels {
			all[i] = i
		}
		return all
	}
	matches := fuzzy.Find(query, labels)
	out := make([]int, len(matches))
	for i, m := range matches {
		out[i] = m.Index
	}
	return out
}

func parseOptionalDate(raw string) (*string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	date, err := dates.Parse(raw, time.Now())
	if err != nil {
		return nil, err
	}
	return &date, nil
}

// parseDateFor resuelve fechas relativas ("hoy", "+3d", "viernes").
func parseDateFor(raw string) (string, error) {
	return dates.Parse(raw, time.Now())
}

func strOr(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}

func ptr(s string) *string { return &s }

func plural(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// resolveProject busca un proyecto por id numérico o por nombre difuso.
// Escribe @web-acme o @Acme; los guiones cuentan como espacios.
func resolveProject(projects []store.Project, query string) *store.Project {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}
	if id, err := strconv.ParseInt(q, 10, 64); err == nil {
		for i := range projects {
			if projects[i].ID == id {
				return &projects[i]
			}
		}
	}
	normalized := normalizeTarget(q)
	for i := range projects {
		if normalizeTarget(projects[i].Name) == normalized && normalized != "" {
			return &projects[i]
		}
	}
	labels := make([]string, len(projects))
	for i, p := range projects {
		labels[i] = p.Name + " " + p.ClientName
	}
	hits := fuzzyIndexes(normalized, labels)
	if len(hits) == 0 {
		return nil
	}
	return &projects[hits[0]]
}

func resolveClient(clients []store.Client, query string) *store.Client {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}
	if id, err := strconv.ParseInt(q, 10, 64); err == nil {
		for i := range clients {
			if clients[i].ID == id {
				return &clients[i]
			}
		}
	}
	normalized := normalizeTarget(q)
	for i := range clients {
		if normalizeTarget(clients[i].Name) == normalized && normalized != "" {
			return &clients[i]
		}
	}
	labels := make([]string, len(clients))
	for i, c := range clients {
		labels[i] = c.Name + " " + c.Company
	}
	hits := fuzzyIndexes(normalized, labels)
	if len(hits) == 0 {
		return nil
	}
	return &clients[hits[0]]
}

func normalizeTarget(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), "-", " "))
}

// masterDetail coloca lista y detalle lado a lado; en terminales angostas los
// apila para que el detalle no se recorte.
func masterDetail(width, height, listWidth int, list func(w, h int) string, detail func(w, h int) string) string {
	if width < 96 {
		listH := max(height*55/100, 4)
		detailH := max(height-listH-1, 1)
		return list(width, listH) + "\n" + faintStyle.Render(strings.Repeat("─", width)) + "\n" +
			fitLines(detail(width, detailH), detailH)
	}
	return joinColumns(list(listWidth, height), vrule(height), fitLines(detail(width-listWidth-3, height), height))
}

func joinColumns(cols ...string) string {
	split := make([][]string, len(cols))
	rows := 0
	for i, c := range cols {
		split[i] = strings.Split(c, "\n")
		rows = max(rows, len(split[i]))
	}
	widths := make([]int, len(cols))
	for i := range split {
		for _, line := range split[i] {
			widths[i] = max(widths[i], lipglossWidth(line))
		}
	}
	var out []string
	for r := 0; r < rows; r++ {
		var b strings.Builder
		for i := range split {
			cell := ""
			if r < len(split[i]) {
				cell = split[i][r]
			}
			if i < len(split)-1 {
				cell += strings.Repeat(" ", widths[i]-lipglossWidth(cell))
			}
			b.WriteString(cell)
		}
		out = append(out, b.String())
	}
	return strings.Join(out, "\n")
}

// fitLines recorta o rellena un bloque a una altura exacta.
func fitLines(block string, height int) string {
	lines := strings.Split(block, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:height], "\n")
}

// clipLines aplana líneas con saltos internos, las recorta al ancho y limita la altura.
func clipLines(lines []string, width, height int) string {
	var flat []string
	for _, l := range lines {
		flat = append(flat, strings.Split(l, "\n")...)
	}
	if len(flat) > height {
		flat = flat[:height]
	}
	for i := range flat {
		flat[i] = truncate(flat[i], width)
	}
	return strings.Join(flat, "\n")
}
