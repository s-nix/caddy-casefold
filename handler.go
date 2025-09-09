package casefold

import (
	"net/http"
	"path"
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
	Mode string `json:"mode,omitempty"`

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
	transformed := c.fold.String(orig)
	if transformed != orig {
		// Expose original path to downstream handlers (request) and client (response)
		r.Header.Set("X-Original-URI", orig)
		w.Header().Set("X-Original-URI", orig)
		r.URL.Path = transformed
		r.RequestURI = transformed
	}
	return next.ServeHTTP(w, r)
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
