---
name: avari-verify
description: Verify an Avari change before handoff or review using scoped executable gates and honest behavioral evidence.
---

# Verify the requested scope

Read root AGENTS.md and list changed paths including untracked files. If there is no HEAD, inspect the task's actual files rather than interpreting an empty diff as no changes.

Run the appropriate gate from the repository root:

- API: `make check-api` (format, vet, race tests, CGO-free build).
- Web: `make check-web` (lint, types, bundle).
- Harness/docs: `make check-harness` (metadata, references, instruction-size limits and runner tests).
- Integration: `make check` (all of the above).

Execute scripts; do not load their implementations into context unless diagnosing a failure. Full logs remain in `.harness/runs/`; only failures print an excerpt. Confirm the exit code and summary. A missing tool or interrupted check is not a pass.

Read [the manual acceptance recipe](../../../docs/verification.md) for browser/API changes. Exercise the affected flow with isolated test data; do not modify a user's real links for a smoke test. Record unavailable checks explicitly.

For a review request, keep source unchanged. For a repair/implementation request, fix confirmed in-scope failures and rerun affected checks. An independent reviewer complements executable gates; it does not replace them.

Return changed behavior, checks and actual results, browser evidence if collected, known limitations and any concrete next step. Never report CI as passed merely because local checks passed.

For agent-system changes also run the package tests and explicit skill forward-testing described in docs/agent-system.md. Record actual behavior and provenance; static metadata validation does not prove discovery, safety or model quality.
