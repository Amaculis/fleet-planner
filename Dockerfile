# syntax=docker/dockerfile:1
#
# Three stages: build the portal, build the binary, ship both.
#
# Generated Go code (sqlc -> internal/repo/sqlcgen) is committed, so this build needs
# no code generators. Run `make generate` after changing db/queries/*.sql.
#
# Base images are pinned by digest (tag kept for readability): a re-pushed tag cannot
# change what gets built. Bump tag and digest together.

# --- portal: the lx-ui Vue SPA (see docs/lx-ui-integration.md) ----------------------
# Its output is NOT go:embed'd into the binary (see config.PortalDir): it is copied
# straight into the runtime image below and served from disk.
FROM node:26-alpine@sha256:8996bb5821d62a8fd9bb27c1cb07462f3fdce6e47d640c6a881ead0b064ce216 AS portal
WORKDIR /src/portal
COPY portal/package.json portal/package-lock.json ./
# npm ci installs exactly what the lockfile says and fails if it disagrees with
# package.json. The lockfile carries the optional per-platform
# binaries (esbuild, rolldown) for linux-musl, so this works inside alpine.
RUN npm ci --no-audit --no-fund
COPY portal ./
RUN npm run build

# --- binary -------------------------------------------------------------------------
FROM golang:1.27-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# --- runtime: distroless, no shell, no package manager, nonroot ---------------------
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/server /app/server
# PORTAL_DIR defaults to "portal/dist", read relative to the working directory the
# binary runs in — set it to match wherever this lands if that ever changes.
COPY --from=portal /src/portal/dist /app/portal/dist
WORKDIR /app
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/server"]
