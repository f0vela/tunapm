// Package agenda arma el calendario: junta en una sola lista, por fecha, las
// tareas con fecha límite, las cuotas por cobrar, el inicio y la entrega de
// los proyectos y los eventos. No guarda nada: todo sale de las tablas existentes.
package agenda

import (
	"sort"
	"time"

	"tunapm/internal/money"
	"tunapm/internal/store"
)

const (
	TypeEvent        = "event"
	TypeTask         = "task"
	TypePayment      = "payment"
	TypeProjectStart = "project_start"
	TypeProjectDue   = "project_due"
)

// Item es una entrada del calendario. ID es el de la fila de origen según Type
// (para project_start y project_due, el id del proyecto).
type Item struct {
	Date        string `json:"date"`
	Time        string `json:"time"` // "" = todo el día
	Type        string `json:"type"`
	Kind        string `json:"kind"` // tipo de evento; vacío para lo demás
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Detail      string `json:"detail"` // proyecto y/o cliente
	ProjectID   int64  `json:"project_id"`
	Done        bool   `json:"done"`
	Overdue     bool   `json:"overdue"`
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
}

// Label es la etiqueta corta del tipo de entrada.
func (i Item) Label() string {
	switch i.Type {
	case TypeTask:
		return "tarea"
	case TypePayment:
		return "cobro"
	case TypeProjectStart:
		return "inicio"
	case TypeProjectDue:
		return "entrega"
	}
	if l, ok := store.EventKindLabels[i.Kind]; ok {
		return l
	}
	return "evento"
}

// Text es el título para mostrar; en un cobro incluye el monto.
func (i Item) Text() string {
	if i.Type == TypePayment {
		amount := money.Format(i.AmountCents, i.Currency)
		if i.Title == "" {
			return amount
		}
		return amount + " " + i.Title
	}
	return i.Title
}

var typeOrder = map[string]int{TypeEvent: 0, TypeProjectDue: 1, TypeProjectStart: 2, TypePayment: 3, TypeTask: 4}

func join(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + " · " + b
}

// Build devuelve las entradas entre from y to (YYYY-MM-DD, inclusive), ordenadas
// por fecha y hora. today decide qué está vencido.
func Build(repo store.Repo, from, to, today string) ([]Item, error) {
	in := func(d string) bool { return d >= from && d <= to }
	items := []Item{}

	events, err := repo.ListEvents(from, to)
	if err != nil {
		return nil, err
	}
	for _, e := range events {
		it := Item{Date: e.Date, Type: TypeEvent, Kind: e.Kind, ID: e.ID, Title: e.Title,
			Detail: join(e.ProjectName, e.ClientName)}
		if e.Time != nil {
			it.Time = *e.Time
		}
		if e.ProjectID != nil {
			it.ProjectID = *e.ProjectID
		}
		items = append(items, it)
	}

	tasks, err := repo.ListTasks(store.TaskFilter{Mode: store.FilterAll})
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		if t.DueDate == nil || !in(*t.DueDate) {
			continue
		}
		done := t.Status == store.TaskHecho
		items = append(items, Item{Date: *t.DueDate, Type: TypeTask, ID: t.ID, Title: t.Title,
			Detail: join(t.ProjectName, t.ClientName), ProjectID: t.ProjectID, Done: done, Overdue: !done && *t.DueDate < today})
	}

	payments, err := repo.ListPayments()
	if err != nil {
		return nil, err
	}
	for _, p := range payments {
		// Solo lo que falta cobrar: una cuota pagada ya no es un pendiente del calendario.
		if p.PaidDate != nil || !in(p.ScheduledDate) {
			continue
		}
		items = append(items, Item{Date: p.ScheduledDate, Type: TypePayment, ID: p.ID, Title: p.Notes,
			Detail: join(p.ProjectName, p.ClientName), ProjectID: p.ProjectID, Overdue: p.ScheduledDate < today,
			AmountCents: p.AmountCents, Currency: p.Currency})
	}

	projects, err := repo.ListProjects()
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if p.Status == store.StatusCancelado {
			continue
		}
		done := p.Status == store.StatusCompletado
		if p.StartDate != nil && in(*p.StartDate) {
			items = append(items, Item{Date: *p.StartDate, Type: TypeProjectStart, ID: p.ID, Title: p.Name,
				Detail: p.ClientName, ProjectID: p.ID, Done: done || *p.StartDate < today})
		}
		if p.DueDate != nil && in(*p.DueDate) {
			items = append(items, Item{Date: *p.DueDate, Type: TypeProjectDue, ID: p.ID, Title: p.Name,
				Detail: p.ClientName, ProjectID: p.ID, Done: done, Overdue: !done && *p.DueDate < today})
		}
	}

	sort.SliceStable(items, func(a, b int) bool {
		x, y := items[a], items[b]
		if x.Date != y.Date {
			return x.Date < y.Date
		}
		// Sin hora primero; después por hora.
		if (x.Time == "") != (y.Time == "") {
			return x.Time == ""
		}
		if x.Time != y.Time {
			return x.Time < y.Time
		}
		if typeOrder[x.Type] != typeOrder[y.Type] {
			return typeOrder[x.Type] < typeOrder[y.Type]
		}
		return x.ID < y.ID
	})
	return items, nil
}

// ByDate agrupa las entradas por fecha.
func ByDate(items []Item) map[string][]Item {
	out := map[string][]Item{}
	for _, it := range items {
		out[it.Date] = append(out[it.Date], it)
	}
	return out
}

// ---------- fechas del calendario (semana de lunes a domingo) ----------

const layout = "2006-01-02"

func parse(date string) time.Time {
	t, err := time.Parse(layout, date)
	if err != nil {
		return time.Now()
	}
	return t
}

// Add suma días a una fecha YYYY-MM-DD.
func Add(date string, days int) string { return parse(date).AddDate(0, 0, days).Format(layout) }

// AddMonths suma meses; si el día no existe en el mes destino usa el último (31 ene + 1 mes = 28 feb).
func AddMonths(date string, months int) string {
	t := parse(date)
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, months, 0)
	last := first.AddDate(0, 1, -1).Day()
	day := t.Day()
	if day > last {
		day = last
	}
	return time.Date(first.Year(), first.Month(), day, 0, 0, 0, 0, time.UTC).Format(layout)
}

// WeekStart es el lunes de la semana de date.
func WeekStart(date string) string {
	t := parse(date)
	offset := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -offset).Format(layout)
}

// Week devuelve los 7 días (lunes a domingo) de la semana de date.
func Week(date string) []string {
	start := WeekStart(date)
	days := make([]string, 7)
	for i := range days {
		days[i] = Add(start, i)
	}
	return days
}

// MonthBounds devuelve el primer y el último día del mes de date.
func MonthBounds(date string) (first, last string) {
	t := parse(date)
	f := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	return f.Format(layout), f.AddDate(0, 1, -1).Format(layout)
}

// MonthGrid devuelve las semanas completas (lunes a domingo) que cubren el mes de date.
func MonthGrid(date string) [][]string {
	first, last := MonthBounds(date)
	var weeks [][]string
	for day := WeekStart(first); day <= last; day = Add(day, 7) {
		weeks = append(weeks, Week(day))
	}
	return weeks
}

var monthNames = []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto",
	"septiembre", "octubre", "noviembre", "diciembre"}
var weekdayNames = []string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"}

// WeekdayShort son las cabeceras de la cuadrícula, de lunes a domingo.
var WeekdayShort = []string{"lu", "ma", "mi", "ju", "vi", "sá", "do"}

// MonthTitle es "noviembre 2026".
func MonthTitle(date string) string {
	t := parse(date)
	return monthNames[t.Month()-1] + " " + t.Format("2006")
}

// DayTitle es "miércoles 25 de noviembre".
func DayTitle(date string) string {
	t := parse(date)
	return weekdayNames[t.Weekday()] + " " + t.Format("2") + " de " + monthNames[t.Month()-1]
}

// WeekTitle es "23 - 29 de noviembre" o "30 nov - 6 dic" si cruza de mes.
func WeekTitle(date string) string {
	days := Week(date)
	a, b := parse(days[0]), parse(days[6])
	if a.Month() == b.Month() {
		return a.Format("2") + " - " + b.Format("2") + " de " + monthNames[b.Month()-1] + " " + b.Format("2006")
	}
	return a.Format("2") + " " + monthNames[a.Month()-1][:3] + " - " + b.Format("2") + " " + monthNames[b.Month()-1][:3] + " " + b.Format("2006")
}

// DayNumber es el día del mes sin cero inicial.
func DayNumber(date string) string { return parse(date).Format("2") }

// SameMonth indica si dos fechas caen en el mismo mes.
func SameMonth(a, b string) bool { return len(a) >= 7 && len(b) >= 7 && a[:7] == b[:7] }
