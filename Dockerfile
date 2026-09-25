# syntax=docker/dockerfile:1
#
# Three stages: build the front-end assets, build the binary, ship the binary.
#
# Generated Go code (sqlc -> internal/repo/sqlcgen, templ -> web/templates/*_templ.go)
# is committed, so this build needs no code generators. Run `make generate` after
# changing db/queries/*.sql or a .templ file.

# --- assets: Tailwind stylesheet, vendored htmx, the lx-ui calendar island ---------
FROM node:22-alpine AS assets
WORKDIR /src
COPY package.json vite.calendar.config.mjs ./
RUN npm install --no-audit --no-fund
COPY web ./web
RUN mkdir -p web/static/css web/static/js web/static/lx-fonts \
 && cp node_modules/htmx.org/dist/htmx.min.js web/static/js/htmx.min.js \
 && npx tailwindcss -i web/src/app.css -o web/static/css/app.css --minify \
 && npx vite build --config vite.calendar.config.mjs \
 && cp node_modules/@dativa-lv/lx-ui/dist/lx-fonts/IBMPlexSansVar.ttf \
       node_modules/@dativa-lv/lx-ui/dist/lx-fonts/IBMPlexSansVar-Italic.ttf \
       node_modules/@dativa-lv/lx-ui/dist/lx-fonts/IBMPlexMono-Regular.ttf \
       node_modules/@dativa-lv/lx-ui/dist/lx-fonts/IBMPlexMono-Italic.ttf \
       node_modules/@dativa-lv/lx-ui/dist/lx-fonts/IBMPlexMono-Light.ttf \
       node_modules/@dativa-lv/lx-ui/dist/lx-fonts/IBMPlexMono-LightItalic.ttf \
       node_modules/@dativa-lv/lx-ui/dist/lx-fonts/IBMPlexMono-SemiBold.ttf \
       node_modules/@dativa-lv/lx-ui/dist/lx-fonts/IBMPlexMono-SemiBoldItalic.ttf \
       web/static/lx-fonts/
# Only the 8 weights calendar-island.css actually references (grep 'url(' for the
# list) — lx-ui ships ~50 across five other font families (Poppins, Ubuntu, Roboto,
# Geist, Departure) for themes this app doesn't use; no reason to ship those too.

# --- portal: the lx-ui Vue SPA (see docs/lx-ui-integration.md) ----------------------
# A separate stage, not part of `assets`: this is a full npm project of its own
# (portal/package.json), not a handful of files copied into web/static, and its
# output is NOT go:embed'd into the binary (see config.PortalDir) — it is copied
# straight into the runtime image below and served from disk.
FROM node:22-alpine AS portal
WORKDIR /src/portal
COPY portal/package.json portal/package-lock.json ./
# npm install, not ci: `npm ci` repeatedly failed here with "Missing: @esbuild/...
# from lock file" for optional per-platform esbuild binaries — reproducible even
# starting from a lockfile just generated fresh inside this exact image, so it isn't
# the earlier Windows-vs-Linux mismatch this comment used to describe. package.json
# pins vite (and now @playwright/test) to exact versions, which is what actually
# stops the different-rolldown-version problem `npm ci` was originally reached for;
# `npm install` self-heals a lockfile npm's own optional-dependency bookkeeping
# didn't fully write out, the same way the `assets` stage above already does.
RUN npm install --no-audit --no-fund
COPY portal ./
RUN npm run build

# --- binary -------------------------------------------------------------------------
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
# The whole asset tree must exist before `go build`: web/embed.go bakes web/static
# into the binary at compile time. Copied as one tree (not file-by-file) because the
# calendar island's build also emits lazily-loaded chunks under js/calendar-assets/
# whose filenames are content-hashed.
COPY --from=assets /src/web/static ./web/static
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# --- runtime: distroless, no shell, no package manager, nonroot ---------------------
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /app/server
# PORTAL_DIR defaults to "portal/dist", read relative to the working directory the
# binary runs in — set it to match wherever this lands if that ever changes.
COPY --from=portal /src/portal/dist /app/portal/dist
WORKDIR /app
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/server"]
