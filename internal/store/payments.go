package store

import (
	"database/sql"
	"errors"
)

var (
	ErrInvalidAmount        = errors.New("el monto de la cuota debe ser mayor que cero")
	ErrTotalBillingOneQuote = errors.New("un proyecto con cobro total solo admite una cuota")
)

func (s *Store) CreatePayment(p *Payment) error {
	if p.AmountCents <= 0 {
		return ErrInvalidAmount
	}
	project, err := s.GetProject(p.ProjectID)
	if err != nil {
		return err
	}
	if project.BillingType == BillingTotal {
		var n int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM payments WHERE project_id = ?", p.ProjectID).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrTotalBillingOneQuote
		}
	}
	res, err := s.db.Exec(
		`INSERT INTO payments (project_id, amount_cents, scheduled_date, paid_date, notes)
		 VALUES (?, ?, ?, ?, ?)`,
		p.ProjectID, p.AmountCents, p.ScheduledDate, p.PaidDate, p.Notes,
	)
	if err != nil {
		return err
	}
	p.ID, err = res.LastInsertId()
	return err
}

func (s *Store) UpdatePayment(p *Payment) error {
	if p.AmountCents <= 0 {
		return ErrInvalidAmount
	}
	res, err := s.db.Exec(
		`UPDATE payments SET amount_cents = ?, scheduled_date = ?, paid_date = ?, notes = ? WHERE id = ?`,
		p.AmountCents, p.ScheduledDate, p.PaidDate, p.Notes, p.ID,
	)
	return affectedOne(res, err)
}

// MarkPaymentPaid registra la fecha de pago; date va en formato YYYY-MM-DD.
func (s *Store) MarkPaymentPaid(id int64, date string) error {
	res, err := s.db.Exec("UPDATE payments SET paid_date = ? WHERE id = ?", date, id)
	return affectedOne(res, err)
}

func (s *Store) DeletePayment(id int64) error {
	res, err := s.db.Exec("DELETE FROM payments WHERE id = ?", id)
	return affectedOne(res, err)
}

func (s *Store) GetPayment(id int64) (Payment, error) {
	p, err := scanPayment(s.db.QueryRow(paymentQuery+" WHERE pay.id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Payment{}, ErrNotFound
	}
	return p, err
}

func (s *Store) ListPayments() ([]Payment, error) {
	return s.queryPayments(paymentQuery + " ORDER BY pay.paid_date IS NOT NULL, pay.scheduled_date, pay.id")
}

func (s *Store) PaymentsByProject(projectID int64) ([]Payment, error) {
	return s.queryPayments(paymentQuery+" WHERE pay.project_id = ? ORDER BY pay.scheduled_date, pay.id", projectID)
}

func (s *Store) ProjectTotals(projectID int64) (ProjectTotals, error) {
	var t ProjectTotals
	err := s.db.QueryRow(
		`SELECT COALESCE(SUM(amount_cents), 0),
		        COALESCE(SUM(CASE WHEN paid_date IS NOT NULL THEN amount_cents ELSE 0 END), 0)
		 FROM payments WHERE project_id = ?`, projectID,
	).Scan(&t.ScheduledCents, &t.PaidCents)
	return t, err
}

// AllProjectTotals devuelve los totales de cuotas de todos los proyectos.
func (s *Store) AllProjectTotals() (map[int64]ProjectTotals, error) {
	rows, err := s.db.Query(
		`SELECT project_id,
		        COALESCE(SUM(amount_cents), 0),
		        COALESCE(SUM(CASE WHEN paid_date IS NOT NULL THEN amount_cents ELSE 0 END), 0)
		 FROM payments GROUP BY project_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64]ProjectTotals)
	for rows.Next() {
		var id int64
		var t ProjectTotals
		if err := rows.Scan(&id, &t.ScheduledCents, &t.PaidCents); err != nil {
			return nil, err
		}
		out[id] = t
	}
	return out, rows.Err()
}

const paymentQuery = `
	SELECT pay.id, pay.project_id, pay.amount_cents, pay.scheduled_date, pay.paid_date, pay.notes,
	       p.name, COALESCE(c.name, ''), p.currency
	FROM payments pay
	JOIN projects p ON p.id = pay.project_id
	JOIN clients c ON c.id = p.client_id`

func (s *Store) queryPayments(query string, args ...any) ([]Payment, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Payment
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanPayment(row scanner) (Payment, error) {
	var p Payment
	var paid sql.NullString
	err := row.Scan(&p.ID, &p.ProjectID, &p.AmountCents, &p.ScheduledDate, &paid, &p.Notes,
		&p.ProjectName, &p.ClientName, &p.Currency)
	p.PaidDate = strPtr(paid)
	return p, err
}
