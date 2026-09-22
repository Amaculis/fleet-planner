.PHONY: generate build test vet fmt vuln verify-schema verify-app up down

# Pinned tool versions. Go 1.26+ is required: the module graph (x/text, x/crypto,
# x/time) needs it, and so does the templ generator.
SQLC_IMAGE  ?= sqlc/sqlc:1.28.0
TEMPL_VER   ?= v0.3.1020
VULN_VER    ?= latest

# Code generation. sqlc runs from its official image: `go run sqlc` uses a wasm build of
# the Postgres parser that crashes ("out of bounds memory access") in this environment.
# Both outputs are committed, so building the app needs neither tool.
generate:
	docker run --rm -v "$(CURDIR):/src" -w /src $(SQLC_IMAGE) generate
	go run github.com/a-h/templ/cmd/templ@$(TEMPL_VER) generate

build:
	go build ./...

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@$(VULN_VER) ./...

# Migrations up -> invariant smoke test -> down -> up, in a throwaway container.
verify-schema:
	bash scripts/verify-schema.sh

# Boots db + migrations + app and runs the auth/CSRF/RBAC/i18n smoke test against it.
verify-app:
	bash scripts/verify-app.sh

up:
	docker compose up --build

down:
	docker compose down
