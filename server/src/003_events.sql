-- Eventos de calendario (reuniones, presentaciones, lanzamientos...).
-- Las tareas, las cuotas y las fechas de los proyectos NO se copian aquí:
-- el calendario las lee de sus propias tablas.
-- time es opcional (HH:MM); sin hora el evento es de todo el día.
-- Proyecto y cliente son opcionales y, si se borran, el evento se conserva.
CREATE TABLE events (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    title      TEXT NOT NULL,
    kind       TEXT NOT NULL DEFAULT 'reunion',
    date       TEXT NOT NULL,
    time       TEXT,
    project_id INTEGER REFERENCES projects (id) ON DELETE SET NULL,
    client_id  INTEGER REFERENCES clients (id) ON DELETE SET NULL,
    notes      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE INDEX idx_events_date ON events (date);
