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
                'notes' => $this->listNotes(),
                'events' => $this->listEvents(),
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

    // ---------- notes ----------

    private const NOTE = "SELECT n.id, n.project_id, n.title, n.body, n.tags, n.created_at, n.updated_at,
        COALESCE(p.name, '') AS project_name, COALESCE(c.name, '') AS client_name
        FROM notes n LEFT JOIN projects p ON p.id = n.project_id LEFT JOIN clients c ON c.id = p.client_id";

    /** Minúsculas, sin "#" ni comas, guiones en vez de espacios (igual que store.NormalizeTag en Go). */
    public static function normalizeTag(string $raw): string
    {
        $t = trim($raw);
        $t = function_exists('mb_strtolower') ? mb_strtolower($t, 'UTF-8') : strtolower($t);
        $t = str_replace(',', ' ', ltrim($t, '#'));
        return implode('-', preg_split('/\s+/u', $t, -1, PREG_SPLIT_NO_EMPTY) ?: []);
    }

    /** @param mixed $raw lista de etiquetas, o texto separado por espacios o comas */
    public static function normalizeTags(mixed $raw): array
    {
        if (is_string($raw)) {
            $raw = preg_split('/[\s,]+/u', $raw, -1, PREG_SPLIT_NO_EMPTY) ?: [];
        }
        if ($raw === null) {
            return [];
        }
        if (!is_array($raw)) {
            throw new BadInput('campo inválido: tags');
        }
        $out = [];
        foreach ($raw as $tag) {
            if (!is_scalar($tag)) {
                throw new BadInput('campo inválido: tags');
            }
            $t = self::normalizeTag((string) $tag);
            if ($t !== '' && !in_array($t, $out, true)) {
                $out[] = $t;
            }
        }
        return $out;
    }

    private static function noteRow(array $row): array
    {
        $row['tags'] = $row['tags'] === '' ? [] : explode(',', $row['tags']);
        return $row;
    }

    public function getNote(int $id): array { return self::noteRow($this->one(self::NOTE . ' WHERE n.id = ?', [$id])); }

    /** @param array{project_id?:int,unassigned?:bool,tag?:string,search?:string} $f */
    public function listNotes(array $f = []): array
    {
        $where = [];
        $args = [];
        if (!empty($f['unassigned'])) {
            $where[] = 'n.project_id IS NULL';
        } elseif (!empty($f['project_id'])) {
            $where[] = 'n.project_id = ?';
            $args[] = (int) $f['project_id'];
        }
        $tag = self::normalizeTag((string) ($f['tag'] ?? ''));
        if ($tag !== '') {
            $where[] = "(',' || n.tags || ',') LIKE ?";
            $args[] = '%,' . $tag . ',%';
        }
        $search = trim((string) ($f['search'] ?? ''));
        if ($search !== '') {
            $where[] = "(n.title LIKE ? OR n.body LIKE ? OR n.tags LIKE ? OR COALESCE(p.name, '') LIKE ?)";
            $needle = '%' . $search . '%';
            array_push($args, $needle, $needle, $needle, $needle);
        }
        $sql = self::NOTE . ($where ? ' WHERE ' . implode(' AND ', $where) : '') . ' ORDER BY n.updated_at DESC, n.id DESC';
        return array_map([self::class, 'noteRow'], $this->all($sql, $args));
    }

    /** Etiquetas en uso, de la más usada a la menos. @return list<array{tag:string,count:int}> */
    public function noteTags(): array
    {
        $counts = [];
        foreach ($this->all("SELECT tags FROM notes WHERE tags <> ''") as $row) {
            foreach (explode(',', $row['tags']) as $t) {
                $counts[$t] = ($counts[$t] ?? 0) + 1;
            }
        }
        $out = [];
        foreach ($counts as $t => $n) {
            $out[] = ['tag' => (string) $t, 'count' => $n];
        }
        usort($out, fn (array $a, array $b) => $b['count'] <=> $a['count'] ?: strcmp($a['tag'], $b['tag']));
        return $out;
    }

    private function noteFields(array $d): array
    {
        $pid = $d['project_id'] ?? null;
        $pid = ($pid === null || $pid === '' || $pid === 0 || $pid === '0') ? null : self::int($d, 'project_id');
        $body = $d['body'] ?? '';
        if (!is_scalar($body)) {
            throw new BadInput('campo inválido: body');
        }
        return [
            $pid,
            self::required(self::str($d, 'title'), 'title'),
            (string) $body,
            implode(',', self::normalizeTags($d['tags'] ?? [])),
        ];
    }

    public function createNote(array $d): array
    {
        $now = self::now();
        $this->guard(fn () => $this->q(
            'INSERT INTO notes (project_id, title, body, tags, created_at, updated_at) VALUES (?,?,?,?,?,?)',
            [...$this->noteFields($d), $now, $now]
        ));
        return $this->getNote((int) $this->db->lastInsertId());
    }

    public function updateNote(int $id, array $d): array
    {
        $st = $this->guard(fn () => $this->q(
            'UPDATE notes SET project_id=?, title=?, body=?, tags=?, updated_at=? WHERE id=?',
            [...$this->noteFields($d), self::now(), $id]
        ));
        $this->affected($st);
        return $this->getNote($id);
    }

    public function deleteNote(int $id): void
    {
        $this->affected($this->q('DELETE FROM notes WHERE id = ?', [$id]));
    }

    // ---------- events ----------

    public const EVENT_KINDS = ['reunion' => 'reunión', 'llamada' => 'llamada', 'presentacion' => 'presentación',
        'lanzamiento' => 'lanzamiento', 'otro' => 'otro'];

    private const EVENT = "SELECT e.id, e.title, e.kind, e.date, e.time, e.project_id, e.client_id, e.notes, e.created_at,
        COALESCE(p.name, '') AS project_name, COALESCE(c.name, pc.name, '') AS client_name
        FROM events e LEFT JOIN projects p ON p.id = e.project_id
        LEFT JOIN clients c ON c.id = e.client_id LEFT JOIN clients pc ON pc.id = p.client_id";

    /** Acepta "9:30", "09:30" o "0930" → "09:30"; null si viene vacío (igual que store.NormalizeTime en Go). */
    public static function normalizeTime(mixed $raw): ?string
    {
        if ($raw === null) {
            return null;
        }
        if (!is_scalar($raw)) {
            throw new BadInput('campo inválido: time');
        }
        $t = trim((string) $raw);
        if ($t === '') {
            return null;
        }
        if (!str_contains($t, ':') && strlen($t) >= 3 && strlen($t) <= 4) {
            $t = substr($t, 0, -2) . ':' . substr($t, -2);
        }
        if (strlen($t) === 4) {
            $t = '0' . $t;
        }
        if (!preg_match('/^([01]\d|2[0-3]):[0-5]\d$/', $t)) {
            throw new BadInput('hora del evento inválida (usa HH:MM, 24 horas)');
        }
        return $t;
    }

    private static function optionalId(array $d, string $k): ?int
    {
        $v = $d[$k] ?? null;
        return ($v === null || $v === '' || $v === 0 || $v === '0') ? null : self::int($d, $k);
    }

    public function getEvent(int $id): array { return $this->one(self::EVENT . ' WHERE e.id = ?', [$id]); }

    /** Eventos entre $from y $to (inclusive); un límite vacío no acota. */
    public function listEvents(string $from = '', string $to = ''): array
    {
        $where = [];
        $args = [];
        if ($from !== '') {
            $where[] = 'e.date >= ?';
            $args[] = $from;
        }
        if ($to !== '') {
            $where[] = 'e.date <= ?';
            $args[] = $to;
        }
        return $this->all(self::EVENT . ($where ? ' WHERE ' . implode(' AND ', $where) : '') .
            ' ORDER BY e.date, e.time IS NOT NULL, e.time, e.id', $args);
    }

    private function eventFields(array $d): array
    {
        $title = self::required(self::str($d, 'title'), 'title');
        $date = self::date($d['date'] ?? null);
        if ($date === null || !checkdate((int) substr($date, 5, 2), (int) substr($date, 8, 2), (int) substr($date, 0, 4))) {
            throw new BadInput('fecha del evento inválida (usa YYYY-MM-DD)');
        }
        $kind = self::str($d, 'kind', 'otro');
        $notes = $d['notes'] ?? '';
        if (!is_scalar($notes)) {
            throw new BadInput('campo inválido: notes');
        }
        return [
            $title, isset(self::EVENT_KINDS[$kind]) ? $kind : 'otro', $date, self::normalizeTime($d['time'] ?? null),
            self::optionalId($d, 'project_id'), self::optionalId($d, 'client_id'), (string) $notes,
        ];
    }

    public function createEvent(array $d): array
    {
        $f = $this->eventFields($d);
        $this->guard(fn () => $this->q(
            'INSERT INTO events (title, kind, date, time, project_id, client_id, notes, created_at) VALUES (?,?,?,?,?,?,?,?)',
            [...$f, self::now()]
        ));
        return $this->getEvent((int) $this->db->lastInsertId());
    }

    public function updateEvent(int $id, array $d): array
    {
        $f = $this->eventFields($d);
        $st = $this->guard(fn () => $this->q(
            'UPDATE events SET title=?, kind=?, date=?, time=?, project_id=?, client_id=?, notes=? WHERE id=?',
            [...$f, $id]
        ));
        $this->affected($st);
        return $this->getEvent($id);
    }

    public function deleteEvent(int $id): void
    {
        $this->affected($this->q('DELETE FROM events WHERE id = ?', [$id]));
    }

    // ---------- agenda (calendario) ----------

    private static function joinNames(string $a, string $b): string
    {
        return $a === '' ? $b : ($b === '' ? $a : $a . ' · ' . $b);
    }

    /**
     * Entradas del calendario entre $from y $to: eventos, tareas con fecha, cuotas por cobrar
     * e inicio/entrega de proyectos. Mismas reglas y mismo orden que internal/agenda en Go.
     */
    public function agenda(string $from, string $to, string $today): array
    {
        $item = fn (array $o) => $o + ['time' => '', 'kind' => '', 'detail' => '', 'project_id' => 0, 'done' => false,
            'overdue' => false, 'amount_cents' => 0, 'currency' => ''];
        $items = [];
        foreach ($this->listEvents($from, $to) as $e) {
            $items[] = $item(['date' => $e['date'], 'time' => (string) ($e['time'] ?? ''), 'type' => 'event', 'kind' => $e['kind'],
                'id' => $e['id'], 'title' => $e['title'], 'detail' => self::joinNames($e['project_name'], $e['client_name']),
                'project_id' => (int) ($e['project_id'] ?? 0)]);
        }
        foreach ($this->all(self::TASK . ' WHERE t.due_date BETWEEN ? AND ?', [$from, $to]) as $t) {
            $done = $t['status'] === 'hecho';
            $items[] = $item(['date' => $t['due_date'], 'type' => 'task', 'id' => $t['id'], 'title' => $t['title'],
                'detail' => self::joinNames($t['project_name'], $t['client_name']), 'project_id' => $t['project_id'],
                'done' => $done, 'overdue' => !$done && $t['due_date'] < $today]);
        }
        // Solo lo que falta cobrar.
        foreach ($this->all(self::PAYMENT . ' WHERE pay.paid_date IS NULL AND pay.scheduled_date BETWEEN ? AND ?', [$from, $to]) as $p) {
            $items[] = $item(['date' => $p['scheduled_date'], 'type' => 'payment', 'id' => $p['id'], 'title' => $p['notes'],
                'detail' => self::joinNames($p['project_name'], $p['client_name']), 'project_id' => $p['project_id'],
                'overdue' => $p['scheduled_date'] < $today, 'amount_cents' => $p['amount_cents'], 'currency' => $p['currency']]);
        }
        foreach ($this->listProjects() as $p) {
            if ($p['status'] === 'cancelado') {
                continue;
            }
            $done = $p['status'] === 'completado';
            $start = $p['start_date'];
            $due = $p['due_date'];
            if ($start !== null && $start >= $from && $start <= $to) {
                $items[] = $item(['date' => $start, 'type' => 'project_start', 'id' => $p['id'], 'title' => $p['name'],
                    'detail' => $p['client_name'], 'project_id' => $p['id'], 'done' => $done || $start < $today]);
            }
            if ($due !== null && $due >= $from && $due <= $to) {
                $items[] = $item(['date' => $due, 'type' => 'project_due', 'id' => $p['id'], 'title' => $p['name'],
                    'detail' => $p['client_name'], 'project_id' => $p['id'], 'done' => $done, 'overdue' => !$done && $due < $today]);
            }
        }
        $order = ['event' => 0, 'project_due' => 1, 'project_start' => 2, 'payment' => 3, 'task' => 4];
        usort($items, fn (array $a, array $b) => strcmp($a['date'], $b['date'])
            ?: (($a['time'] === '' ? 0 : 1) <=> ($b['time'] === '' ? 0 : 1))
            ?: strcmp($a['time'], $b['time'])
            ?: ($order[$a['type']] <=> $order[$b['type']])
            ?: ($a['id'] <=> $b['id']));
        return $items;
    }
}
