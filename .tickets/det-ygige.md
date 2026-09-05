---
id: det-ygige
status: closed
deps: []
links: []
created: 2026-09-05T10:54:46Z
type: feature
priority: 2
assignee: Stavros Korokithakis
---
# Add health check hysteresis to damp backend flapping

Ready for implementation.

Objective: stop a flapping backend from repeatedly taking over the active slot in main.go.

Replace the `live []bool` / `known []bool` pair on healthChecker with a single `successCount []int`. On each check, a success increments that backend's counter and a failure resets it to 0.

Selection, evaluated in backend priority order:
1. First backend with successCount >= threshold wins.
2. If none qualify, first backend with successCount >= 1 wins.
3. If none, nil (existing 503 path).

Rule 2 is deliberate. Hysteresis exists to avoid unnecessary switching; with no healthy backend to stay on, a marginal backend beats a 503. Without rule 2, cold start and total-outage recovery would each cost threshold x CHECK_INTERVAL of downtime.

New env var HEALTHY_THRESHOLD, default 3, minimum 1, parsed and validated in loadConfig alongside CHECK_INTERVAL. A value of 1 must reproduce current behaviour exactly.

Keep the existing per-backend "backend X is up/down" log driven by the RAW probe result, not the damped state, so flapping stays visible to operators. The "active backend changed to X" log stays as is.

Extract the selection rule into a pure function (counts + threshold -> chosen index or -1) and cover it with one table-driven test in a new main_test.go: flapping backend never preempts a stable one, a stable backend regains active after 3 checks, cold start picks on first success, total outage recovers on first success, threshold of 1 matches old behaviour. The repo has no tests today, so this is the first test file.

Update README.md: document HEALTHY_THRESHOLD in the config table, and explain the damping and the warmup behaviour in prose.

Scope: main.go, new main_test.go, README.md.

Non-goals: penalty scores, exponential suppression, decay, or any per-backend history beyond the current streak. No wall-clock timers. No configurable down-threshold, one failed check always disqualifies. No changes to check scheduling, timeouts, or the 503 response.

## Design

Counting checks rather than wall-clock time was chosen because the recovery delay is then simply threshold x CHECK_INTERVAL and needs no extra state.

Known and accepted: for the first two intervals after startup no backend can have a streak of 3, so rule 2 governs and a backend that flaps from boot can still be selected during that window. Damping engages from the third check. This was weighed against the alternative (apply the threshold uniformly, hence a guaranteed 3 x CHECK_INTERVAL of 503 on every deploy) and judged the cheaper trade.

