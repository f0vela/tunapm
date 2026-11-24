<?php
declare(strict_types=1);

namespace Tuna;

/** Error de dominio con código estable (el cliente Go lo traduce a sus errores). */
class AppError extends \RuntimeException
{
    public function __construct(public readonly int $status, public readonly string $errCode, string $message)
    {
        parent::__construct($message);
    }
}

final class NotFound extends AppError
{
    public function __construct(string $m = 'no encontrado') { parent::__construct(404, 'not_found', $m); }
}
final class ClientHasProjects extends AppError
{
    public function __construct() { parent::__construct(409, 'client_has_projects', 'el cliente tiene proyectos'); }
}
final class TotalOneQuote extends AppError
{
    public function __construct() { parent::__construct(409, 'total_one_quote', 'un proyecto con cobro total solo admite una cuota'); }
}
final class InvalidAmount extends AppError
{
    public function __construct() { parent::__construct(422, 'invalid_amount', 'el monto de la cuota debe ser mayor que cero'); }
}
final class BadInput extends AppError
{
    public function __construct(string $m) { parent::__construct(422, 'bad_request', $m); }
}
