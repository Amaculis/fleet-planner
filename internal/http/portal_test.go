package http

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPortalHandler(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>shell</html>"), 0o644); err != nil {
		t.Fatalf("writing index.html: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatalf("making assets dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatalf("writing app.js: %v", err)
	}

	s := testServer(t)
	s.cfg.PortalDir = dir
	h := s.portalHandler()

	t.Run("a real asset is served", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/app/assets/app.js", nil))
		if w.Code != 200 {
			t.Fatalf("status %d, want 200", w.Code)
		}
		if w.Body.String() != "console.log(1)" {
			t.Errorf("body = %q", w.Body.String())
		}
	})

	t.Run("a client-side route falls back to the SPA shell", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/app/dashboard", nil))
		if w.Code != 200 {
			t.Fatalf("status %d, want 200", w.Code)
		}
		if w.Body.String() != "<html>shell</html>" {
			t.Errorf("body = %q, want the SPA shell", w.Body.String())
		}
	})

	t.Run("a missing asset 404s instead of getting the shell", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/app/assets/does-not-exist.js", nil))
		if w.Code != 404 {
			t.Errorf("status %d, want 404", w.Code)
		}
	})

	t.Run("the root path serves the shell", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/app/", nil))
		if w.Code != 200 || w.Body.String() != "<html>shell</html>" {
			t.Errorf("status %d, body %q", w.Code, w.Body.String())
		}
	})
}
