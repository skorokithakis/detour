---
id: det-cpuup
status: closed
deps: [det-qidhd]
links: []
created: 2026-09-05T09:01:03Z
type: task
priority: 2
assignee: Stavros Korokithakis
---
# Dockerfile, Dokku compatible

Objective: a multi-stage Dockerfile producing a small image, deployable on Dokku with no extra configuration.

Scope:
- Build stage on the official Go image matching go.mod. Build a static binary with CGO_ENABLED=0.
- Runtime stage FROM gcr.io/distroless/static. It ships the CA bundle, which the https health checks need.
- Do NOT add an EXPOSE directive. This is deliberate and must carry a short comment in the Dockerfile explaining why, because it looks like an omission and someone will try to 'fix' it.
- CMD runs the binary. No config file, no volume, no entrypoint script. All configuration comes from environment variables.

Why no EXPOSE (from the Dokku port management docs): a Dockerfile app WITH EXPOSE is published by Dokku on that same port number, so EXPOSE 8080 serves the app on port 8080 instead of 80. A Dockerfile app WITHOUT EXPOSE gets the default mapping host 80 -> container 5000, and Dokku sets PORT=5000 in the environment. detour reads PORT, so omitting EXPOSE gives the correct result on Dokku and stays fine under plain docker run with an explicit -p mapping.

Non-goals: no docker-compose, no CI workflow, no image publishing, no healthcheck directive.

Ready for implementation.

## Acceptance Criteria

docker build succeeds. Running the image with no BACKENDS set fails immediately with a clear message. Running it with BACKENDS and PORT set serves redirects on that port.

