<?php
declare(strict_types=1);

namespace Tuna;

/** Web app renderizada en el servidor (móvil primero). Usa el mismo Repo que el API. */
final class Web
{
    private const COOKIE = 'tuna_session';
    private const TTL = 30 * 24 * 3600;

    public function __construct(private Repo $repo, private array $cfg, private bool $https) {}

    public function handle(string $method, string $path): void
    {
        View::$auth = true;
        if ($path === '/login') {
            $method === 'POST' ? $this->login() : View::render('login', [], '', 'Entrar');
            return;
        }
        if (!$this->loggedIn()) {
            header('Location: /login', true, 303);
            return;
        }
        View::$csrf = $this->csrf();
        if ($method === 'POST') {
            if (!hash_equals(View::$csrf, (string) ($_POST['_csrf'] ?? ''))) {
                throw new AppError(403, 'forbidden', 'Token CSRF inválido; recarga la página.');
            }
            if ($path === '/logout') {
                setcookie(self::COOKIE, '', $this->cookieOpts(time() - 3600));
                header('Location: /login', true, 303);
                return;
            }
        }
        foreach ($this->routes() as [$m, $re, $fn]) {
            if ($m === $method && preg_match('#^' . $re . '$#', $path, $mm)) {
                $fn(...array_map('intval', array_slice($mm, 1)));
                return;
            }
        }
        throw new NotFound('No encontrado');
    }

    // ---------- auth ----------

    private function sign(string $payload): string { return hash_hmac('sha256', $payload, $this->cfg['secret']); }

    private function cookieOpts(int $expires): array
    {
        return ['expires' => $expires, 'path' => '/', 'secure' => $this->https, 'httponly' => true, 'samesite' => 'Strict'];
    }

    private function loggedIn(): bool
    {
        $c = (string) ($_COOKIE[self::COOKIE] ?? '');
        $parts = explode('.', $c);
        return count($parts) === 2 && ctype_digit($parts[0]) && (int) $parts[0] > time()
            && hash_equals($this->sign('session:' . $parts[0]), $parts[1]);
    }

    private function csrf(): string { return $this->sign('csrf:' . ($_COOKIE[self::COOKIE] ?? '')); }

    private function login(): void
    {
        usleep(400_000);
        if (!hash_equals($this->cfg['web_password'], (string) ($_POST['password'] ?? ''))) {
            View::render('login', [], '', 'Entrar', 401, 'Contraseña incorrecta');
            return;
        }
        $exp = time() + self::TTL;
        setcookie(self::COOKIE, $exp . '.' . $this->sign('session:' . $exp), $this->cookieOpts($exp));
        header('Location: /', true, 303);
    }

    // ---------- helpers ----------

    private static function post(string $k): string { return trim((string) ($_POST[$k] ?? '')); }

    private static function redirect(string $to, string $flash = ''): void
    {
        if ($flash !== '') {
            $to .= (str_contains($to, '?') ? '&' : '?') . 'ok=' . rawurlencode($flash);
        }
        header('Location: ' . $to, true, 303);
    }

    /** Vuelve a "next" solo si es una ruta local. */
    private static function back(string $fallback, string $flash = ''): void
    {
        $next = (string) ($_POST['next'] ?? '');
        self::redirect(str_starts_with($next, '/') && !str_starts_with($next, '//') ? $next : $fallback, $flash);
    }

    private static function next(): string
    {
        $n = (string) ($_GET['next'] ?? $_POST['next'] ?? '');
        return str_starts_with($n, '/') && !str_starts_with($n, '//') ? $n : '';
    }

    private static function optDate(string $v): ?string { return $v === '' ? null : $v; }

    private function routes(): array
    {
        return [
            ['GET', '/', fn () => $this->dashboard()],

            ['GET', '/tasks', fn () => $this->tasksList()],
            ['GET', '/tasks/new', fn () => $this->taskForm(['project_id' => (int) ($_GET['project_id'] ?? 0), 'status' => 'pendiente', 'priority' => 1])],
            ['POST', '/tasks', fn () => $this->taskSave(0)],
            ['GET', '/tasks/(\d+)/edit', fn (int $id) => $this->taskForm($this->repo->getTask($id))],
            ['POST', '/tasks/(\d+)', fn (int $id) => $this->taskSave($id)],
            ['POST', '/tasks/(\d+)/toggle', fn (int $id) => $this->taskToggle($id)],
            ['POST', '/tasks/(\d+)/delete', function (int $id) { $this->repo->deleteTask($id); self::back('/tasks', 'Tarea eliminada'); }],

            ['GET', '/projects', fn () => $this->projectsList()],
            ['GET', '/projects/new', fn () => $this->projectForm(['client_id' => (int) ($_GET['client_id'] ?? 0), 'status' => 'activo', 'currency' => 'GTQ', 'billing_type' => 'total'])],
            ['POST', '/projects', fn () => $this->projectSave(0)],
            ['GET', '/projects/(\d+)', fn (int $id) => $this->projectShow($id)],
            ['GET', '/projects/(\d+)/edit', fn (int $id) => $this->projectForm($this->repo->getProject($id))],
            ['POST', '/projects/(\d+)', fn (int $id) => $this->projectSave($id)],
            ['POST', '/projects/(\d+)/delete', function (int $id) { $this->repo->deleteProject($id); self::redirect('/projects', 'Proyecto eliminado'); }],

            ['GET', '/clients', fn () => View::render('clients', ['clients' => $this->repo->listClients()], 'clientes', 'Clientes')],
            ['GET', '/clients/new', fn () => $this->clientForm([])],
            ['POST', '/clients', fn () => $this->clientSave(0)],
            ['GET', '/clients/(\d+)', fn (int $id) => $this->clientShow($id)],
            ['GET', '/clients/(\d+)/edit', fn (int $id) => $this->clientForm($this->repo->getClient($id))],
            ['POST', '/clients/(\d+)', fn (int $id) => $this->clientSave($id)],
            ['POST', '/clients/(\d+)/delete', fn (int $id) => $this->clientDelete($id)],

            ['GET', '/notes', fn () => $this->notesList()],
            ['GET', '/notes/new', fn () => $this->noteForm([
                'project_id' => (int) ($_GET['project_id'] ?? 0) ?: null,
                'tags' => Repo::normalizeTags((string) ($_GET['tag'] ?? '')),
            ])],
            ['POST', '/notes', fn () => $this->noteSave(0)],
            ['GET', '/notes/(\d+)/edit', fn (int $id) => $this->noteForm($this->repo->getNote($id))],
            ['POST', '/notes/(\d+)', fn (int $id) => $this->noteSave($id)],
            ['POST', '/notes/(\d+)/delete', function (int $id) { $this->repo->deleteNote($id); self::back('/notes', 'Nota eliminada'); }],

            ['GET', '/payments', fn () => View::render('payments', ['payments' => $this->repo->listPayments()], 'cuotas', 'Cuotas')],
            ['GET', '/payments/new', fn () => $this->paymentForm(['project_id' => (int) ($_GET['project_id'] ?? 0), 'scheduled_date' => date('Y-m-d')])],
            ['POST', '/payments', fn () => $this->paymentSave(0)],
            ['GET', '/payments/(\d+)/edit', fn (int $id) => $this->paymentForm($this->repo->getPayment($id))],
            ['POST', '/payments/(\d+)', fn (int $id) => $this->paymentSave($id)],
            ['POST', '/payments/(\d+)/pay', function (int $id) { $this->repo->markPaymentPaid($id, date('Y-m-d')); self::back('/payments', 'Cuota marcada como pagada'); }],
            ['POST', '/payments/(\d+)/unpay', function (int $id) {
                $pay = $this->repo->getPayment($id);
                $pay['paid_date'] = null;
                $this->repo->updatePayment($id, $pay);
                self::back('/payments', 'Cuota marcada como no pagada');
            }],
            ['POST', '/payments/(\d+)/delete', function (int $id) { $this->repo->deletePayment($id); self::back('/payments', 'Cuota eliminada'); }],
        ];
    }

    // ---------- dashboard ----------

    private function dashboard(): void
    {
        $today = date('Y-m-d');
        $tasks = array_slice($this->repo->listTasks(['mode' => 'abiertas']), 0, 8);
        $overdue = 0;
        $payments = [];
        foreach ($this->repo->listPayments() as $p) {
            if ($p['paid_date'] !== null) {
                continue;
            }
            $overdue += paymentStatus($p, $today) === 'vencido' ? 1 : 0;
            if (count($payments) < 6) {
                $payments[] = $p;
            }
        }
        View::render('dashboard', compact('tasks', 'payments', 'overdue'), 'inicio', 'Inicio');
    }

    // ---------- tasks ----------

    private function tasksList(): void
    {
        $mode = (string) ($_GET['mode'] ?? 'abiertas');
        $q = (string) ($_GET['q'] ?? '');
        $pid = (int) ($_GET['project_id'] ?? 0);
        View::render('tasks', [
            'tasks' => $this->repo->listTasks(['mode' => $mode, 'search' => $q, 'project_id' => $pid]),
            'mode' => $mode, 'q' => $q, 'project_id' => $pid, 'projects' => $this->repo->listProjects(),
        ], 'tareas', 'Tareas');
    }

    private function taskForm(array $t, string $error = '', int $status = 200): void
    {
        View::render('task_form', ['t' => $t, 'projects' => $this->repo->listProjects(), 'next' => self::next()],
            'tareas', isset($t['id']) ? 'Editar tarea' : 'Nueva tarea', $status, $error);
    }

    private function taskSave(int $id): void
    {
        $t = [
            'project_id' => (int) ($_POST['project_id'] ?? 0), 'title' => self::post('title'), 'notes' => self::post('notes'),
            'status' => self::post('status'), 'priority' => (int) ($_POST['priority'] ?? 1), 'due_date' => self::optDate(self::post('due_date')),
        ];
        $id && $t['id'] = $id;
        try {
            $id ? $this->repo->updateTask($id, $t) : $this->repo->createTask($t);
        } catch (AppError $e) {
            $this->taskForm($t, $e->getMessage(), 422);
            return;
        }
        self::back('/tasks', 'Tarea guardada');
    }

    private function taskToggle(int $id): void
    {
        $t = $this->repo->getTask($id);
        $t['status'] = $t['status'] === 'hecho' ? 'pendiente' : 'hecho';
        $this->repo->updateTask($id, $t);
        self::back('/tasks');
    }

    // ---------- projects ----------

    private function projectsList(): void
    {
        $progress = $this->repo->taskProgress();
        $totals = $this->repo->allProjectTotals();
        $rows = array_map(fn (array $p) => $p + [
            'progress' => $progress[$p['id']] ?? ['done' => 0, 'total' => 0],
            'totals' => $totals[$p['id']] ?? ['scheduled_cents' => 0, 'paid_cents' => 0],
        ], $this->repo->listProjects());
        View::render('projects', ['rows' => $rows], 'proyectos', 'Proyectos');
    }

    private function projectShow(int $id): void
    {
        $p = $this->repo->getProject($id);
        View::render('project', [
            'p' => $p,
            'tasks' => $this->repo->listTasks(['mode' => 'todas', 'project_id' => $id]),
            'payments' => $this->repo->paymentsByProject($id),
            'totals' => $this->repo->projectTotals($id),
            'notes' => $this->repo->listNotes(['project_id' => $id]),
        ], 'proyectos', $p['name']);
    }

    private function projectForm(array $p, string $error = '', int $status = 200): void
    {
        View::render('project_form', ['p' => $p, 'clients' => $this->repo->listClients()],
            'proyectos', isset($p['id']) ? 'Editar proyecto' : 'Nuevo proyecto', $status, $error);
    }

    private function projectSave(int $id): void
    {
        $p = [
            'client_id' => (int) ($_POST['client_id'] ?? 0), 'name' => self::post('name'), 'description' => self::post('description'),
            'status' => self::post('status'), 'start_date' => self::optDate(self::post('start_date')),
            'due_date' => self::optDate(self::post('due_date')), 'currency' => self::post('currency'),
            'billing_type' => self::post('billing_type'), 'budget_cents' => 0,
        ];
        $id && $p['id'] = $id;
        if (self::post('budget') !== '') {
            $cents = parseMoney(self::post('budget'));
            if ($cents === null) {
                $this->projectForm($p, 'Monto inválido', 422);
                return;
            }
            $p['budget_cents'] = $cents;
        }
        try {
            $saved = $id ? $this->repo->updateProject($id, $p) : $this->repo->createProject($p);
        } catch (AppError $e) {
            $this->projectForm($p, $e->getMessage(), 422);
            return;
        }
        self::redirect('/projects/' . $saved['id'], 'Proyecto guardado');
    }

    // ---------- clients ----------

    private function clientShow(int $id, string $error = '', int $status = 200): void
    {
        $c = $this->repo->getClient($id);
        View::render('client', ['c' => $c, 'projects' => $this->repo->projectsByClient($id)], 'clientes', $c['name'], $status, $error);
    }

    private function clientForm(array $c, string $error = '', int $status = 200): void
    {
        View::render('client_form', ['c' => $c], 'clientes', isset($c['id']) ? 'Editar cliente' : 'Nuevo cliente', $status, $error);
    }

    private function clientSave(int $id): void
    {
        $c = ['name' => self::post('name'), 'company' => self::post('company'), 'email' => self::post('email'),
              'phone' => self::post('phone'), 'notes' => self::post('notes')];
        $id && $c['id'] = $id;
        try {
            $saved = $id ? $this->repo->updateClient($id, $c) : $this->repo->createClient($c);
        } catch (AppError $e) {
            $this->clientForm($c, $e->getMessage(), 422);
            return;
        }
        self::redirect('/clients/' . $saved['id'], 'Cliente guardado');
    }

    private function clientDelete(int $id): void
    {
        try {
            $this->repo->deleteClient($id);
        } catch (ClientHasProjects $e) {
            $this->clientShow($id, 'El cliente tiene proyectos; elimínalos o muévelos primero.', 409);
            return;
        }
        self::redirect('/clients', 'Cliente eliminado');
    }

    // ---------- payments ----------

    private function paymentForm(array $p, string $error = '', int $status = 200): void
    {
        View::render('payment_form', ['p' => $p, 'projects' => $this->repo->listProjects(), 'next' => self::next()],
            'cuotas', isset($p['id']) ? 'Editar cuota' : 'Nueva cuota', $status, $error);
    }

    private function paymentSave(int $id): void
    {
        $p = ['project_id' => (int) ($_POST['project_id'] ?? 0), 'scheduled_date' => self::post('scheduled_date'),
              'paid_date' => self::optDate(self::post('paid_date')), 'notes' => self::post('notes'), 'amount_cents' => 0];
        $id && $p['id'] = $id;
        $cents = parseMoney(self::post('amount'));
        if ($cents === null) {
            $this->paymentForm($p, 'Monto inválido', 422);
            return;
        }
        $p['amount_cents'] = $cents;
        try {
            $id ? $this->repo->updatePayment($id, $p) : $this->repo->createPayment($p);
        } catch (AppError $e) {
            $this->paymentForm($p, $e->getMessage(), 422);
            return;
        }
        self::back('/payments', 'Cuota guardada');
    }

    // ---------- notes ----------

    private function notesList(): void
    {
        $q = (string) ($_GET['q'] ?? '');
        $tag = Repo::normalizeTag((string) ($_GET['tag'] ?? ''));
        $project = (string) ($_GET['project_id'] ?? '');
        $project = $project === 'none' ? 'none' : ((int) $project > 0 ? (string) (int) $project : '');
        View::render('notes', [
            'notes' => $this->repo->listNotes([
                'search' => $q, 'tag' => $tag, 'unassigned' => $project === 'none', 'project_id' => (int) $project,
            ]),
            'tags' => $this->repo->noteTags(), 'projects' => $this->repo->listProjects(),
            'q' => $q, 'tag' => $tag, 'project' => $project,
        ], 'notas', 'Notas');
    }

    private function noteForm(array $n, string $error = '', int $status = 200): void
    {
        View::render('note_form', ['n' => $n, 'projects' => $this->repo->listProjects(), 'tags' => $this->repo->noteTags(), 'next' => self::next()],
            'notas', isset($n['id']) ? 'Editar nota' : 'Nueva nota', $status, $error);
    }

    private function noteSave(int $id): void
    {
        $n = [
            'project_id' => (int) ($_POST['project_id'] ?? 0) ?: null, 'title' => self::post('title'),
            'body' => str_replace("\r\n", "\n", self::post('body')), 'tags' => Repo::normalizeTags(self::post('tags')),
        ];
        $id && $n['id'] = $id;
        try {
            $id ? $this->repo->updateNote($id, $n) : $this->repo->createNote($n);
        } catch (AppError $e) {
            $this->noteForm($n, $e->getMessage() === 'title es obligatorio' ? 'El título es obligatorio' : $e->getMessage(), 422);
            return;
        }
        self::back('/notes', 'Nota guardada');
    }
}
