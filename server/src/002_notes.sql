-- Notas: pueden pertenecer a un proyecto o quedar sueltas (project_id NULL).
-- Al borrar un proyecto sus notas no se pierden: quedan sin proyecto.
-- tags guarda las etiquetas normalizadas separadas por coma ("idea,reunion").
CREATE TABLE notes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER REFERENCES projects (id) ON DELETE SET NULL,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL DEFAULT '',
    tags       TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_notes_project ON notes (project_id);
CREATE INDEX idx_notes_updated ON notes (updated_at);
