package quickadd

import (
	"errors"
	"fmt"
	"strings"

	"tunapm/internal/money"
)

// Kind identifica la entidad que se crea con la sintaxis rápida.
type Kind string

const (
	KindTask    Kind = "t"
	KindClient  Kind = "c"
	KindProject Kind = "p"
	KindPayment Kind = "$"
	KindNote    Kind = "n"
	KindEvent   Kind = "e"
)

var (
	ErrNotQuickAdd     = errors.New("no es un comando rápido")
	ErrMissingTitle    = errors.New("falta el título")
	ErrMissingAmount   = errors.New("falta el monto")
	ErrInvalidAmount   = errors.New("monto inválido")
	ErrInvalidPriority = errors.New("prioridad inválida")
	ErrInvalidCompany  = errors.New("empresa inválida")
	ErrInvalidTarget   = errors.New("destino inválido")
)

// Draft es lo que se creará. Target es el proyecto (tareas y cuotas) o el
// cliente (proyectos) escrito tras @, sin resolver todavía; la UI lo busca
// con fuzzy y avisa si no existe.
type Draft struct {
	Kind        Kind
	Title       string
	Target      string
	Company     string
	Tags        string // etiquetas de una nota, separadas por espacio
	AmountCents int64
	Currency    string
	Priority    int    // -1 si no se indicó
	Date        string // texto de ^, sin parsear (ver internal/dates)
	Time        string // hora de un evento (HH:MM), sin validar
}

// Parse interpreta:
//
//	t Diseñar logo @web-acme !alta ^viernes
//	c Juan Pérez #Acme
//	p Sitio web @Acme $5000 usd ^2026-12-20
//	$ 2500 @web-acme ^15/12
//	n Idea para el home @web-acme #idea #diseño
//	e Reunión de avance @web-acme ^viernes 15:30 #reunion
func Parse(input string) (Draft, error) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return Draft{}, ErrNotQuickAdd
	}

	kind := Kind(fields[0])
	switch kind {
	case KindTask, KindClient, KindProject, KindPayment, KindNote, KindEvent:
	default:
		return Draft{}, ErrNotQuickAdd
	}

	d := Draft{Kind: kind, Currency: "GTQ", Priority: -1}

	var titleWords []string
	for _, field := range fields[1:] {
		switch {
		case strings.HasPrefix(field, "#") && kind == KindNote:
			// En una nota cada #palabra es una etiqueta.
			if tag := strings.TrimLeft(field, "#"); tag != "" {
				d.Tags = strings.TrimSpace(d.Tags + " " + tag)
			}
		case kind == KindNote && !strings.HasPrefix(field, "@"):
			// El texto de una nota es libre: ^, ! o "usd" no son sintaxis aquí.
			titleWords = append(titleWords, field)
		case kind == KindEvent && strings.HasPrefix(field, "#"):
			// En un evento #palabra es el tipo (reunion, llamada, presentacion, lanzamiento).
			if tag := strings.TrimLeft(field, "#"); tag != "" {
				d.Tags = tag
			}
		case kind == KindEvent && d.Time == "" && looksLikeTime(field):
			d.Time = field
		case strings.HasPrefix(field, "^"):
			if len(field) == 1 {
				return Draft{}, fmt.Errorf("fecha vacía")
			}
			d.Date = field[1:]
		case strings.HasPrefix(field, "!"):
			p, ok := priorities[strings.ToLower(field[1:])]
			if !ok {
				return Draft{}, fmt.Errorf("%w: %q", ErrInvalidPriority, field)
			}
			d.Priority = p
		case strings.HasPrefix(field, "#"):
			if len(field) == 1 {
				return Draft{}, ErrInvalidCompany
			}
			d.Company = field[1:]
		case strings.HasPrefix(field, "@"):
			if len(field) == 1 {
				return Draft{}, ErrInvalidTarget
			}
			d.Target = field[1:]
		case strings.HasPrefix(field, "$") && kind == KindProject:
			cents, err := money.Parse(field[1:])
			if err != nil {
				return Draft{}, fmt.Errorf("%w: %q", ErrInvalidAmount, field)
			}
			d.AmountCents = cents
		case kind == KindPayment && d.AmountCents == 0:
			cents, err := money.Parse(field)
			if err != nil {
				return Draft{}, fmt.Errorf("%w: %q", ErrInvalidAmount, field)
			}
			d.AmountCents = cents
		case strings.EqualFold(field, "usd"):
			d.Currency = "USD"
		case strings.EqualFold(field, "gtq") || strings.EqualFold(field, "q"):
			d.Currency = "GTQ"
		default:
			titleWords = append(titleWords, field)
		}
	}
	d.Title = strings.Join(titleWords, " ")

	if d.Kind == KindPayment && d.AmountCents <= 0 {
		return d, ErrMissingAmount
	}
	if d.Title == "" && d.Kind != KindPayment {
		return d, ErrMissingTitle
	}
	return d, nil
}

var priorities = map[string]int{
	"baja": 0, "media": 1, "alta": 2, "urgente": 3,
}

// looksLikeTime reconoce "9:30" o "15:00" (no valida el rango).
func looksLikeTime(s string) bool {
	h, m, ok := strings.Cut(s, ":")
	if !ok || len(h) == 0 || len(h) > 2 || len(m) != 2 {
		return false
	}
	for _, r := range h + m {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
