package ui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/key"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tunapm/internal/dates"
	"tunapm/internal/money"
	"tunapm/internal/store"
)

type payRow struct {
	header  string
	color   lipgloss.TerminalColor
	totals  map[string]int64
	count   int
	payment *store.Payment
}

type Payments struct {
	st            store.Repo
	width, height int
	rows          []payRow
	cursor        int
	selectedID    int64
}

func newPayments(st store.Repo) *Payments {
	return &Payments{st: st, width: 100, height: 20}
}

func (p *Payments) layout(width, height int) {
	p.width, p.height = width, height
	p.reload()
}

func (p *Payments) reload() {
	all, err := p.st.ListPayments()
	if err != nil {
		return
	}
	today := dates.Today()
	limit := time.Now().AddDate(0, 0, 30).Format(dates.Layout)

	var overdue, upcoming, later, paid []store.Payment
	for _, pay := range all {
		switch {
		case pay.PaidDate != nil:
			paid = append(paid, pay)
		case pay.ScheduledDate < today:
			overdue = append(overdue, pay)
		case pay.ScheduledDate <= limit:
			upcoming = append(upcoming, pay)
		default:
			later = append(later, pay)
		}
	}
	sort.Slice(paid, func(i, j int) bool {
		return strOr(paid[i].PaidDate, "") > strOr(paid[j].PaidDate, "")
	})
	if len(paid) > 30 {
		paid = paid[:30]
	}

	p.rows = nil
	p.addGroup("VENCIDOS", colorRed, overdue)
	p.addGroup("PRÓXIMOS 30 DÍAS", colorYellow, upcoming)
	p.addGroup("MÁS ADELANTE", colorMuted, later)
	p.addGroup("PAGADOS RECIENTES", colorGreen, paid)

	p.cursor = 0
	if p.selectedID != 0 {
		for i, row := range p.rows {
			if row.payment != nil && row.payment.ID == p.selectedID {
				p.cursor = i
				break
			}
		}
	}
	if len(p.rows) == 0 {
		return
	}
	if p.cursor >= len(p.rows) || p.rows[p.cursor].payment == nil {
		if next := p.nextPayment(p.cursor, 1); next >= 0 {
			p.cursor = next
		} else {
			p.cursor = p.prevPayment(len(p.rows)-1, -1)
		}
	}
	if pay := p.selected(); pay != nil {
		p.selectedID = pay.ID
	}
}

func (p *Payments) addGroup(title string, color lipgloss.TerminalColor, payments []store.Payment) {
	if len(payments) == 0 {
		return
	}
	totals := make(map[string]int64)
	for _, pay := range payments {
		totals[pay.Currency] += pay.AmountCents
	}
	p.rows = append(p.rows, payRow{header: title, color: color, totals: totals, count: len(payments)})
	for i := range payments {
		p.rows = append(p.rows, payRow{payment: &payments[i]})
	}
}

func (p *Payments) nextPayment(from, delta int) int {
	for i := from; i >= 0 && i < len(p.rows); i += delta {
		if p.rows[i].payment != nil {
			return i
		}
	}
	return -1
}

func (p *Payments) prevPayment(from, delta int) int {
	for i := from; i >= 0 && i < len(p.rows); i += delta {
		if p.rows[i].payment != nil {
			return i
		}
	}
	return 0
}

func (p *Payments) selected() *store.Payment {
	if p.cursor < 0 || p.cursor >= len(p.rows) {
		return nil
	}
	return p.rows[p.cursor].payment
}

func (p *Payments) view() string {
	if len(p.rows) == 0 {
		return fitLines("\n"+mutedStyle.Render("  Sin cuotas programadas. ctrl+k y $ registra un cobro."), p.height)
	}

	today := dates.Today()
	var lines []string
	cursorLine := 0
	for i, row := range p.rows {
		if row.payment != nil {
			selected := i == p.cursor
			if selected {
				cursorLine = len(lines)
			}
			line := cursorMark(selected) + p.paymentLine(*row.payment, today)
			if selected {
				line = selectedStyle.Width(p.width).Render(line)
			}
			lines = append(lines, line)
			continue
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		label := lipgloss.NewStyle().Bold(true).Foreground(row.color).Render(row.header)
		extra := mutedStyle.Render(fmt.Sprintf("  %s  %s", plural(row.count, "cuota", "cuotas"), formatTotals(row.totals)))
		lines = append(lines, "  "+label+extra)
	}

	// Ventana centrada en el cursor (una línea reservada al encabezado de columnas).
	bodyH := max(p.height-1, 1)
	start := cursorLine - (bodyH-1)/2
	if start > len(lines)-bodyH {
		start = len(lines) - bodyH
	}
	if start < 0 {
		start = 0
	}
	end := min(start+bodyH, len(lines))
	window := append([]string{p.columnHeader()}, lines[start:end]...)
	return fitLines(strings.Join(window, "\n"), p.height)
}

type payColumns struct{ date, project, client, amount, status, note int }

func (p *Payments) columns() payColumns {
	c := payColumns{date: 7, amount: 12, status: 9}
	avail := p.width - 2 - c.date - c.amount - c.status - 4
	switch {
	case p.width >= 110:
		c.project = min(avail*2/5, 40)
		c.client = min(avail/5, 20)
		c.note = avail - c.project - c.client - 2
	case p.width >= 80:
		c.project = avail * 3 / 5
		c.client = avail - c.project - 1
	default:
		c.project = max(avail, 8)
	}
	return c
}

// columnHeader usa los mismos anchos que paymentLine para alinear los títulos.
func (p *Payments) columnHeader() string {
	c := p.columns()
	line := "  " + padRight("FECHA", c.date) + " " + padRight("PROYECTO", c.project)
	if c.client > 0 {
		line += " " + padRight("CLIENTE", c.client)
	}
	line += " " + padLeft("MONTO", c.amount) + "  " + padRight("ESTADO", c.status)
	if c.note > 0 {
		line += " " + "CONCEPTO"
	}
	return headStyle.Render(truncate(line, p.width))
}

func (p *Payments) paymentLine(pay store.Payment, today string) string {
	c := p.columns()
	date := pay.ScheduledDate
	if pay.PaidDate != nil {
		date = *pay.PaidDate
	}
	status := pay.StatusToday(today)
	dateText := mutedStyle.Render(padRight(dates.Human(date, today), c.date))
	if status == store.PaymentVencido {
		dateText = errorStyle.Render(padRight(dates.Human(date, today), c.date))
	}
	line := dateText + " " + padRight(truncate(pay.ProjectName, c.project), c.project)
	if c.client > 0 {
		line += " " + mutedStyle.Render(padRight(truncate(pay.ClientName, c.client), c.client))
	}
	line += " " + padLeft(money.Format(pay.AmountCents, pay.Currency), c.amount) + "  " +
		padRight(paymentStatusText(status), c.status)
	if c.note > 0 && pay.Notes != "" {
		line += " " + mutedStyle.Render(truncate(pay.Notes, c.note))
	}
	return truncate(line, p.width-2)
}

func formatTotals(totals map[string]int64) string {
	var parts []string
	for _, currency := range []string{store.CurrencyGTQ, store.CurrencyUSD} {
		if totals[currency] > 0 {
			parts = append(parts, money.Format(totals[currency], currency))
		}
	}
	if len(parts) == 0 {
		return money.Format(0, store.CurrencyGTQ)
	}
	return strings.Join(parts, " · ")
}

func (p *Payments) info() string {
	pending, overdue := 0, 0
	today := dates.Today()
	for _, row := range p.rows {
		if row.payment == nil {
			continue
		}
		if row.payment.PaidDate == nil {
			pending++
			if row.payment.ScheduledDate < today {
				overdue++
			}
		}
	}
	word := "vencidas"
	if overdue == 1 {
		word = "vencida"
	}
	return fmt.Sprintf("Cobros · %d por cobrar · %d %s", pending, overdue, word)
}

func (p *Payments) handleKey(a *App, msg tea.KeyMsg) {
	p.shortHelp() // actualiza qué atajos aplican
	switch {
	case key.Matches(msg, paymentKeys.Down):
		if next := p.nextPayment(p.cursor+1, 1); next >= 0 {
			p.cursor = next
		}
		p.syncSelection()
	case key.Matches(msg, paymentKeys.Up):
		if prev := p.prevPayment(p.cursor-1, -1); prev >= 0 {
			p.cursor = prev
		}
		p.syncSelection()
	case key.Matches(msg, paymentKeys.Pay):
		if pay := p.selected(); pay != nil {
			if pay.PaidDate != nil {
				a.toastf("Esa cuota ya está pagada (u para deshacer el pago)")
				return
			}
			a.openPaidDateForm(*pay)
		}
	case key.Matches(msg, paymentKeys.Unpay):
		if pay := p.selected(); pay != nil {
			if pay.PaidDate == nil {
				a.toastf("Esa cuota no está pagada")
				return
			}
			payment := *pay
			a.confirmAction(fmt.Sprintf("¿Marcar como NO pagada la cuota de %s?", money.Format(payment.AmountCents, payment.Currency)), func(a *App) {
				payment.PaidDate = nil
				if err := a.st.UpdatePayment(&payment); err != nil {
					a.errf("No se pudo deshacer el pago: %v", err)
					return
				}
				a.toastf("Cuota marcada como no pagada")
				a.reloadAll()
			})
		}
	}
}

func (p *Payments) syncSelection() {
	if pay := p.selected(); pay != nil {
		p.selectedID = pay.ID
	}
}

func padLeft(s string, width int) string {
	w := lipglossWidth(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}
