<?php
declare(strict_types=1);

namespace Tuna;

final class Api
{
    public function __construct(private Repo $repo, private string $token) {}

    public function handle(string $method, string $path): void
    {
        if ($path === '/api/health') {
            self::json(200, ['status' => 'ok']);
            return;
        }
        $this->authenticate();

        foreach ($this->routes() as [$m, $re, $fn]) {
            if ($m === $method && preg_match('#^' . $re . '$#', $path, $mm)) {
                $args = array_map('intval', array_slice($mm, 1));
                $fn(...$args);
                return;
            }
        }
        // Ruta existente con otro método → 405; si no, 404.
        foreach ($this->routes() as [, $re]) {
            if (preg_match('#^' . $re . '$#', $path)) {
                throw new AppError(405, 'method_not_allowed', 'método no permitido');
            }
        }
        throw new AppError(404, 'not_found', 'ruta no encontrada');
    }

    private function authenticate(): void
    {
        $h = $_SERVER['HTTP_AUTHORIZATION'] ?? $_SERVER['REDIRECT_HTTP_AUTHORIZATION'] ?? '';
        if ($h === '' && function_exists('getallheaders')) {
            foreach (getallheaders() as $k => $v) {
                if (strcasecmp($k, 'Authorization') === 0) {
                    $h = $v;
                }
            }
        }
        $got = str_starts_with($h, 'Bearer ') ? substr($h, 7) : '';
        if ($this->token === '' || !hash_equals($this->token, $got)) {
            usleep(300_000); // frena fuerza bruta barata
            throw new AppError(401, 'unauthorized', 'token inválido');
        }
    }

    /** @return list<array{0:string,1:string,2:callable}> */
    private function routes(): array
    {
        $r = $this->repo;
        $created = fn (array $row) => self::json(201, $row);
        $ok = fn (mixed $v) => self::json(200, $v);
        $none = fn () => self::noContent();
        $id = '(\d+)';

        return [
            ['GET', '/api/snapshot', fn () => $this->snapshot()],

            ['GET', '/api/clients', fn () => $ok($r->listClients())],
            ['POST', '/api/clients', fn () => $created($r->createClient(self::body()))],
            ['GET', "/api/clients/$id", fn (int $i) => $ok($r->getClient($i))],
            ['PUT', "/api/clients/$id", fn (int $i) => $ok($r->updateClient($i, self::body()))],
            ['DELETE', "/api/clients/$id", function (int $i) use ($r, $none) { $r->deleteClient($i); $none(); }],
            ['GET', "/api/clients/$id/projects", fn (int $i) => $ok($r->projectsByClient($i))],
            ['GET', "/api/clients/$id/project-count", fn (int $i) => $ok(['count' => $r->countProjectsByClient($i)])],

            ['GET', '/api/projects', fn () => $ok($r->listProjects())],
            ['POST', '/api/projects', fn () => $created($r->createProject(self::body()))],
            ['GET', '/api/projects/totals', fn () => $ok((object) $r->allProjectTotals())],
            ['GET', '/api/projects/progress', fn () => $ok((object) $r->taskProgress())],
            ['GET', "/api/projects/$id", fn (int $i) => $ok($r->getProject($i))],
            ['PUT', "/api/projects/$id", fn (int $i) => $ok($r->updateProject($i, self::body()))],
            ['DELETE', "/api/projects/$id", function (int $i) use ($r, $none) { $r->deleteProject($i); $none(); }],
            ['GET', "/api/projects/$id/payments", fn (int $i) => $ok($r->paymentsByProject($i))],
            ['GET', "/api/projects/$id/totals", fn (int $i) => $ok($r->projectTotals($i))],

            ['GET', '/api/tasks', fn () => $ok($r->listTasks([
                'mode' => (string) ($_GET['mode'] ?? ''),
                'search' => (string) ($_GET['search'] ?? ''),
                'project_id' => (int) ($_GET['project_id'] ?? 0),
                'client_id' => (int) ($_GET['client_id'] ?? 0),
            ]))],
            ['POST', '/api/tasks', fn () => $created($r->createTask(self::body()))],
            ['GET', "/api/tasks/$id", fn (int $i) => $ok($r->getTask($i))],
            ['PUT', "/api/tasks/$id", fn (int $i) => $ok($r->updateTask($i, self::body()))],
            ['DELETE', "/api/tasks/$id", function (int $i) use ($r, $none) { $r->deleteTask($i); $none(); }],

            ['GET', '/api/payments', fn () => $ok($r->listPayments())],
            ['POST', '/api/payments', fn () => $created($r->createPayment(self::body()))],
            ['GET', "/api/payments/$id", fn (int $i) => $ok($r->getPayment($i))],
            ['PUT', "/api/payments/$id", fn (int $i) => $ok($r->updatePayment($i, self::body()))],
            ['DELETE', "/api/payments/$id", function (int $i) use ($r, $none) { $r->deletePayment($i); $none(); }],
            ['POST', "/api/payments/$id/pay", function (int $i) use ($r, $none) {
                $r->markPaymentPaid($i, (string) (self::body()['date'] ?? ''));
                $none();
            }],
        ];
    }

    /** Todos los datos con ETag (304 si no cambió) y gzip si el cliente lo acepta. */
    private function snapshot(): void
    {
        $body = json_encode($this->repo->snapshot(), JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES | JSON_THROW_ON_ERROR);
        $etag = '"' . substr(hash('sha256', $body), 0, 32) . '"';
        header('ETag: ' . $etag);
        header('Cache-Control: no-cache');
        $sent = trim((string) ($_SERVER['HTTP_IF_NONE_MATCH'] ?? ''));
        if (str_starts_with($sent, 'W/')) {
            $sent = substr($sent, 2);
        }
        if ($sent === $etag) {
            http_response_code(304);
            return;
        }
        http_response_code(200);
        header('Content-Type: application/json; charset=utf-8');
        header('Vary: Accept-Encoding');
        if (str_contains((string) ($_SERVER['HTTP_ACCEPT_ENCODING'] ?? ''), 'gzip') && function_exists('gzencode')) {
            $body = (string) gzencode($body, 6);
            header('Content-Encoding: gzip');
        }
        header('Content-Length: ' . strlen($body));
        echo $body;
    }

    private static function body(): array
    {
        $raw = file_get_contents('php://input', false, null, 0, 1_048_577);
        if ($raw === false || strlen($raw) > 1_048_576) {
            throw new AppError(413, 'bad_request', 'cuerpo demasiado grande');
        }
        $data = json_decode($raw, true);
        if (!is_array($data)) {
            throw new AppError(400, 'bad_request', 'JSON inválido');
        }
        return $data;
    }

    public static function json(int $status, mixed $value): void
    {
        http_response_code($status);
        header('Content-Type: application/json; charset=utf-8');
        echo json_encode($value, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES | JSON_THROW_ON_ERROR);
    }

    public static function noContent(): void { http_response_code(204); }

    public static function error(int $status, string $code, string $message): void
    {
        self::json($status, ['code' => $code, 'error' => $message]);
    }
}
