---
id: det-qidhd
status: closed
deps: [det-vwhwy]
links: []
created: 2026-09-05T09:00:53Z
type: task
priority: 2
assignee: Stavros Korokithakis
---
# main.go: env config, health checker, 302 handler

Objective: implement all of detour in a single main.go, around 150 lines, standard library only.

Configuration, from environment variables:
- BACKENDS (required): comma-separated base URLs in priority order, e.g. "https://a.example.com,https://b.example.com". Empty or missing is a fatal startup error.
- CHECK_INTERVAL (optional, default "60s"): Go duration string.
- PORT (optional, default "8080"): the TCP port to listen on. Dokku sets this itself.

Behaviour:
- Parse and validate every backend URL at startup. An invalid URL or an unparseable duration is a fatal startup error.
- Health checker goroutine: runs once immediately at startup, then on every CHECK_INTERVAL tick. Checks all backends CONCURRENTLY, one goroutine each, and waits for all of them. Live means GET <backend>/ returns status exactly 200 within a 10 second timeout. The health check client must NOT follow redirects.
- Strict failover: the active backend is the FIRST live backend in configured order.
- Handler: answer 302 Found. Location is the active backend base URL joined with the incoming request path and query string, unchanged. Set Cache-Control: no-store. If no backend is live, answer 503.

Design constraints:
- Shared state is a single atomic pointer holding the active backend. The checker writes it; the handler only reads it. No mutex in the request path.
- Zero external dependencies.
- Set timeouts on the http.Server, at minimum ReadHeaderTimeout.

Logging: log once at startup (listen port, backend count, interval). Otherwise log ONLY backend state transitions (up->down, down->up) and changes of the active backend. Do not log individual requests.

Caveats:
- Joining the base URL with the request URI must not create a double slash, must not drop the query string, and must work when the backend URL has a trailing slash or a path prefix.
- Health checks go to https backends, so they use TLS.

Non-goals: no reverse proxying, no request bodies, no retries, no metrics, no admin endpoint, no config reload, no TLS termination, no per-backend health path, no weights, no config file.

Ready for implementation.

## Design

302 rather than a reverse proxy is deliberate: the client contacts the backend directly. Strict failover rather than round-robin, so only one backend URL is exposed publicly at a time. A 302 gives no signal when a real request fails, so the periodic check is the only source of liveness.

## Acceptance Criteria

Failover selects the first live backend in configured order. Path and query survive the redirect exactly. No live backend gives 503. Redirects carry Cache-Control: no-store.

