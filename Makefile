.PHONY: generate build test test-unit vet fmt fmt-check vuln hooks portal-build portal-test verify-schema verify-app verify-e2e up down

# Pinned tool versions. Go 1.26+ is required: the module graph (x/text, x/crypto,
# x/time) needs it.
SQLC_IMAGE  ?= sqlc/sqlc:1.28.0
VULN_VER    ?= v1.9.0 # keep in step with .github/workflows/ci.yml

# Code generation. sqlc runs from its official image: `go run sqlc` uses a wasm build of
# the Postgres parser that crashes ("out of bounds memory access") in this environment.
# The output is committed, so building the app does not need sqlc.
generate:
	docker run --rm -v "$(CURDIR):/src" -w /src $(SQLC_IMAGE) generate

build:
	go build ./...

# Full suite: unit + integration tests against a throwaway Postgres (needs Docker).
test:
	REQUIRE_DB_TESTS=1 bash scripts/verify-tests.sh

# Unit tests only. The integration tests in internal/inttest are SKIPPED without a
# database, so a green result here does not cover them; use `make test`.
test-unit:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w cmd internal

# Fails when a file is not gofmt-formatted (used by CI).
fmt-check:
	@out="$$(gofmt -l cmd internal)"; if [ -n "$$out" ]; then echo "not formatted:"; echo "$$out"; exit 1; fi

# Enables the repo's git hooks (gofmt, vet, gosec before commit/push).
hooks:
	git config core.hooksPath scripts/hooks

portal-build:
	cd portal && npm ci && npm run build

portal-test:
	cd portal && npm ci && npm test

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@$(VULN_VER) ./...

# Migrations up -> invariant smoke test -> down -> up, in a throwaway container.
verify-schema:
	bash scripts/verify-schema.sh

# Boots db + migrations + app and runs the auth/CSRF/RBAC/i18n smoke test against it.
verify-app:
	bash scripts/verify-app.sh

# Playwright suite against the running stack (docker compose up -d first).
verify-e2e:
	bash scripts/verify-e2e.sh

up:
	docker compose up --build

down:
	docker compose down
