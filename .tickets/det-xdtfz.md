---
id: det-xdtfz
status: closed
deps: [det-cpuup]
links: []
created: 2026-09-05T09:01:07Z
type: task
priority: 2
assignee: Stavros Korokithakis
---
# Create public GitHub repo and push

Objective: publish the repository.

Scope:
- Commit everything with a sensible initial commit message.
- Create github.com/skorokithakis/detour as a PUBLIC repository using gh, with a one-line description.
- Push master.

Caveat: do not create the repo until the README exists; the architect writes it separately.

Non-goals: no releases, no tags, no branch protection, no CI, no topics.

Ready for implementation.

## Acceptance Criteria

The public repo exists and master is pushed with the full tree.

