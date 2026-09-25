#!/bin/sh
# End-to-end checks of the auth / CSRF / RBAC / i18n pipeline against a running app.
# Runs inside a curl container; scripts/verify-app.sh does the orchestration.
#
# BASE must be a dotted hostname: libcurl refuses to store cookies for single-label
# hosts, which would silently break every authenticated assertion below.
set -u
BASE="${BASE:-http://app.test:8080}"
PASS=0; FAIL=0

check() { # check <description> <expected> <actual>
  if [ "$2" = "$3" ]; then PASS=$((PASS+1)); echo "  PASS  $1"
  else FAIL=$((FAIL+1)); echo "  FAIL  $1 (expected '$2', got '$3')"; fi
}
ok()   { PASS=$((PASS+1)); echo "  PASS  $1"; }
bad()  { FAIL=$((FAIL+1)); echo "  FAIL  $1"; }

EMAIL="${ADMIN_EMAIL:-admin@example.lv}"
PW="${ADMIN_PASSWORD:-correct-horse-battery-staple}"

echo "== health =="
code=$(curl -s -o /tmp/h -w '%{http_code}' "$BASE/healthz")
check "GET /healthz returns 200" 200 "$code"
check "healthz body is 'ok'" "ok" "$(cat /tmp/h)"

echo "== security headers =="
curl -s -D /tmp/hdr -o /dev/null "$BASE/login"
for h in "x-content-type-options: nosniff" "x-frame-options: DENY" "cache-control: no-store"; do
  name=$(echo "$h" | cut -d: -f1)
  got=$(grep -i "^$name:" /tmp/hdr | tr -d '\r' | cut -d' ' -f2-)
  check "$name header" "$(echo "$h" | cut -d' ' -f2-)" "$got"
done
csp=$(grep -i '^content-security-policy:' /tmp/hdr | tr -d '\r')
case "$csp" in *"default-src 'none'"*) ok "CSP starts from default-src 'none'";;
  *) bad "CSP missing default-src 'none': $csp";; esac
case "$csp" in *unsafe-inline*|*unsafe-eval*) bad "CSP allows unsafe script execution";;
  *) ok "CSP has no unsafe-inline / unsafe-eval";; esac

echo "== anonymous access is refused =="
code=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/")
check "GET / anonymous redirects" 303 "$code"
loc=$(curl -s -D - -o /dev/null "$BASE/" | grep -i '^location:' | tr -d '\r' | cut -d' ' -f2)
check "  ...to /login" "/login" "$loc"
code=$(curl -s -o /dev/null -w '%{http_code}' -H 'HX-Request: true' "$BASE/")
check "GET / anonymous via htmx is 401" 401 "$code"

echo "== login form =="
curl -s -c /tmp/jar -D /tmp/hdr2 -o /tmp/login.html "$BASE/login"
TOKEN=$(sed -n 's/.*name="csrf_token" value="\([^"]*\)".*/\1/p' /tmp/login.html | head -1)
[ -n "$TOKEN" ] && ok "login form carries a CSRF token" || bad "no CSRF token in the login form"
grep -qi 'set-cookie:.*fleet_csrf' /tmp/hdr2 && ok "CSRF nonce cookie issued" || bad "no CSRF nonce cookie"
grep -qi 'httponly' /tmp/hdr2 && ok "cookie is HttpOnly" || bad "cookie is not HttpOnly"

echo "== CSRF enforcement =="
code=$(curl -s -b /tmp/jar -o /dev/null -w '%{http_code}' -X POST \
  -d "email=$EMAIL&password=$PW" "$BASE/login")
check "POST /login without a CSRF token is refused" 403 "$code"
code=$(curl -s -b /tmp/jar -o /dev/null -w '%{http_code}' -X POST \
  -d "csrf_token=forged&email=$EMAIL&password=$PW" "$BASE/login")
check "POST /login with a forged CSRF token is refused" 403 "$code"

echo "== credentials =="
t_wrong=$(curl -s -b /tmp/jar -o /tmp/bad.html -w '%{time_total}' -X POST \
  -d "csrf_token=$TOKEN&email=$EMAIL&password=wrong-password" "$BASE/login")
code=$(curl -s -b /tmp/jar -o /dev/null -w '%{http_code}' -X POST \
  -d "csrf_token=$TOKEN&email=$EMAIL&password=wrong-password" "$BASE/login")
check "wrong password is refused" 401 "$code"
grep -q "Incorrect email or password" /tmp/bad.html && ok "generic error message" || bad "unexpected error body"
t_unknown=$(curl -s -b /tmp/jar -o /tmp/unknown.html -w '%{time_total}' -X POST \
  -d "csrf_token=$TOKEN&email=nobody@example.lv&password=wrong-password" "$BASE/login")
code=$(curl -s -b /tmp/jar -o /dev/null -w '%{http_code}' -X POST \
  -d "csrf_token=$TOKEN&email=nobody@example.lv&password=wrong-password" "$BASE/login")
check "unknown email gives the same status" 401 "$code"
# Every page carries a fresh CSP nonce, so strip it before comparing: what must not
# differ is the visible response, not the random per-request value.
sed 's/nonce="[^"]*"/nonce="X"/g' /tmp/bad.html > /tmp/bad.norm
sed 's/nonce="[^"]*"/nonce="X"/g' /tmp/unknown.html > /tmp/unknown.norm
if cmp -s /tmp/bad.norm /tmp/unknown.norm; then ok "unknown email and wrong password are byte-identical"
else bad "responses differ between unknown email and wrong password"; fi
echo "  INFO  response time: wrong password ${t_wrong}s vs unknown email ${t_unknown}s (argon2id dummy hash keeps these comparable)"

echo "== successful login =="
code=$(curl -s -b /tmp/jar -c /tmp/jar -D /tmp/hdr3 -o /dev/null -w '%{http_code}' -X POST \
  -d "csrf_token=$TOKEN&email=$EMAIL&password=$PW" "$BASE/login")
check "login succeeds" 303 "$code"
grep -qi 'set-cookie:.*fleet_session' /tmp/hdr3 && ok "session cookie issued" || bad "no session cookie"
SESSION=$(grep -i 'set-cookie:.*fleet_session' /tmp/hdr3 | sed 's/.*fleet_session=\([^;]*\).*/\1/' | tr -d '\r')
case "$(grep -i 'set-cookie:.*fleet_session' /tmp/hdr3 | tr -d '\r')" in
  *HttpOnly*SameSite=Lax*|*SameSite=Lax*HttpOnly*) ok "session cookie is HttpOnly + SameSite=Lax";;
  *) bad "session cookie attributes: $(grep -i 'set-cookie:.*fleet_session' /tmp/hdr3)";;
esac

echo "== authenticated access =="
# "/" has no content of its own — it exists only as a stable post-login redirect
# target (see handleHome) — so authenticated checks target /timeline, what an
# admin/dispatcher actually lands on.
code=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/")
check "GET / with no session redirects to login" 303 "$code"
code=$(curl -s -b /tmp/jar -o /tmp/home.html -w '%{http_code}' "$BASE/timeline")
check "GET /timeline with a session" 200 "$code"
grep -q 'action="/logout"' /tmp/home.html && ok "the timeline page is a real authenticated page" || bad "unexpected timeline page content"
code=$(curl -s -b /tmp/jar -o /dev/null -w '%{http_code}' "$BASE/login")
check "GET /login while signed in redirects" 303 "$code"

echo "== i18n =="
curl -s -b /tmp/jar -H 'Accept-Language: lv-LV,lv;q=0.9' -o /tmp/lv.html "$BASE/timeline"
grep -q "Autobusu parks" /tmp/lv.html && ok "Latvian rendered from Accept-Language" || bad "no Latvian copy"
curl -s -H 'Accept-Language: ru-RU,ru;q=0.9' -o /tmp/ru.html "$BASE/login"
grep -q "Вход" /tmp/ru.html && ok "Russian rendered from Accept-Language" || bad "no Russian copy"

echo "== logout invalidates server-side =="
SESSION_TOKEN=$(sed -n 's/.*name="csrf_token" value="\([^"]*\)".*/\1/p' /tmp/home.html | head -1)
code=$(curl -s -b /tmp/jar -o /dev/null -w '%{http_code}' -X POST "$BASE/logout")
check "POST /logout without CSRF is refused" 403 "$code"
code=$(curl -s -b /tmp/jar -o /dev/null -w '%{http_code}' -X POST -d "csrf_token=$SESSION_TOKEN" "$BASE/logout")
check "POST /logout with CSRF succeeds" 303 "$code"
code=$(curl -s -H "Cookie: fleet_session=$SESSION" -o /dev/null -w '%{http_code}' "$BASE/")
check "the old session cookie no longer works" 303 "$code"

echo "== rate limiting on the login endpoint =="
curl -s -c /tmp/jar2 -o /tmp/l2.html "$BASE/login" >/dev/null
T2=$(sed -n 's/.*name="csrf_token" value="\([^"]*\)".*/\1/p' /tmp/l2.html | head -1)
last=""; i=0
while [ $i -lt 12 ]; do
  last=$(curl -s -b /tmp/jar2 -o /dev/null -w '%{http_code}' -X POST \
    -d "csrf_token=$T2&email=$EMAIL&password=nope" "$BASE/login")
  i=$((i+1))
done
check "repeated failed logins are rate limited" 429 "$last"

echo
echo "passed=$PASS failed=$FAIL"
[ "$FAIL" -eq 0 ]
