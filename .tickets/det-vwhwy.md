---
id: det-vwhwy
status: closed
deps: []
links: []
created: 2026-09-05T09:00:41Z
type: task
priority: 2
assignee: Stavros Korokithakis
---
# Repo skeleton: go.mod, AGPL license, gitignore

Objective: base files for the detour repository.

Scope:
- go.mod, module path github.com/skorokithakis/detour, Go 1.24. No dependencies.
- LICENSE: the full, verbatim GNU Affero General Public License v3.0 text. Fetch the canonical text; do not paraphrase or truncate.
- .gitignore: the compiled binary and usual Go noise.

Non-goals: no main.go, no Dockerfile, no README, no CI, no Makefile, no config file (detour is configured by environment variables).

Ready for implementation.

## Acceptance Criteria

go.mod is valid and dependency-free. LICENSE is the complete AGPL-3.0 text.

