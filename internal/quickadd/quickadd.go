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
	AmountCents int64
	Currency    string
	Priority    int    // -1 si no se indicó
	Date        string // texto de ^, sin parsear (ver internal/dates)
}

// Parse interpreta:
//
//	t Diseñar logo @web-acme !alta ^viernes
//	c Juan Pérez #Acme
//	p Sitio web @Acme $5000 usd ^2026-12-20
//	$ 2500 @web-acme ^15/12
func Parse(input string) (Draft, error) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return Draft{}, ErrNotQuickAdd
	}

	kind := Kind(fields[0])
	switch kind {
	case KindTask, KindClient, KindProject, KindPayment:
	default:
		return Draft{}, ErrNotQuickAdd
	}

	d := Draft{Kind: kind, Currency: "GTQ", Priority: -1}

	var titleWords []string
	for _, field := range fields[1:] {
		switch {
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
