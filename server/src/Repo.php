<?php
declare(strict_types=1);

namespace Tuna;

/** Acceso a datos: mismas reglas que internal/store de la versión Go. */
final class Repo
{
    public function __construct(private \PDO $db) {}

    public static function now(): string { return gmdate('Y-m-d\TH:i:s\Z'); }

    // ---------- helpers ----------

    private function q(string $sql, array $args = []): \PDOStatement
    {
        $st = $this->db->prepare($sql);
        $st->execute($args);
        return $st;
    }

    private function all(string $sql, array $args = []): array { return $this->q($sql, $args)->fetchAll(); }

    private function one(string $sql, array $args, string $what = 'no encontrado'): array
    {
        $row = $this->q($sql, $args)->fetch();
        if ($row === false) {
            throw new NotFound($what);
        }
        return $row;
    }

    private function scalar(string $sql, array $args = []): int { return (int) $this->q($sql, $args)->fetchColumn(); }

    private function affected(\PDOStatement $st): void
    {
        if ($st->rowCount() === 0) {
            throw new NotFound();
        }
    }

    /** Convierte violaciones de CHECK/FK/NOT NULL en errores 422. */
    private function guard(callable $fn): mixed
    {
        try {
            return $fn();
        } catch (\PDOException $e) {
            if (str_starts_with((string) $e->getCode(), '23')) {
                throw new BadInput('datos inválidos: ' . $e->getMessage());
            }
            throw $e;
        }
    }

    private static function date(mixed $v): ?string
    {
        if ($v === null || $v === '') {
            return null;
        }
        if (!is_string($v) || !preg_match('/^\d{4}-\d{2}-\d{2}$/', $v)) {
            throw new BadInput('fecha inválida (usa YYYY-MM-DD)');
        }
        return $v;
    }

    private static function str(array $d, string $k, string $default = ''): string
    {
        $v = $d[$k] ?? $default;
        if (!is_scalar($v)) {
            throw new BadInput("campo inválido: $k");
        }
        return trim((string) $v);
    }

    private static function int(array $d, string $k, int $default = 0): int
    {
        $v = $d[$k] ?? $default;
        if (is_int($v) || (is_string($v) && preg_match('/^-?\d+$/', $v))) {
            return (int) $v;
        }
        throw new BadInput("campo inválido: $k");
    }

    private static function required(string $v, string $label): string
    {
        if ($v === '') {
            throw new BadInput("$label es obligatorio");
        }
        return $v;
    }

    /** Todos los datos en una lectura consistente (para /api/snapshot). */
    public function snapshot(): array
    {
        $this->db->beginTransaction();
        try {
            return [
                'clients' => $this->listClients(),
                'projects' => $this->listProjects(),
                'tasks' => $this->listTasks(['mode' => 'todas']),
                'payments' => $this->listPayments(),
            ];
        } finally {
            $this->db->commit();
        }
    }

    // ---------- clients ----------

    private const CLIENT = 'SELECT id, name, company, email, phone, notes, created_at FROM clients';

    public function listClients(): array { return $this->all(self::CLIENT . ' ORDER BY name COLLATE NOCASE'); }

    public function getClient(int $id): array { return $this->one(self::CLIENT . ' WHERE id = ?', [$id]); }

    public function createClient(array $d): array
    {
        $this->guard(fn () => $this->q(
            'INSERT INTO clients (name, company, email, phone, notes, created_at) VALUES (?,?,?,?,?,?)',
            [self::required(self::str($d, 'name'), 'name'), self::str($d, 'company'), self::str($d, 'email'),
             self::str($d, 'phone'), self::str($d, 'notes'), self::now()]
        ));
        return $this->getClient((int) $this->db->lastInsertId());
    }

    public function updateClient(int $id, array $d): array
    {
        $st = $this->guard(fn () => $this->q(
            'UPDATE clients SET name=?, company=?, email=?, phone=?, notes=? WHERE id=?',
            [self::required(self::str($d, 'name'), 'name'), self::str($d, 'company'), self::str($d, 'email'),
             self::str($d, 'phone'), self::str($d, 'notes'), $id]
        ));
        $this->affected($st);
        return $this->getClient($id);
    }

    public function deleteClient(int $id): void
    {
        if ($this->countProjectsByClient($id) > 0) {
            throw new ClientHasProjects();
        }
        $this->affected($this->q('DELETE FROM clients WHERE id = ?', [$id]));
    }

    // ---------- projects ----------

    private const PROJECT = "SELECT p.id, p.client_id, p.name, p.description, p.status, p.start_date, p.due_date,
        p.budget_cents, p.currency, p.billing_type, p.created_at, p.updated_at, COALESCE(c.name, '') AS client_name
        FROM projects p JOIN clients c ON c.id = p.client_id";

    private const PROJECT_ORDER = " ORDER BY CASE p.status WHEN 'activo' THEN 0 WHEN 'prospecto' THEN 1
        WHEN 'pausado' THEN 2 WHEN 'completado' THEN 3 ELSE 4 END, p.name COLLATE NOCASE";

    public function listProjects(): array { return $this->all(self::PROJECT . self::PROJECT_ORDER); }

    public function projectsByClient(int $clientId): array
    {
        return $this->all(self::PROJECT . ' WHERE p.client_id = ?' . self::PROJECT_ORDER, [$clientId]);
    }

    public function countProjectsByClient(int $clientId): int
    {
        return $this->scalar('SELECT COUNT(*) FROM projects WHERE client_id = ?', [$clientId]);
    }

    public function getProject(int $id): array { return $this->one(self::PROJECT . ' WHERE p.id = ?', [$id]); }

    private function projectFields(array $d): array
    {
        return [
            self::int($d, 'client_id'),
            self::required(self::str($d, 'name'), 'name'),
            self::str($d, 'description'),
            self::str($d, 'status', 'prospecto'),
            self::date($d['start_date'] ?? null),
            self::date($d['due_date'] ?? null),
            self::int($d, 'budget_cents'),
            self::str($d, 'currency', 'GTQ'),
            self::str($d, 'billing_type', 'total'),
        ];
    }

    public function createProject(array $d): array
    {
        $now = self::now();
        $this->guard(fn () => $this->q(
            'INSERT INTO projects (client_id, name, description, status, start_date, due_date, budget_cents,
             currency, billing_type, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)',
            [...$this->projectFields($d), $now, $now]
        ));
        return $this->getProject((int) $this->db->lastInsertId());
    }

    public function updateProject(int $id, array $d): array
    {
        $f = $this->projectFields($d);
        if ($f[8] === 'total' && $this->scalar('SELECT COUNT(*) FROM payments WHERE project_id = ?', [$id]) > 1) {
            throw new TotalOneQuote();
        }
        $st = $this->guard(fn () => $this->q(
            'UPDATE projects SET client_id=?, name=?, description=?, status=?, start_date=?, due_date=?,
             budget_cents=?, currency=?, billing_type=?, updated_at=? WHERE id=?',
            [...$f, self::now(), $id]
        ));
        $this->affected($st);
        return $this->getProject($id);
    }

    public function deleteProject(int $id): void
    {
        $this->affected($this->q('DELETE FROM projects WHERE id = ?', [$id]));
    }

    // ---------- tasks ----------

    private const TASK = "SELECT t.id, t.project_id, t.title, t.notes, t.status, t.priority, t.due_date,
        t.created_at, t.completed_at, p.name AS project_name, COALESCE(c.name, '') AS client_name
        FROM tasks t JOIN projects p ON p.id = t.project_id JOIN clients c ON c.id = p.client_id";

    public function getTask(int $id): array { return $this->one(self::TASK . ' WHERE t.id = ?', [$id]); }

    /** @param array{mode?:string,project_id?:?int,client_id?:?int,search?:string} $f */
    public function listTasks(array $f = []): array
    {
        $where = [];
        $args = [];
        $mode = $f['mode'] ?? '';
        if ($mode === 'hechas') {
            $where[] = "t.status = 'hecho'";
        } elseif ($mode !== 'todas') {
            $where[] = "t.status <> 'hecho'";
        }
        if (!empty($f['project_id'])) {
            $where[] = 't.project_id = ?';
            $args[] = (int) $f['project_id'];
        }
        if (!empty($f['client_id'])) {
            $where[] = 'p.client_id = ?';
            $args[] = (int) $f['client_id'];
        }
        $search = trim((string) ($f['search'] ?? ''));
        if ($search !== '') {
            $where[] = '(t.title LIKE ? OR t.notes LIKE ? OR p.name LIKE ? OR c.name LIKE ?)';
            $needle = '%' . $search . '%';
            array_push($args, $needle, $needle, $needle, $needle);
        }
        $sql = self::TASK . ($where ? ' WHERE ' . implode(' AND ', $where) : '') .
            " ORDER BY CASE WHEN t.status = 'hecho' THEN 1 ELSE 0 END, t.priority DESC,
              t.due_date IS NULL, t.due_date ASC, t.id";
        return $this->all($sql, $args);
    }

    private function taskFields(array $d): array
    {
        return [
            self::int($d, 'project_id'),
            self::required(self::str($d, 'title'), 'title'),
            self::str($d, 'notes'),
            self::str($d, 'status', 'pendiente'),
            self::int($d, 'priority', 1),
            self::date($d['due_date'] ?? null),
        ];
    }

    public function createTask(array $d): array
    {
        $f = $this->taskFields($d);
        $now = self::now();
        $this->guard(fn () => $this->q(
            'INSERT INTO tasks (project_id, title, notes, status, priority, due_date, created_at, completed_at)
             VALUES (?,?,?,?,?,?,?,?)',
            [...$f, $now, $f[3] === 'hecho' ? $now : null]
        ));
        return $this->getTask((int) $this->db->lastInsertId());
    }

    public function updateTask(int $id, array $d): array
    {
        $f = $this->taskFields($d);
        $current = $this->getTask($id);
        // completed_at siempre coherente con el estado.
        $completed = $f[3] === 'hecho' ? ($current['completed_at'] ?? self::now()) : null;
        $st = $this->guard(fn () => $this->q(
            'UPDATE tasks SET project_id=?, title=?, notes=?, status=?, priority=?, due_date=?, completed_at=? WHERE id=?',
            [...$f, $completed, $id]
        ));
        $this->affected($st);
        return $this->getTask($id);
    }

    public function deleteTask(int $id): void
    {
        $this->affected($this->q('DELETE FROM tasks WHERE id = ?', [$id]));
    }

    /** @return array<int,array{done:int,total:int}> */
    public function taskProgress(): array
    {
        $out = [];
        foreach ($this->all("SELECT project_id, COUNT(*) AS total, SUM(CASE WHEN status='hecho' THEN 1 ELSE 0 END) AS done
                             FROM tasks GROUP BY project_id") as $r) {
            $out[(int) $r['project_id']] = ['done' => (int) $r['done'], 'total' => (int) $r['total']];
        }
        return $out;
    }

    // ---------- payments ----------

    private const PAYMENT = "SELECT pay.id, pay.project_id, pay.amount_cents, pay.scheduled_date, pay.paid_date, pay.notes,
        p.name AS project_name, COALESCE(c.name, '') AS client_name, p.currency
        FROM payments pay JOIN projects p ON p.id = pay.project_id JOIN clients c ON c.id = p.client_id";

    public function getPayment(int $id): array { return $this->one(self::PAYMENT . ' WHERE pay.id = ?', [$id]); }

    public function listPayments(): array
    {
        return $this->all(self::PAYMENT . ' ORDER BY pay.paid_date IS NOT NULL, pay.scheduled_date, pay.id');
    }

    public function paymentsByProject(int $projectId): array
    {
        return $this->all(self::PAYMENT . ' WHERE pay.project_id = ? ORDER BY pay.scheduled_date, pay.id', [$projectId]);
    }

    private function paymentFields(array $d): array
    {
        $amount = self::int($d, 'amount_cents');
        if ($amount <= 0) {
            throw new InvalidAmount();
        }
        $scheduled = self::date($d['scheduled_date'] ?? null);
        if ($scheduled === null) {
            throw new BadInput('scheduled_date es obligatorio');
        }
        return [$amount, $scheduled, self::date($d['paid_date'] ?? null), self::str($d, 'notes')];
    }

    public function createPayment(array $d): array
    {
        $projectId = self::int($d, 'project_id');
        $f = $this->paymentFields($d);
        $project = $this->getProject($projectId);
        if ($project['billing_type'] === 'total'
            && $this->scalar('SELECT COUNT(*) FROM payments WHERE project_id = ?', [$projectId]) > 0) {
            throw new TotalOneQuote();
        }
        $this->guard(fn () => $this->q(
            'INSERT INTO payments (project_id, amount_cents, scheduled_date, paid_date, notes) VALUES (?,?,?,?,?)',
            [$projectId, ...$f]
        ));
        return $this->getPayment((int) $this->db->lastInsertId());
    }

    public function updatePayment(int $id, array $d): array
    {
        $f = $this->paymentFields($d);
        $this->affected($this->q(
            'UPDATE payments SET amount_cents=?, scheduled_date=?, paid_date=?, notes=? WHERE id=?',
            [...$f, $id]
        ));
        return $this->getPayment($id);
    }

    public function markPaymentPaid(int $id, string $date): void
    {
        $this->affected($this->q('UPDATE payments SET paid_date = ? WHERE id = ?', [self::date($date) ?? throw new BadInput('date es obligatorio'), $id]));
    }

    public function deletePayment(int $id): void
    {
        $this->affected($this->q('DELETE FROM payments WHERE id = ?', [$id]));
    }

    /** @return array{scheduled_cents:int,paid_cents:int} */
    public function projectTotals(int $projectId): array
    {
        $r = $this->q("SELECT COALESCE(SUM(amount_cents),0) AS s,
                       COALESCE(SUM(CASE WHEN paid_date IS NOT NULL THEN amount_cents ELSE 0 END),0) AS p
                       FROM payments WHERE project_id = ?", [$projectId])->fetch();
        return ['scheduled_cents' => (int) $r['s'], 'paid_cents' => (int) $r['p']];
    }

    /** @return array<int,array{scheduled_cents:int,paid_cents:int}> */
    public function allProjectTotals(): array
    {
        $out = [];
        foreach ($this->all("SELECT project_id, COALESCE(SUM(amount_cents),0) AS s,
                             COALESCE(SUM(CASE WHEN paid_date IS NOT NULL THEN amount_cents ELSE 0 END),0) AS p
                             FROM payments GROUP BY project_id") as $r) {
            $out[(int) $r['project_id']] = ['scheduled_cents' => (int) $r['s'], 'paid_cents' => (int) $r['p']];
        }
        return $out;
    }
}
