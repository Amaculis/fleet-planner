package httpapi

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
	tripSeries  *service.TripSeriesService
	clients     *service.ClientService
	assignments *service.AssignmentService
	users       *service.UserService
	i18n        *i18n.Bundle
	db          Pinger

	// Two limiters: a strict one for credential endpoints, a loose global default.
	loginLimiter  *Limiter
	globalLimiter *Limiter
	// Per account (email), spent only by failed logins: stops a distributed guess at one
	// password that the per-IP limiter cannot see. Keyed on whatever email was typed,
	// so it reveals nothing about which accounts exist.
	accountLimiter *Limiter
}

// Services groups the business services the HTTP layer drives.
type Services struct {
	Auth        *service.AuthService
	Fleet       *service.FleetService
	Trips       *service.TripService
	TripSeries  *service.TripSeriesService
	Clients     *service.ClientService
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
		tripSeries:  svc.TripSeries,
		clients:     svc.Clients,
		assignments: svc.Assignments,
		users:       svc.Users,
		i18n:        bundle,
		db:          db,
		// Strict on credentials, loose globally. Both per client IP.
		loginLimiter: NewLimiter(orDuration(cfg.LoginRateEvery, time.Minute), orInt(cfg.LoginRateBurst, 5)),
		// Same defaults as config.Load() — see the comment there on why 120/500ms
		// (sized for the old server-rendered app) is too tight for the SPA's chunk
		// waterfall on a cold load.
		globalLimiter: NewLimiter(orDuration(cfg.GlobalRateEvery, 100*time.Millisecond), orInt(cfg.GlobalRateBurst, 600)),
		// 10 failures, then one more try every 3 minutes (full refill: 30 minutes).
		accountLimiter: NewLimiter(3*time.Minute, 10).WithIdleTTL(30 * time.Minute),
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
	s.accountLimiter.StartCleanup(ctx)
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

	// Liveness/readiness for the container healthcheck. No session, no PII, no details.
	r.Get("/healthz", s.handleHealth)

	// Static assets: embedded in the binary, long-lived cache, no cookies needed.
	r.Handle("/static/*", s.staticHandler())

	// lx-ui's pre-compiled dist bundle computes its own asset URLs — chunk imports and
	// its @font-face url()s alike — as new URL(path, window.location.origin) /
	// root-relative paths, resolving to the site root (/js/..., /css/..., /lx-fonts/...)
	// regardless of where the files were actually deployed (/static/...). That logic
	// is compiled into the dependency, not something this app's own Vite config
	// controls. main.js also sets createLx's publicUrl option, which may be the
	// "intended" fix — unverified without a real browser — so both are in place:
	// whichever one lx-ui's runtime actually honours, the files exist where it looks.
	// Named prefixes only (never the whole /static/ tree) so this can't shadow a real
	// app route later.
	r.Handle("/js/*", s.rootStaticHandler())
	r.Handle("/css/*", s.rootStaticHandler())
	r.Handle("/lx-fonts/*", s.rootStaticHandler())

	// The lx-ui SPA (portal/). Served straight from disk, not go:embed'd — see
	// config.PortalDir. No session/CSRF middleware here: the SPA authenticates
	// itself entirely through /api/auth/* once loaded (see api_auth.go), the same
	// way any other static asset needs no session to be fetched.
	r.Handle("/app/*", s.portalHandler())

	// The service worker must be served from the root to control the whole origin;
	// /static/sw.js would only ever control /static/.
	r.Get("/sw.js", s.handleServiceWorker)

	r.Group(func(r chi.Router) {
		// Static assets (above) are deliberately outside this limiter: lx-ui's cold
		// load fetches a hundred-odd small chunks, and several concurrent page loads
		// would exhaust any sane per-IP budget. Flood protection belongs on the dynamic,
		// session-bearing endpoints below, not on cacheable static files.
		r.Use(s.rateLimit(s.globalLimiter))
		r.Use(s.LoadSession)
		r.Use(s.CSRF)

		// Public.
		// The offline page is cached by the service worker, so it must render without
		// a session and must never contain data.
		r.Get("/offline", s.handleOffline)
		r.Get("/login", s.handleLoginForm)
		r.With(s.rateLimit(s.loginLimiter)).Post("/login", s.handleLoginSubmit)

		// JSON API for the lx-ui SPA (portal/). Same LoadSession + CSRF pipeline as every
		// other route here -- no separate auth mechanism, just a different response
		// format. /api/auth/me is how the SPA bootstraps on every fresh load: it works
		// whether or not a session cookie is present.
		r.Route("/api/auth", func(r chi.Router) {
			r.Get("/me", s.handleAPIMe)
			r.With(s.rateLimit(s.loginLimiter)).Post("/login", s.handleAPILogin)
			r.With(s.RequireAuth).Post("/logout", s.handleAPILogout)
		})

		// Authenticated.
		r.Group(func(r chi.Router) {
			r.Use(s.RequireAuth)

			r.Post("/logout", s.handleLogout)
			r.Get("/", s.handleHome)

			// Planning: dispatchers and admins. Reading the fleet is included,
			// because you cannot assign what you cannot see.
			r.Group(func(r chi.Router) {
				r.Use(s.RequireRole(domain.RoleAdmin, domain.RoleDispatcher))
				s.mountPlannerRoutes(r)
			})

			// Fleet and user management: admins only.
			r.Group(func(r chi.Router) {
				r.Use(s.RequireRole(domain.RoleAdmin))
				s.mountAdminRoutes(r)
			})

			// The driver's own trips. No route here takes a driver id: ownership comes
			// from the session and is re-checked inside the SQL statement.
			r.Group(func(r chi.Router) {
				r.Use(s.RequireRole(domain.RoleDriver))
				s.mountDriverRoutes(r)
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

// mountPlannerRoutes registers the planning routes (admin + dispatcher): trips, and read-only fleet
// access, because you cannot assign what you cannot see. Includes the JSON mirror for the SPA (portal/).
func (s *Server) mountPlannerRoutes(r chi.Router) {
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

	// JSON mirror of the group above, for the SPA (portal/).
	r.Get("/api/trips", s.handleAPITripList)
	r.Post("/api/trips", s.handleAPITripCreate)
	r.Post("/api/trip-series", s.handleAPITripSeriesCreate)
	r.Get("/api/trips/{id}", s.handleAPITripGet)
	r.Put("/api/trips/{id}", s.handleAPITripUpdate)
	r.Delete("/api/trips/{id}", s.handleAPITripDelete)
	r.Post("/api/trips/{id}/status", s.handleAPITripStatus)
	r.Post("/api/trips/{id}/assign", s.handleAPITripAssign)
	r.Post("/api/trips/{id}/unassign", s.handleAPITripUnassign)
	r.Get("/api/buses", s.handleAPIBusList)
	r.Get("/api/drivers", s.handleAPIDriverList)
	r.Get("/api/clients", s.handleAPIClientList)
	r.Get("/api/clients/{id}", s.handleAPIClientGet)
	r.Post("/api/clients", s.handleAPIClientCreate)
	r.Put("/api/clients/{id}", s.handleAPIClientUpdate)
}

// mountAdminRoutes registers fleet and user management (admin only), with the JSON mirror for the SPA (portal/).
func (s *Server) mountAdminRoutes(r chi.Router) {
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

	// JSON mirror of the group above, for the SPA (portal/).
	r.Get("/api/buses/{id}", s.handleAPIBusGet)
	r.Post("/api/buses", s.handleAPIBusCreate)
	r.Put("/api/buses/{id}", s.handleAPIBusUpdate)
	r.Delete("/api/buses/{id}", s.handleAPIBusDelete)

	r.Get("/api/drivers/{id}", s.handleAPIDriverGet)
	r.Post("/api/drivers", s.handleAPIDriverCreate)
	r.Put("/api/drivers/{id}", s.handleAPIDriverUpdate)
	r.Post("/api/drivers/{id}/anonymize", s.handleAPIDriverAnonymize)
	r.Post("/api/clients/{id}/anonymize", s.handleAPIClientAnonymize)

	r.Get("/api/users", s.handleAPIUserList)
	r.Post("/api/users", s.handleAPIUserCreate)
	r.Post("/api/users/{id}/activate", s.handleAPIUserActivate)
	r.Post("/api/users/{id}/deactivate", s.handleAPIUserDeactivate)
	r.Post("/api/users/{id}/password", s.handleAPIUserSetPassword)
}

// mountDriverRoutes registers the driver's own trips. No route takes a driver id: ownership comes from the
// session and is re-checked inside the SQL statement.
func (s *Server) mountDriverRoutes(r chi.Router) {
	r.Get("/my/trips", s.handleMyTrips)
	r.Post("/my/trips/{id}/start", s.handleMyTripStart)
	r.Post("/my/trips/{id}/finish", s.handleMyTripFinish)

	// JSON mirror of the group above, for the SPA (portal/).
	r.Get("/api/my/trips", s.handleAPIMyTrips)
	r.Post("/api/my/trips/{id}/start", s.handleAPIMyTripStart)
	r.Post("/api/my/trips/{id}/finish", s.handleAPIMyTripFinish)
}

func (s *Server) staticHandler() http.Handler {
	fs := http.FileServer(http.FS(web.StaticFS()))
	return http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		fs.ServeHTTP(w, r)
	}))
}

// rootStaticHandler serves the same embedded tree with no prefix stripped — see the
// comment above its /js/ and /css/ mount points in Routes().
func (s *Server) rootStaticHandler() http.Handler {
	fs := http.FileServer(http.FS(web.StaticFS()))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		fs.ServeHTTP(w, r)
	})
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
