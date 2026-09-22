package http

import (
	"context"
	"io"
	"net/http"
	"time"

	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/buscompany/bus_fleet/internal/config"
	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/i18n"
	"github.com/buscompany/bus_fleet/internal/service"
	"github.com/buscompany/bus_fleet/web"
	"github.com/buscompany/bus_fleet/web/templates"
)

// Pinger is the health check's view of the database.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Server struct {
	cfg         config.Config
	log         *slog.Logger
	auth        *service.AuthService
	fleet       *service.FleetService
	trips       *service.TripService
	assignments *service.AssignmentService
	users       *service.UserService
	i18n        *i18n.Bundle
	db          Pinger

	// Two limiters: a strict one for credential endpoints, a loose global default.
	loginLimiter  *Limiter
	globalLimiter *Limiter
}

// Services groups the business services the HTTP layer drives.
type Services struct {
	Auth        *service.AuthService
	Fleet       *service.FleetService
	Trips       *service.TripService
	Assignments *service.AssignmentService
	Users       *service.UserService
}

func NewServer(cfg config.Config, log *slog.Logger, svc Services, bundle *i18n.Bundle, db Pinger) *Server {
	return &Server{
		cfg:         cfg,
		log:         log,
		auth:        svc.Auth,
		fleet:       svc.Fleet,
		trips:       svc.Trips,
		assignments: svc.Assignments,
		users:       svc.Users,
		i18n:        bundle,
		db:          db,
		// Strict on credentials, loose globally. Both per client IP.
		loginLimiter:  NewLimiter(orDuration(cfg.LoginRateEvery, time.Minute), orInt(cfg.LoginRateBurst, 5)),
		globalLimiter: NewLimiter(orDuration(cfg.GlobalRateEvery, 500*time.Millisecond), orInt(cfg.GlobalRateBurst, 120)),
	}
}

// orDuration and orInt keep a zero-valued Config (tests, embedding) on the production
// defaults instead of a limiter that refuses everything.
func orDuration(v, fallback time.Duration) time.Duration {
	if v <= 0 {
		return fallback
	}
	return v
}

func orInt(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}

// StartBackground runs the limiters' cleanup loops; they stop with ctx.
func (s *Server) StartBackground(ctx context.Context) {
	s.loginLimiter.StartCleanup(ctx)
	s.globalLimiter.StartCleanup(ctx)
}

// Routes builds the router. Middleware order is fixed:
//
//	request-id -> recover -> security-headers -> rate-limit -> session -> csrf -> rbac
//
// request-id sits outside recover (rather than inside, as the conventions sketch it) so
// that a panic is logged with its request id; everything after recover is protected.
// Nothing is mounted outside this chain, so no route can accidentally skip a step.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(s.RequestID)
	r.Use(s.Recover)
	r.Use(s.SecurityHeaders)
	r.Use(s.rateLimit(s.globalLimiter))

	// Liveness/readiness for the container healthcheck. No session, no PII, no details.
	r.Get("/healthz", s.handleHealth)

	// Static assets: embedded in the binary, long-lived cache, no cookies needed.
	r.Handle("/static/*", s.staticHandler())

	// The service worker must be served from the root to control the whole origin;
	// /static/sw.js would only ever control /static/.
	r.Get("/sw.js", s.handleServiceWorker)

	r.Group(func(r chi.Router) {
		r.Use(s.LoadSession)
		r.Use(s.CSRF)

		// Public.
		// The offline page is cached by the service worker, so it must render without
		// a session and must never contain data.
		r.Get("/offline", s.handleOffline)
		r.Get("/login", s.handleLoginForm)
		r.With(s.rateLimit(s.loginLimiter)).Post("/login", s.handleLoginSubmit)

		// Authenticated.
		r.Group(func(r chi.Router) {
			r.Use(s.RequireAuth)

			r.Post("/logout", s.handleLogout)
			r.Get("/", s.handleHome)

			// Planning: dispatchers and admins. Reading the fleet is included,
			// because you cannot assign what you cannot see.
			r.Group(func(r chi.Router) {
				r.Use(s.RequireRole(domain.RoleAdmin, domain.RoleDispatcher))

				r.Get("/timeline", s.handleTimeline)
				r.Get("/trips", s.handleTripList)
				r.Get("/trips/new", s.handleTripNew)
				r.Post("/trips", s.handleTripCreate)
				r.Get("/trips/{id}", s.handleTripDetail)
				r.Get("/trips/{id}/edit", s.handleTripEdit)
				r.Post("/trips/{id}", s.handleTripUpdate)
				r.Post("/trips/{id}/status", s.handleTripStatus)
				r.Post("/trips/{id}/delete", s.handleTripDelete)
				r.Post("/trips/{id}/assign", s.handleTripAssign)
				r.Post("/trips/{id}/unassign", s.handleTripUnassign)

				r.Get("/buses", s.handleBusList)
				r.Get("/drivers", s.handleDriverList)
			})

			// Fleet and user management: admins only.
			r.Group(func(r chi.Router) {
				r.Use(s.RequireRole(domain.RoleAdmin))

				r.Get("/buses/new", s.handleBusNew)
				r.Post("/buses", s.handleBusCreate)
				r.Get("/buses/{id}/edit", s.handleBusEdit)
				r.Post("/buses/{id}", s.handleBusUpdate)
				r.Post("/buses/{id}/delete", s.handleBusDelete)

				r.Get("/drivers/new", s.handleDriverNew)
				r.Post("/drivers", s.handleDriverCreate)
				r.Get("/drivers/{id}/edit", s.handleDriverEdit)
				r.Post("/drivers/{id}", s.handleDriverUpdate)
				r.Post("/drivers/{id}/anonymize", s.handleDriverAnonymize)

				r.Get("/users", s.handleUserList)
				r.Get("/users/new", s.handleUserNew)
				r.Post("/users", s.handleUserCreate)
				r.Post("/users/{id}/activate", s.handleUserActivate)
				r.Post("/users/{id}/deactivate", s.handleUserDeactivate)
				r.Get("/users/{id}/password", s.handleUserPasswordForm)
				r.Post("/users/{id}/password", s.handleUserPasswordSet)
			})

			// The driver's own trips. No route here takes a driver id: ownership comes
			// from the session and is re-checked inside the SQL statement.
			r.Group(func(r chi.Router) {
				r.Use(s.RequireRole(domain.RoleDriver))

				r.Get("/my/trips", s.handleMyTrips)
				r.Post("/my/trips/{id}/start", s.handleMyTripStart)
				r.Post("/my/trips/{id}/finish", s.handleMyTripFinish)
			})
		})
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		s.renderError(w, r, http.StatusNotFound, "error.not_found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		s.renderError(w, r, http.StatusMethodNotAllowed, "error.not_found")
	})
	return r
}

func (s *Server) staticHandler() http.Handler {
	fs := http.FileServer(http.FS(web.StaticFS()))
	return http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		fs.ServeHTTP(w, r)
	}))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.Ping(ctx); err != nil {
		s.log.Error("health check failed", "error", err)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

// handleServiceWorker serves the worker from the origin root so its scope is "/".
// It is revalidated on every load: a stale worker would pin an old app shell.
func (s *Server) handleServiceWorker(w http.ResponseWriter, r *http.Request) {
	file, err := web.StaticFS().Open("sw.js")
	if err != nil {
		s.renderError(w, r, http.StatusNotFound, "error.not_found")
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Service-Worker-Allowed", "/")
	if _, err := io.Copy(w, file); err != nil {
		s.log.Error("serving service worker", "request_id", RequestIDFrom(r.Context()), "error", err)
	}
}

// handleOffline renders the page the service worker falls back to. Public and static:
// it holds no data, so caching it leaks nothing.
func (s *Server) handleOffline(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, templates.OfflinePage(s.view(r)))
}

// renderError renders a generic, translated message. Internal detail never reaches the
// client; it is logged against the request id instead.
func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, messageKey string) {
	s.renderErrorText(w, r, status, s.printerFor(r).T(messageKey))
}

// renderErrorText renders an already-translated message.
func (s *Server) renderErrorText(w http.ResponseWriter, r *http.Request, status int, text string) {
	printer := s.printerFor(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	page := templates.ErrorPage(printer, cspNonceFrom(r.Context()), text)
	if err := page.Render(r.Context(), w); err != nil {
		s.log.Error("rendering error page", "request_id", RequestIDFrom(r.Context()), "error", err)
	}
}

// printerFor never returns nil, even for a request that failed before LoadSession ran.
func (s *Server) printerFor(r *http.Request) *i18n.Printer {
	if printer := PrinterFrom(r.Context()); printer != nil {
		return printer
	}
	return s.i18n.Printer(i18n.Fallback)
}
