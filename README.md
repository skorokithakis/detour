# detour

A tiny HTTP load balancer that answers with a `302` redirect instead of proxying.

`detour` keeps a list of backends in priority order and checks their health in the
background. Every request gets a `302 Found` pointing at the first healthy backend,
with the original path and query string unchanged. The client then talks to that
backend directly, so `detour` never carries any traffic itself.

Because it only ever redirects, `detour` cannot see when a real request fails. The
periodic health check is the only source of liveness.

## Configuration

All configuration is by environment variable.

| Variable | Required | Default | Meaning |
| --- | --- | --- | --- |
| `BACKENDS` | yes | | Comma-separated base URLs, in priority order. |
| `CHECK_INTERVAL` | no | `60s` | How often to check every backend. |
| `PORT` | no | `8080` | Port to listen on. |

A backend counts as healthy when `GET <backend>/` answers `200` within 10 seconds.
All backends are checked in parallel, once at startup and then on every interval.
If no backend is healthy, `detour` answers `503`.

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
