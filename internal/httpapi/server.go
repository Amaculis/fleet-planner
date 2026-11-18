package httpapi

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/buscompany/bus_fleet/internal/config"
	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/i18n"
	"github.com/buscompany/bus_fleet/internal/service"
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
		// Same defaults as config.Load() — see the comment there on why the SPA's
		// chunk waterfall on a cold load needs a generous burst.
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

	// lx-ui's stylesheet refers to its fonts by absolute path (/lx-fonts/...), whatever
	// base the app is built for, so they are served from the site root.
	r.Handle("/lx-fonts/*", s.fontsHandler())

	// The lx-ui SPA (portal/). Served straight from disk, see config.PortalDir. No
	// session/CSRF middleware: the SPA authenticates itself through /api/auth/* once
	// loaded, the same way any static asset needs no session to be fetched.
	r.Handle("/app/*", s.portalHandler())

	// Bookmarks and the old server-rendered UI's URLs lead into the portal.
	r.Get("/", redirectTo("/app/"))
	r.Get("/login", redirectTo("/app/login"))

	// A service worker registered at "/" by the retired server-rendered UI may still
	// sit on a user's phone. This replacement unregisters it (see serviceWorkerRetired).
	r.Get("/sw.js", s.handleRetiredServiceWorker)

	r.Group(func(r chi.Router) {
		// Static assets (above) are deliberately outside this limiter: lx-ui's cold
		// load fetches a hundred-odd small chunks, and several concurrent page loads
		// would exhaust any sane per-IP budget. Flood protection belongs on the
		// dynamic, session-bearing endpoints below, not on cacheable static files.
		r.Use(s.rateLimit(s.globalLimiter))
		r.Use(s.LoadSession)
		r.Use(s.CSRF)

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

	r.Get("/api/my/trips", s.handleAPIMyTrips)
	r.Post("/api/my/trips/{id}/start", s.handleAPIMyTripStart)
	r.Post("/api/my/trips/{id}/finish", s.handleAPIMyTripFinish)
}

// fontsHandler serves the lx-ui fonts the portal build ships in <PortalDir>/lx-fonts.
func (s *Server) fontsHandler() http.Handler {
	dir := http.Dir(filepath.Join(s.cfg.PortalDir, "lx-fonts"))
	files := http.StripPrefix("/lx-fonts/", http.FileServer(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Font files never change under the same name.
		w.Header().Set("Cache-Control", "public, max-age=86400")
		files.ServeHTTP(w, r)
	})
}

func redirectTo(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, path, http.StatusFound)
	}
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

// serviceWorkerRetired replaces the worker the old server-rendered UI registered at
// "/": it drops that worker's caches and unregisters itself. The portal's own worker
// lives at /app/pwa-sw.js with scope /app/ and is unaffected.
const serviceWorkerRetired = `self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.map((key) => caches.delete(key))))
      .then(() => self.registration.unregister())
  );
});
`

func (s *Server) handleRetiredServiceWorker(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = io.WriteString(w, serviceWorkerRetired)
}

// renderError answers with a generic, translated message. Internal detail never reaches
// the client; it is logged against the request id instead. API callers get JSON, anyone
// else plain text.
func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, messageKey string) {
	s.renderErrorText(w, r, status, s.printerFor(r).T(messageKey))
}

// renderErrorText answers with an already-translated message.
func (s *Server) renderErrorText(w http.ResponseWriter, r *http.Request, status int, text string) {
	if isAPI(r) {
		s.writeJSON(w, r, status, apiError{Error: text})
		return
	}
	http.Error(w, text, status)
}

// printerFor never returns nil, even for a request that failed before LoadSession ran.
func (s *Server) printerFor(r *http.Request) *i18n.Printer {
	if printer := PrinterFrom(r.Context()); printer != nil {
		return printer
	}
	return s.i18n.Printer(i18n.Fallback)
}
