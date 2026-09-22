# syntax=docker/dockerfile:1
#
# Three stages: build the stylesheet, build the binary, ship the binary.
#
# Generated Go code (sqlc -> internal/repo/sqlcgen, templ -> web/templates/*_templ.go)
# is committed, so this build needs no code generators. Run `make generate` after
# changing db/queries/*.sql or a .templ file.

# --- assets: Tailwind stylesheet + vendored htmx -----------------------------------
FROM node:22-alpine AS assets
WORKDIR /src
COPY package.json ./
RUN npm install --no-audit --no-fund
COPY web ./web
RUN mkdir -p web/static/css web/static/js \
 && cp node_modules/htmx.org/dist/htmx.min.js web/static/js/htmx.min.js \
 && npx tailwindcss -i web/src/app.css -o web/static/css/app.css --minify

# --- binary -------------------------------------------------------------------------
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
# The stylesheet and htmx must exist before `go build`: web/embed.go bakes
# web/static into the binary at compile time.
COPY --from=assets /src/web/static/css/app.css ./web/static/css/app.css
COPY --from=assets /src/web/static/js/htmx.min.js ./web/static/js/htmx.min.js
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# --- runtime: distroless, no shell, no package manager, nonroot ---------------------
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /app/server
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/server"]
