# caddy-casefold

Case-insensitive (Unicode-aware) path matching for Caddy by normalizing the incoming request path **before** the rest of the HTTP routing graph runs.

This lightweight middleware rewrites `r.URL.Path` using either simple lowercase or full Unicode case folding, so that all downstream path, file server, and route matchers behave case-insensitively (unless explicitly excluded).

## Why?

Out of the box Caddy performs case-sensitive path matching. When migrating from ecosystems where URLs were treated case-insensitively (some legacy IIS / Windows deployments, user generated links, etc.) you might want a drop-in way to accept any case variant without rewriting every matcher in your Caddyfile.

## Features

* Global case-insensitive behavior via one directive
* Three modes: `lower` (default), Unicode `fold`, or filesystem canonical `fs`
* Optional exclusion globs for paths that must remain case-sensitive
* Optional `verbose` flag for detailed debug logging of rewrites/skips
* Adds `X-Original-URI` header preserving the pre-transform path

## Installation

Use `xcaddy` (recommended):

```powershell
xcaddy build --with github.com/s-nix/caddy-casefold@latest
```

Or add to an existing `xcaddy build` command.

## Caddyfile Usage

```caddyfile
{
		order casefold first
}

example.com {
		casefold {
				# mode fold | lower | fs (default lower)
				mode fold
				# root only needed for fs mode (filesystem canonical casing)
				# root /var/www/site
				# one or more exclude patterns (path.Match globs)
				exclude /api/CaseSensitive/*
				exclude /media/*.ZIP
				# enable debug logging for this middleware instance
				verbose
		}

		handle /Hello {
				respond "Hi" 200
		}

		# Will match /hello, /HeLLo, /HELLO, etc.
}
```

### JSON Config

```jsonc
{
	"apps": {
		"http": {
			"servers": {
				"srv0": {
					"routes": [
						{
							"handle": [
								{"handler": "casefold", "mode": "fold", "exclude": ["/api/*"]},
								{"handler": "static_response", "body": "OK"}
							]
						}
					]
				}
			}
		}
	}
}
```

## Notes & Caveats

* Apply early: be sure to declare the `order casefold first` block so the path is transformed before other matchers evaluate.
* Exclusions use Go's `path.Match` (wildcards `*`, `?`, character classes). They are evaluated against the full path (leading slash included).
* `fold` mode uses Unicode case folding (ß → ss, Greek sigma handling, etc.). This may slightly increase allocations vs simple lowercase.
* `fs` mode walks the filesystem for each incoming path to map segments to their actual on-disk casing (use sparingly; involves directory reads per request; consider caching behind a CDN). Requires `root`.
* `verbose` adds debug-level logs (set global logging level to `debug` to see them) showing skips, transformations, and canonicalization results.
* Only the path component is transformed; query string is untouched.
* If downstream logic depends on the original casing, read the `X-Original-URI` header.

## Testing

```powershell
go test ./...
```

## License

MIT
