---
name: avari-debug
description: Reproduce and diagnose Avari API, SQLite or UI failures; implement a regression fix when the user requested repair.
---

# Reproduce before fixing

Read the relevant AGENTS.md and inspect current working changes. Capture input, expected behavior, actual behavior and the smallest reproducible path. Distinguish a pre-existing failure from this change; a missing Git HEAD is not a clean baseline.

Trace the failing boundary: UI state/query key → transport/DTO → handler/service → repository. Read the failing log excerpt in `.harness/runs/` before expanding to full logs. Never print `.env` contents or credentials for diagnosis.

Test one evidence-based hypothesis at a time. For intermittent behavior, document reproduction frequency and inspect concurrency, timing and cleanup; avoid fixed sleeps that merely hide races.

If the request is diagnosis only, return the cause and evidence. If repair is requested, add a meaningful regression test where practical, demonstrate the failure, implement the fix and run the focused test plus the affected `make check-*` gate.

After repeated failure of the same approach, change the hypothesis or identify the missing dependency; do not loop identical commands. Return root cause, fix if authorized, commands/results and any remaining reproduction gap.

For work/MCP failures inspect actor role/scope, aggregate version, run state, submission digest and idempotency key before retry. Never repair a 403 by changing credentials or scope.
