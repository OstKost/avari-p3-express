---
name: avari-feature
description: Implement an Avari feature across Go API and React UI, keeping contracts, product specs and user-visible verification aligned.
---

# Deliver a vertical feature

Read root and affected scoped AGENTS.md. Inspect the nearest implementation and tests. Use an existing task file if present; create a short plan only for work that crosses layers or sessions.

Write concrete success and failure examples. For P3 features identify the source spec and distinguish mock UI from persisted functionality. Define JSON fields, optionality, status/errors and query invalidation before splitting API and web work.

Implement the narrowest complete slice. Keep dependencies and data migrations proportional to the feature. Add regression tests at the layer owning changed behavior; persistence needs temporary SQLite verification. Preserve existing user changes.

During iteration run focused checks. At integration run `make check-api`, `make check-web` or `make check` for the affected scope. UI changes also need a browser interaction check when tools are available; a green bundle does not establish correctness.

For substantial work delegate an independent bounded review to `avari-reviewer` if available, passing acceptance criteria and changed paths, not an expected verdict. Resolve confirmed in-scope findings. Return behavior, evidence and limitations; update the active task's next step or mark its acceptance criteria complete.

Attach versioned outputs and actual test evidence. For a task attempt use its snapshot; hand the submitted revision and digest to independent QA without supplying an expected verdict.
