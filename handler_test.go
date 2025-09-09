package casefold

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/caddyserver/caddy/v2"
)

type recordHandler struct{ t *testing.T }

func (h recordHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("X-Final-Path", r.URL.Path)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(r.URL.Path))
	return nil
}

func TestCasefoldLower(t *testing.T) {
	c := &Casefold{Mode: "lower"}
	if err := c.Provision(caddy.Context{}); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.test/HeLLo/World", nil)
	if err := c.ServeHTTP(rr, req, recordHandler{t}); err != nil {
		t.Fatal(err)
	}
	if got := rr.Header().Get("X-Final-Path"); got != "/hello/world" {
		t.Fatalf("expected transformed path /hello/world, got %s", got)
	}
	if orig := rr.Result().Header.Get("X-Original-URI"); orig == "" {
		t.Fatalf("expected X-Original-URI header to be set")
	}
}

func TestCasefoldExclude(t *testing.T) {
	c := &Casefold{Mode: "lower", Exclude: []string{"/API/*"}}
	if err := c.Provision(caddy.Context{}); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.test/API/MixedCase", nil)
	if err := c.ServeHTTP(rr, req, recordHandler{t}); err != nil {
		t.Fatal(err)
	}
	if got := rr.Header().Get("X-Final-Path"); got != "/API/MixedCase" {
		t.Fatalf("expected original path preserved, got %s", got)
	}
}

func TestCasefoldFoldMode(t *testing.T) {
	c := &Casefold{Mode: "fold"}
	if err := c.Provision(caddy.Context{}); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.test/straße", nil)
	if err := c.ServeHTTP(rr, req, recordHandler{t}); err != nil {
		t.Fatal(err)
	}
	if got := rr.Header().Get("X-Final-Path"); got != "/strasse" {
		t.Fatalf("expected folded path /strasse, got %s", got)
	}
}

func TestCasefoldFSMode(t *testing.T) {
	root := t.TempDir()
	// create nested structure: scripts/MyScript.bat
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(root, "scripts", "MyScript.bat")
	if err := os.WriteFile(filePath, []byte("echo test"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := &Casefold{Mode: "fs", Root: root}
	if err := c.Provision(caddy.Context{}); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.test/scripts/myscript.bat", nil)
	if err := c.ServeHTTP(rr, req, recordHandler{t}); err != nil {
		t.Fatal(err)
	}
	if got := rr.Header().Get("X-Final-Path"); got != "/scripts/MyScript.bat" {
		t.Fatalf("expected canonical FS path /scripts/MyScript.bat, got %s", got)
	}
}
