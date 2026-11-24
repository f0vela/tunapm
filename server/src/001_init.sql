CREATE TABLE clients (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    company    TEXT NOT NULL DEFAULT '',
    email      TEXT NOT NULL DEFAULT '',
    phone      TEXT NOT NULL DEFAULT '',
    notes      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE projects (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    client_id    INTEGER NOT NULL REFERENCES clients (id) ON DELETE RESTRICT,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'prospecto'
                 CHECK (status IN ('prospecto', 'activo', 'pausado', 'completado', 'cancelado')),
    start_date   TEXT,
    due_date     TEXT,
    budget_cents INTEGER NOT NULL DEFAULT 0,
    currency     TEXT NOT NULL DEFAULT 'GTQ' CHECK (currency IN ('GTQ', 'USD')),
    billing_type TEXT NOT NULL DEFAULT 'total' CHECK (billing_type IN ('total', 'parcial')),
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);

CREATE TABLE tasks (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id   INTEGER NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    title        TEXT NOT NULL,
    notes        TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'pendiente'
                 CHECK (status IN ('pendiente', 'en_curso', 'hecho')),
    priority     INTEGER NOT NULL DEFAULT 1 CHECK (priority BETWEEN 0 AND 3),
    due_date     TEXT,
    created_at   TEXT NOT NULL,
    completed_at TEXT
);

CREATE TABLE payments (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id     INTEGER NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    amount_cents   INTEGER NOT NULL,
    scheduled_date TEXT NOT NULL,
    paid_date      TEXT,
    notes          TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_projects_client ON projects (client_id);
CREATE INDEX idx_tasks_project ON tasks (project_id);
CREATE INDEX idx_tasks_status ON tasks (status);
CREATE INDEX idx_payments_project ON payments (project_id);
CREATE INDEX idx_payments_scheduled ON payments (scheduled_date);
