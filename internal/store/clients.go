package store

import (
	"database/sql"
	"errors"
)

func (s *Store) CreateClient(c *Client) error {
	c.CreatedAt = now()
	res, err := s.db.Exec(
		`INSERT INTO clients (name, company, email, phone, notes, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		c.Name, c.Company, c.Email, c.Phone, c.Notes, c.CreatedAt,
	)
	if err != nil {
		return err
	}
	c.ID, err = res.LastInsertId()
	return err
}

func (s *Store) UpdateClient(c *Client) error {
	res, err := s.db.Exec(
		`UPDATE clients SET name = ?, company = ?, email = ?, phone = ?, notes = ? WHERE id = ?`,
		c.Name, c.Company, c.Email, c.Phone, c.Notes, c.ID,
	)
	return affectedOne(res, err)
}

func (s *Store) DeleteClient(id int64) error {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM projects WHERE client_id = ?", id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrClientHasProjects
	}
	res, err := s.db.Exec("DELETE FROM clients WHERE id = ?", id)
	return affectedOne(res, err)
}

func (s *Store) GetClient(id int64) (Client, error) {
	c, err := scanClient(s.db.QueryRow(clientQuery+" WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Client{}, ErrNotFound
	}
	return c, err
}

func (s *Store) ListClients() ([]Client, error) {
	rows, err := s.db.Query(clientQuery + " ORDER BY name COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Client
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

const clientQuery = `SELECT id, name, company, email, phone, notes, created_at FROM clients`

type scanner interface{ Scan(dest ...any) error }

func scanClient(row scanner) (Client, error) {
	var c Client
	err := row.Scan(&c.ID, &c.Name, &c.Company, &c.Email, &c.Phone, &c.Notes, &c.CreatedAt)
	return c, err
}

func affectedOne(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
