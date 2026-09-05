# detour

A tiny HTTP load balancer that answers with a `302` redirect instead of proxying.

`detour` keeps a list of backends in priority order and checks their health in the
background. While at least one backend is healthy, every request gets a `302 Found`
pointing at the first healthy one, with the original path and query string
unchanged. The client then talks to that backend directly, so `detour` never
carries any traffic itself. When no backend is healthy, it serves a `503` page
instead.

Because it only ever redirects, `detour` cannot see when a real request fails. The
periodic health check is the only source of liveness.

## Configuration

All configuration is by environment variable.

| Variable | Required | Default | Meaning |
| --- | --- | --- | --- |
| `BACKENDS` | yes | | Comma-separated base URLs, in priority order. |
| `CHECK_INTERVAL` | no | `60s` | How often to check every backend. |
| `FALLBACK_URL` | no | `https://github.com/skorokithakis/detour` | Where visitors are sent when no backend is healthy. Must be an absolute HTTP(S) URL, and may include a query string or fragment. |
| `HEALTHY_THRESHOLD` | no | `3` | Consecutive successful checks before a backend counts as stable enough to claim the active slot. Minimum `1`. |
| `PORT` | no | `8080` | Port to listen on. |

A backend counts as healthy when `GET <backend>/` answers `200` within 10 seconds.
All backends are checked in parallel, once at startup and then on every interval.
If no backend is healthy, `detour` answers `503` with a small HTML page. The page
says there are no healthy backends, redirects the visitor to `FALLBACK_URL` after
5 seconds via a meta refresh, and includes a plain link for anyone who is not
redirected automatically. It uses inline CSS only, with no scripts or external
assets, since the network those would load from may itself be part of the outage.
The status stays `503` rather than `200` so uptime monitoring still sees the
outage.

Each backend carries a streak of consecutive successes: every successful check
increments the streak and a single failure resets it to zero. The active slot goes
to the first backend in priority order whose streak has reached
`HEALTHY_THRESHOLD`. This damping stops a flapping backend from repeatedly taking
over the active slot: one failed check disqualifies it until it has stayed up for
the whole threshold again — unless no backend has reached the threshold, in which
case the fallback below can still select it.

If no backend has reached the threshold, the first backend with at least one
success wins anyway. A marginal backend beats a `503`: the first check runs
immediately at startup, so a cold start selects a backend as soon as that first
check completes, and a recovery from total outage waits at most one
`CHECK_INTERVAL` for the next check — instead of `HEALTHY_THRESHOLD` x
`CHECK_INTERVAL`. The tradeoff is a warmup window: for the first
`HEALTHY_THRESHOLD - 1` intervals after startup no streak can have reached the
threshold, so a backend that flaps from boot can still be selected. Damping
engages as soon as at least one backend has reached the threshold; if no backend
ever reaches it, the fallback rule governs indefinitely.

The "backend X is up/down" log lines follow the raw probe result rather than the
damped streak, so flapping stays visible in the logs. With the default settings a
backend that alternates up and down on every check logs that alternation while
never reaching the threshold.

Lower `CHECK_INTERVAL` if you want failures noticed sooner. A backend that dies just
after a check keeps receiving users until the next one.

## Run it

```sh
docker build -t detour .
docker run -p 8080:8080 \
  -e BACKENDS="https://first.example.com,https://second.example.com" \
  detour
```

## Deploy on Dokku

```sh
dokku apps:create detour
dokku config:set detour BACKENDS="https://first.example.com,https://second.example.com"
git remote add dokku dokku@your-host:detour
git push dokku master
```

The Dockerfile has no `EXPOSE` directive on purpose, so Dokku serves the app on port
80 and sets `PORT` itself. See the comment in the `Dockerfile` before you change it.

## License

AGPL-3.0. See [LICENSE](LICENSE).
