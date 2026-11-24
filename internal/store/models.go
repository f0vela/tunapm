package store

import "time"

const (
	StatusProspecto  = "prospecto"
	StatusActivo     = "activo"
	StatusPausado    = "pausado"
	StatusCompletado = "completado"
	StatusCancelado  = "cancelado"

	TaskPendiente = "pendiente"
	TaskEnCurso   = "en_curso"
	TaskHecho     = "hecho"

	PaymentPendiente = "pendiente"
	PaymentVencido   = "vencido"
	PaymentPagado    = "pagado"

	CurrencyGTQ = "GTQ"
	CurrencyUSD = "USD"

	BillingTotal   = "total"
	BillingParcial = "parcial"
)

// ProjectStatuses y TaskStatuses alimentan los selectores de la UI.
var ProjectStatuses = []string{StatusProspecto, StatusActivo, StatusPausado, StatusCompletado, StatusCancelado}
var TaskStatuses = []string{TaskPendiente, TaskEnCurso, TaskHecho}

type Client struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Company   string `json:"company"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Notes     string `json:"notes"`
	CreatedAt string `json:"created_at"`
}

type Project struct {
	ID          int64   `json:"id"`
	ClientID    int64   `json:"client_id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	StartDate   *string `json:"start_date"` // YYYY-MM-DD
	DueDate     *string `json:"due_date"`   // YYYY-MM-DD
	BudgetCents int64   `json:"budget_cents"`
	Currency    string  `json:"currency"`
	BillingType string  `json:"billing_type"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`

	// Solo lectura, rellenado por las consultas con JOIN.
	ClientName string `json:"client_name"`
}

type Task struct {
	ID          int64   `json:"id"`
	ProjectID   int64   `json:"project_id"`
	Title       string  `json:"title"`
	Notes       string  `json:"notes"`
	Status      string  `json:"status"`
	Priority    int     `json:"priority"` // 0 baja .. 3 urgente
	DueDate     *string `json:"due_date"`
	CreatedAt   string  `json:"created_at"`
	CompletedAt *string `json:"completed_at"`

	// Solo lectura, rellenado por las consultas con JOIN.
	ProjectName string `json:"project_name"`
	ClientName  string `json:"client_name"`
}

type Payment struct {
	ID            int64   `json:"id"`
	ProjectID     int64   `json:"project_id"`
	AmountCents   int64   `json:"amount_cents"`
	ScheduledDate string  `json:"scheduled_date"` // YYYY-MM-DD
	PaidDate      *string `json:"paid_date"`
	Notes         string  `json:"notes"`

	// Solo lectura, rellenado por las consultas con JOIN.
	ProjectName string `json:"project_name"`
	ClientName  string `json:"client_name"`
	Currency    string `json:"currency"`
}

// StatusToday deriva el estado de una cuota: pagada, vencida o pendiente.
// today debe venir en formato YYYY-MM-DD.
func (p Payment) StatusToday(today string) string {
	switch {
	case p.PaidDate != nil:
		return PaymentPagado
	case p.ScheduledDate < today:
		return PaymentVencido
	default:
		return PaymentPendiente
	}
}

type ProjectTotals struct {
	ScheduledCents int64 `json:"scheduled_cents"` // todas las cuotas programadas
	PaidCents      int64 `json:"paid_cents"`      // cuotas con paid_date
}

// UnassignedCents es lo presupuestado que aún no tiene cuota programada.
func (t ProjectTotals) UnassignedCents(budgetCents int64) int64 {
	return budgetCents - t.ScheduledCents
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
