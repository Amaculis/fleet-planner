#!/bin/sh
# End-to-end checks of the auth / CSRF / RBAC pipeline of the JSON API against a running
# app. Runs inside a curl container; scripts/verify-app.sh does the orchestration.
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
ok()  { PASS=$((PASS+1)); echo "  PASS  $1"; }
bad() { FAIL=$((FAIL+1)); echo "  FAIL  $1"; }

# csrf_of <file>: the csrfToken field of a JSON response body.
csrf_of() { sed -n 's/.*"csrfToken":"\([^"]*\)".*/\1/p' "$1" | head -1; }

# login_json <jar> <token> <email> <password>: POST /api/auth/login, prints the status.
login_json() {
  curl -s -b "$1" -c "$1" -o /tmp/login.json -w '%{http_code}' -X POST \
    -H 'Content-Type: application/json' -H "X-CSRF-Token: $2" \
    -d "{\"email\":\"$3\",\"password\":\"$4\"}" "$BASE/api/auth/login"
}

EMAIL="${ADMIN_EMAIL:-admin@example.lv}"
PW="${ADMIN_PASSWORD:-correct-horse-battery-staple}"

echo "== health =="
code=$(curl -s -o /tmp/h -w '%{http_code}' "$BASE/healthz")
check "GET /healthz returns 200" 200 "$code"
check "healthz body is 'ok'" "ok" "$(cat /tmp/h)"

echo "== security headers =="
curl -s -c /tmp/jar -D /tmp/hdr -o /tmp/me.json "$BASE/api/auth/me"
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

echo "== old UI addresses lead into the portal =="
check "GET / redirects" 302 "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/")"
check "  ...to /app/" "/app/" "$(curl -s -D - -o /dev/null "$BASE/" | grep -i '^location:' | tr -d '\r' | cut -d' ' -f2)"

echo "== anonymous access is refused =="
check "GET /api/trips anonymous is 401" 401 "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/api/trips")"
check "GET /api/users anonymous is 401" 401 "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/api/users")"

echo "== CSRF bootstrap =="
TOKEN=$(csrf_of /tmp/me.json)
[ -n "$TOKEN" ] && ok "/api/auth/me carries an anonymous CSRF token" || bad "no CSRF token from /api/auth/me"
grep -qi 'set-cookie:.*fleet_csrf' /tmp/hdr && ok "CSRF nonce cookie issued" || bad "no CSRF nonce cookie"
grep -qi 'httponly' /tmp/hdr && ok "cookie is HttpOnly" || bad "cookie is not HttpOnly"

echo "== CSRF enforcement =="
code=$(curl -s -b /tmp/jar -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PW\"}" "$BASE/api/auth/login")
check "POST /api/auth/login without a CSRF token is refused" 403 "$code"
code=$(login_json /tmp/jar forged "$EMAIL" "$PW")
check "POST /api/auth/login with a forged CSRF token is refused" 403 "$code"

echo "== credentials =="
code=$(login_json /tmp/jar "$TOKEN" "$EMAIL" "wrong-password")
check "wrong password is refused" 401 "$code"
cp /tmp/login.json /tmp/bad.json
code=$(login_json /tmp/jar "$TOKEN" "nobody@example.lv" "wrong-password")
check "unknown email gives the same status" 401 "$code"
if cmp -s /tmp/bad.json /tmp/login.json; then ok "unknown email and wrong password give identical responses"
else bad "responses differ between unknown email and wrong password"; fi

echo "== successful login =="
code=$(curl -s -b /tmp/jar -c /tmp/jar -D /tmp/hdr3 -o /tmp/ok.json -w '%{http_code}' -X POST \
  -H 'Content-Type: application/json' -H "X-CSRF-Token: $TOKEN" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PW\"}" "$BASE/api/auth/login")
check "login succeeds" 200 "$code"
grep -qi 'set-cookie:.*fleet_session' /tmp/hdr3 && ok "session cookie issued" || bad "no session cookie"
SESSION=$(grep -i 'set-cookie:.*fleet_session' /tmp/hdr3 | sed 's/.*fleet_session=\([^;]*\).*/\1/' | tr -d '\r')
case "$(grep -i 'set-cookie:.*fleet_session' /tmp/hdr3 | tr -d '\r')" in
  *HttpOnly*SameSite=Lax*|*SameSite=Lax*HttpOnly*) ok "session cookie is HttpOnly + SameSite=Lax";;
  *) bad "session cookie attributes: $(grep -i 'set-cookie:.*fleet_session' /tmp/hdr3)";;
esac
SESSION_TOKEN=$(csrf_of /tmp/ok.json)

echo "== authenticated access =="
check "GET /api/trips with a session" 200 "$(curl -s -b /tmp/jar -o /dev/null -w '%{http_code}' "$BASE/api/trips")"
check "GET /api/my/trips as an admin is forbidden" 403 "$(curl -s -b /tmp/jar -o /dev/null -w '%{http_code}' "$BASE/api/my/trips")"

echo "== i18n =="
curl -s -H 'X-App-Locale: lv' -o /tmp/lv.json "$BASE/api/trips"
grep -q "piekļuves" /tmp/lv.json && ok "Latvian error text from X-App-Locale" || bad "no Latvian error text: $(cat /tmp/lv.json)"

echo "== logout invalidates server-side =="
code=$(curl -s -b /tmp/jar -o /dev/null -w '%{http_code}' -X POST "$BASE/api/auth/logout")
check "POST /api/auth/logout without CSRF is refused" 403 "$code"
code=$(curl -s -b /tmp/jar -o /dev/null -w '%{http_code}' -X POST -H "X-CSRF-Token: $SESSION_TOKEN" "$BASE/api/auth/logout")
check "POST /api/auth/logout with CSRF succeeds" 200 "$code"
code=$(curl -s -H "Cookie: fleet_session=$SESSION" -o /dev/null -w '%{http_code}' "$BASE/api/trips")
check "the old session cookie no longer works" 401 "$code"

echo "== rate limiting on the login endpoint =="
curl -s -c /tmp/jar2 -o /tmp/me2.json "$BASE/api/auth/me" >/dev/null
T2=$(csrf_of /tmp/me2.json)
last=""; i=0
while [ $i -lt 12 ]; do
  last=$(login_json /tmp/jar2 "$T2" "$EMAIL" "nope")
  i=$((i+1))
done
check "repeated failed logins are rate limited" 429 "$last"

echo
echo "passed=$PASS failed=$FAIL"
[ "$FAIL" -eq 0 ]
