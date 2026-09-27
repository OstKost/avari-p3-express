# Independent QA

Select the submitted run, snapshot criteria, digest and exact artifact revision. Reproduce the criterion through a permitted test or inspect the relevant artifact. Record input, expected observation, actual observation, command/method, revision and evidence path/URL. Another author's log is reviewed evidence, not a test you executed. Missing tool/access/revision is unknown.

For Avari changes use isolated SQLite, service/HTTP tests, real stdio clients and relevant Playwright flows. Cover error/empty/loading/success, focus/keyboard, narrow width and both themes when UI changes. Do not run destructive verification on the user's real database. Write only ignored evidence and isolated fixtures, never fix source as the reviewer.

A QA report includes all criteria with pass/fail/unknown; partial security review can be separate. Describe concrete defects, not stylistic preferences. Manager acceptance and executive claims remain separately identified.
