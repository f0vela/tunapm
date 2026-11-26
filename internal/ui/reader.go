package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Reader es una ventana de lectura (nota, tarea, evento o la ayuda) sobre un
// viewport de bubbles: j/k, espacio/b, d/u y re pág/av pág desplazan; g/G van al
// inicio y al final; esc, q o enter cierran.
type Reader struct {
	title string
	lines []string
	// render, si está, arma el contenido según el ancho (la ayuda se acomoda en columnas).
	render func(width int) string

	vp           viewport.Model
	lastW, lastH int
}

var readerKeys = struct{ Close, Top, Bottom key.Binding }{
	Close:  bind(k("esc", "q", "enter"), "esc", "cerrar"),
	Top:    bind(k("g", "home"), "g", "inicio"),
	Bottom: bind(k("G", "end"), "G", "fin"),
}

func newReader(title string, lines []string) *Reader {
	return &Reader{title: title, lines: lines, vp: viewport.New(0, 0)}
}

// handleKey devuelve true si hay que cerrar la ventana.
func (r *Reader) handleKey(msg tea.KeyMsg) bool {
	switch {
	case key.Matches(msg, readerKeys.Close):
		return true
	case key.Matches(msg, readerKeys.Top):
		r.vp.GotoTop()
	case key.Matches(msg, readerKeys.Bottom):
		r.vp.GotoBottom()
	default:
		r.vp, _ = r.vp.Update(msg)
	}
	return false
}

func (r *Reader) view(width, height int) string {
	maxWidth := max(min(width-8, 96), 24)
	inner := maxWidth - 6 // borde y relleno horizontal
	// Fuera del texto: borde y relleno (4), título y separación (2), pie (2).
	page := max(height-8, 3)

	if inner != r.lastW || page != r.lastH {
		var content string
		if r.render != nil {
			content = r.render(inner)
		} else {
			var out []string
			for _, line := range r.lines {
				for _, wrapped := range strings.Split(wrapLine(line, inner), "\n") {
					out = append(out, truncate(wrapped, inner))
				}
			}
			content = strings.Join(out, "\n")
		}
		offset := r.vp.YOffset
		r.vp.Width, r.vp.Height = inner, page
		r.vp.SetContent(content)
		r.vp.SetYOffset(offset)
		r.lastW, r.lastH = inner, page
	}

	body := r.vp.View()
	footer := renderHints("esc cerrar")
	total := r.vp.TotalLineCount()
	if total > r.vp.Height {
		r.vp.Height = page
		position := fmt.Sprintf("%d%%", int(r.vp.ScrollPercent()*100+0.5))
		switch {
		case r.vp.AtTop():
			position = "↓ hay más"
		case r.vp.AtBottom():
			position = "fin"
		}
		footer = spread(renderHints("j/k mover · espacio página · g/G inicio/fin · esc cerrar"), mutedStyle.Render(position), inner)
	} else {
		// Sin desplazamiento la ventana se ajusta al contenido.
		body = strings.Join(strings.Split(body, "\n")[:max(total, 1)], "\n")
	}
	return modalStyle.Width(maxWidth).Render(titleStyle.Render(truncate(r.title, inner)) + "\n\n" + body + "\n\n" + footer)
}
