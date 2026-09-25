package http

import (
	"net/http"
	"path/filepath"
	"strings"
)

// portalHandler serves the built lx-ui SPA (config.PortalDir) at /app/, with a
// client-side-routing fallback: any /app/* path that looks like a route (no file
// extension — /app/login, /app/dashboard, ...) and is not a real file gets
// index.html instead, so vue-router's createWebHistory can resolve the route itself
// (see portal/src/router/index.js). A path that *does* look like a file
// (/app/assets/whatever.js) 404s normally when missing — silently handing back HTML
// for a missing script would turn a clean 404 into a confusing "Unexpected token '<'"
// in the browser instead. config.PortalDir unset, or the SPA never built, means
// nothing under /app/ resolves and the index.html fallback itself 404s too.
func (s *Server) portalHandler() http.Handler {
	dir := http.Dir(s.cfg.PortalDir)
	assets := http.StripPrefix("/app/", http.FileServer(dir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/app"), "/")
		if rel == "" {
			rel = "index.html"
		}

		// index.html itself must never take the cacheable-asset branch below, even
		// though it's a real file that "/" resolves to (rel defaults to it above) —
		// it's the one file whose content legitimately changes between deploys at a
		// fixed URL (unlike the content-hashed assets it references), and a stale
		// cached copy keeps referencing yesterday's JS/CSS hashes. Confirmed live: a
		// plain visit to /app/ was caching index.html for an hour, silently masking a
		// just-deployed CSS fix behind the previous build until the cache expired.
		if rel != "index.html" {
			if f, err := dir.Open("/" + rel); err == nil {
				f.Close()
				w.Header().Set("Cache-Control", "public, max-age=3600")
				assets.ServeHTTP(w, r)
				return
			}

			if filepath.Ext(rel) != "" {
				http.NotFound(w, r)
				return
			}
		}

		// Either the path names a route rather than an asset (SPA shell as a
		// client-side-routing fallback), or it resolved to index.html directly.
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, filepath.Join(string(dir), "index.html"))
	})
}
