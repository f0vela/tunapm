package store

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound          = errors.New("no encontrado")
	ErrClientHasProjects = errors.New("el cliente tiene proyectos")
)

//go:embed migrations/001_init.sql
var initSQL string

//go:embed migrations/002_notes.sql
var notesSQL string

type Store struct {
	db *sql.DB
}

// Open abre (o crea) la base de datos y aplica la migración inicial.
// Usa ":memory:" para una base efímera, útil en tests.
func Open(path string) (*Store, error) {
	dsn := path
	if path == ":memory:" {
		dsn = "file::memory:?cache=shared"
	} else if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("crear directorio de datos: %w", err)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Una sola conexión: asegura los PRAGMAs y evita bloqueos de escritura.
	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	// Cada migración se aplica una sola vez y sube user_version.
	for i, script := range []string{initSQL, notesSQL} {
		if version > i {
			continue
		}
		if _, err := s.db.Exec(script); err != nil {
			return fmt.Errorf("migración %03d: %w", i+1, err)
		}
		if _, err := s.db.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			return err
		}
	}
	return nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func strPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}
