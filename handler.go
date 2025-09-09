package casefold

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
	"golang.org/x/text/cases"
)

// Casefold is an HTTP middleware that rewrites the request URL path using
// a chosen case folding strategy before other matchers/handlers run. This
// makes path matchers defined in the Caddyfile effectively case-insensitive
// (unless excluded.)
//
// Example Caddyfile usage:
//
//	{
//	    order casefold first
//	}
//	:8080 {
//	    casefold {
//	        mode fold   # or "lower" (default)
//	        exclude /api/CaseSensitive/*
//	        exclude /downloads/*.ZIP
//	    }
//	    handle /Hello {
//	        respond "Hi" 200
//	    }
//	}
//
// A request for /hello or /HeLLo will match /Hello.
type Casefold struct {
	// Mode selects the transformation applied to the path. Supported values:
	//  - "lower" (default): simple ASCII + Unicode ToLower
	//  - "fold": Unicode case folding (locale-independent)
	//  - "fs": canonicalize each existing path segment to the actual filesystem casing
	Mode string `json:"mode,omitempty"`

	// Root is required for mode "fs" and denotes the filesystem root directory
	// that request paths are resolved against for canonical casing. If empty
	// when mode=fs, the middleware skips canonicalization.
	Root string `json:"root,omitempty"`

	// Exclude is an optional list of glob patterns (evaluated with path.Match)
	// that, if any matches the original request path, will skip rewriting.
	// Patterns are matched against the leading slash form of the path.
	Exclude []string `json:"exclude,omitempty"`

	fold caser `json:"-"`
}

// caser abstracts the Fold or Lower implementation we pick at provision time.
type caser interface{ String(string) string }

// CaddyModule returns the Caddy module information.
func (Casefold) CaddyModule() caddy.ModuleInfo { //nolint:revive
	return caddy.ModuleInfo{
		ID:  "http.handlers.casefold",
		New: func() caddy.Module { return new(Casefold) },
	}
}

// Provision sets up the module.
func (c *Casefold) Provision(ctx caddy.Context) error { //nolint:revive
	switch strings.ToLower(strings.TrimSpace(c.Mode)) {
	case "", "lower":
		c.fold = lowerCaser{}
	case "fold":
		c.fold = cases.Fold()
	case "fs":
		// handled dynamically in ServeHTTP; keep fold nil
		if c.Root == "" {
			ctx.Logger().Warn("fs mode enabled but root not set; skipping canonicalization")
		} else {
			// normalize root to absolute for safety
			if !filepath.IsAbs(c.Root) {
				abs, err := filepath.Abs(c.Root)
				if err == nil {
					c.Root = abs
				}
			}
		}
	default:
		ctx.Logger().Warn("unknown casefold mode; defaulting to lower", zap.String("mode", c.Mode))
		c.fold = lowerCaser{}
	}
	return nil
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (c *Casefold) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error { //nolint:revive
	orig := r.URL.Path
	if orig == "" || orig == "/" {
		return next.ServeHTTP(w, r)
	}
	if c.skip(orig) {
		return next.ServeHTTP(w, r)
	}

	mode := strings.ToLower(strings.TrimSpace(c.Mode))
	transformed := orig
	switch mode {
	case "", "lower", "fold":
		transformed = c.fold.String(orig)
	case "fs":
		canon, ok := c.canonicalFS(orig)
		if ok {
			transformed = canon
		} else {
			// fallback to original (no change) if not all segments resolved
		}
	}

	if transformed != orig {
		r.Header.Set("X-Original-URI", orig)
		w.Header().Set("X-Original-URI", orig)
		r.URL.Path = transformed
		r.RequestURI = transformed
	}
	return next.ServeHTTP(w, r)
}

// canonicalFS attempts to replace each path segment with the actual casing
// found on disk under Root. Returns (newPath, true) on success. If Root is empty,
// a segment is missing, or a security check fails, returns original path, false.
func (c *Casefold) canonicalFS(p string) (string, bool) {
	if c.Root == "" {
		return p, false
	}
	clean := path.Clean(p)
	if !strings.HasPrefix(clean, "/") {
		return p, false
	}
	if clean == "/" {
		return p, false
	}
	segs := strings.Split(strings.TrimPrefix(clean, "/"), "/")
	curDir := c.Root
	// prevent traversal outside root: reject any segment with '..'
	for _, s := range segs {
		if s == ".." {
			return p, false
		}
	}
	built := make([]string, 0, len(segs))
	for i, seg := range segs {
		entries, err := os.ReadDir(curDir)
		if err != nil {
			return p, false
		}
		var matchName string
		// first attempt exact match
		for _, e := range entries {
			name := e.Name()
			if name == seg {
				matchName = name
				break
			}
		}
		if matchName == "" {
			// case-insensitive search
			lowered := strings.ToLower(seg)
			for _, e := range entries {
				if strings.ToLower(e.Name()) == lowered {
					matchName = e.Name()
					break
				}
			}
		}
		if matchName == "" {
			return p, false
		}
		built = append(built, matchName)
		if i < len(segs)-1 { // descend only if not final segment
			curDir = filepath.Join(curDir, matchName)
			// optional: if it's not a dir we can stop early
			fi, err := os.Stat(curDir)
			if err != nil || !fi.IsDir() {
				if i != len(segs)-1 {
					return p, false
				}
			}
		}
	}
	return "/" + strings.Join(built, "/"), true
}

// skip returns true if the path matches an exclude pattern.
func (c *Casefold) skip(p string) bool {
	for _, gl := range c.Exclude {
		if gl == "" {
			continue
		}
		if ok, _ := path.Match(gl, p); ok {
			return true
		}
	}
	return false
}

// lowerCaser provides a simple Unicode lower mapping using strings.ToLower.
type lowerCaser struct{}

func (lowerCaser) String(s string) string { return strings.ToLower(s) }

// Interface guards
var _ caddy.Module = (*Casefold)(nil)
var _ caddyhttp.MiddlewareHandler = (*Casefold)(nil)

func init() { caddy.RegisterModule(Casefold{}) }
