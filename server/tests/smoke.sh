#!/usr/bin/env bash
# Prueba de humo del servidor PHP: API (con el cliente Go como contrato) + web.
# Uso: server/tests/smoke.sh   (requiere php y go)
set -euo pipefail
cd "$(dirname "$0")/../.."
TMP=$(mktemp -d); trap 'kill $PID 2>/dev/null || true; rm -rf "$TMP"' EXIT
PORT=${PORT:-18095}; U=http://127.0.0.1:$PORT
TUNA_DB=$TMP/t.db TUNA_TOKEN=secreto TUNA_WEB_PASSWORD=pw TUNA_REQUIRE_HTTPS=0 \
  php -S 127.0.0.1:$PORT -t server/public server/public/index.php >"$TMP/php.log" 2>&1 & PID=$!
sleep 1

echo "== contrato del API (tests del cliente Go)"
TUNA_TEST_URL=$U TUNA_TEST_TOKEN=secreto go test -count=1 ./internal/client

echo "== web"
fail() { echo "FALLO: $*"; cat "$TMP/php.log" | tail -5; exit 1; }
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }
J=$TMP/jar
[ "$(code $U/)" = 303 ] || fail "sin login debe redirigir"
[ "$(code -d password=mal $U/login)" = 401 ] || fail "login malo"
[ "$(code -c $J -d password=pw $U/login)" = 303 ] || fail "login"
for p in / /tasks /projects /projects/1 /clients /clients/1 /payments /tasks/new /projects/new /payments/new /clients/new /tasks/1/edit /static/app.css /notes /notes/new '/notes?project_id=none&tag=idea&q=x'; do
  [ "$(code -b $J $U$p)" = 200 ] || fail "GET $p"
done
[ "$(code -b $J $U/projects/999)" = 404 ] || fail "404 proyecto"
CSRF=$(curl -s -b $J $U/tasks/new | grep -o 'name="_csrf" value="[^"]*"' | head -1 | sed 's/.*value="//;s/"//')
[ -n "$CSRF" ] || fail "sin token csrf"
[ "$(code -b $J -d 'title=x&project_id=1' $U/tasks)" = 403 ] || fail "csrf debe bloquear"
[ "$(code -b $J --data-urlencode "_csrf=$CSRF" -d 'title=Desde+web&project_id=1&status=pendiente&priority=2&next=/tasks' $U/tasks)" = 303 ] || fail "crear tarea"
curl -s -H 'Authorization: Bearer secreto' "$U/api/tasks?search=Desde+web" | grep -q 'Desde web' || fail "la tarea no llegó a la base"
[ "$(code -b $J -d "_csrf=$CSRF&project_id=1&amount=abc&scheduled_date=2026-12-01" $U/payments)" = 422 ] || fail "monto inválido"
[ "$(code -b $J -d "_csrf=$CSRF&project_id=1&amount=1,500.50&scheduled_date=2026-12-01&next=//evil.com" $U/payments)" = 303 ] || fail "crear cuota"
[ "$(code -b $J -d "_csrf=$CSRF&next=/payments" $U/payments/1/pay)" = 303 ] || fail "pagar"
curl -s -H 'Authorization: Bearer secreto' $U/api/payments/1 | grep -q '"paid_date":"' || fail "no quedó pagada"
[ "$(code -b $J -d "_csrf=$CSRF&next=/payments" $U/payments/1/unpay)" = 303 ] || fail "deshacer pago"
curl -s -H 'Authorization: Bearer secreto' $U/api/payments/1 | grep -q '"paid_date":null' || fail "sigue pagada"
LOC=$(curl -s -b $J -o /dev/null -w '%{redirect_url}' -d "_csrf=$CSRF&next=//evil.com" $U/tasks/1/toggle)
case "$LOC" in *evil*) fail "open redirect: $LOC";; esac
# Notas: crear con proyecto y etiquetas, verlas en el API, editar (sin proyecto) y borrar.
[ "$(code -b $J --data-urlencode "_csrf=$CSRF" -d 'title=+&next=/notes' $U/notes)" = 422 ] || fail "nota sin título"
[ "$(code -b $J --data-urlencode "_csrf=$CSRF" --data-urlencode 'title=Nota web zzw' --data-urlencode 'tags=Reunión, #Idea idea' -d 'project_id=1&body=uno%0D%0Ados&next=/notes' $U/notes)" = 303 ] || fail "crear nota"
NOTE=$(curl -s -H 'Authorization: Bearer secreto' "$U/api/notes?search=zzw&tag=idea&project_id=1")
echo "$NOTE" | grep -q '"tags":\["reunión","idea"\]' || fail "etiquetas de la nota: $NOTE"
echo "$NOTE" | grep -q '"body":"uno\\ndos"' || fail "cuerpo de la nota: $NOTE"
NID=$(echo "$NOTE" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
for p in /notes "/notes/$NID/edit" /projects/1 '/notes?tag=idea'; do
  [ "$(code -b $J $U$p)" = 200 ] || fail "GET $p"
done
[ "$(code -b $J --data-urlencode "_csrf=$CSRF" --data-urlencode 'title=Nota web zzw 2' -d 'project_id=&tags=&next=/notes' $U/notes/$NID)" = 303 ] || fail "editar nota"
curl -s -H 'Authorization: Bearer secreto' "$U/api/notes?project_id=none&search=zzw" | grep -q '"project_id":null' || fail "la nota debió quedar sin proyecto"
[ "$(code -b $J --data-urlencode "_csrf=$CSRF" -d 'next=/notes' $U/notes/$NID/delete)" = 303 ] || fail "borrar nota"
[ "$(code -H 'Authorization: Bearer secreto' $U/api/notes/$NID)" = 404 ] || fail "la nota sigue existiendo"
echo "OK"
