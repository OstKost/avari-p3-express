---
name: avari-release
description: Prepare release readiness, migration and rollback evidence for a versioned software result; deployment requires its own authorization.
---

# Prepare a release decision

Read [shared boundaries](../avari-agent-work/references/boundaries.md) and the project's actual release instructions. Identify the exact release/commit, accepted results, independent findings, unresolved blockers and required compatibility evidence.

Check packaging, configuration, schema changes, restore/rollback procedure and operator instructions against the target environment. For Avari use make check, isolated migration tests and the targeted Playwright/MCP flows; a local build does not prove Docker/CI or deployment. Run environment checks only when authorized and available; otherwise name the gap.

Return a version-specific readiness note: included results, commands and observed results, migration/rollback plan, outstanding conditions and the manager decision needed. Produce release notes from verified changes. Do not merge, publish, deploy, send messages or change SDLC/P3 statuses merely because readiness checks pass.
