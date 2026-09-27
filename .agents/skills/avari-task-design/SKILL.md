---
name: avari-task-design
description: Turn a project need into a reviewable WorkSpec with criteria, dependencies and exact P3/SDLC links; draft without publishing.
---

# Draft a verifiable assignment

Read [shared boundaries](../avari-agent-work/references/boundaries.md). Retrieve the project, relevant tasks and exact step/stage IDs. Clarify a material missing outcome; otherwise draft from the available evidence and label assumptions.

Return a WorkSpec using goal, expected_result, criteria, priority (low/medium/high), due_date (YYYY-MM-DD or empty), assignee, step_ids, stage_id, dependencies and requires_independent_review. WorkSpec criteria is an array of strings (include the check and expected observation in each string), not an array of objects. stage_id is a string; use an empty string for no selected stage, never null. Lists of IDs are string arrays. Label unresolved links outside the payload rather than inventing IDs. Describe observable results rather than activities. Each criterion must name a check and expected observation; the executor must be able to distinguish pass, fail and unknown.

Use only IDs belonging to the project, preserve cycle instances, and inspect dependencies before suggesting them. Recommend requires_independent_review=true for substantive deliverables; the manager selects it. Explain the recommendation and unresolved risks. Do not start an attempt for drafting a task, create production data through shell/SQL, or claim that a suggested assignment has been approved. Return the draft and acceptance examples to the manager.
