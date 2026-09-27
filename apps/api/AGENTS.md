# API scope

- Go module root is `apps/api`; run Go commands there. `make check-api` runs from the repository root.
- `internal/domain` owns entities/interfaces/errors; `service` owns use cases; `repository/sqlite` owns SQL; `handler` owns HTTP/DTO mapping. Wire dependencies in `cmd/server/main.go`.
- Preserve error wrapping and `errors.Is` mapping. Propagate request contexts to synchronous I/O.
- Parameterize SQL. Keep the pure-Go SQLite build (`CGO_ENABLED=0`); race tests use normal host CGO settings. Verify SQLite pragmas through queries when changing connection setup; comments and DSN spelling are not evidence that pragmas took effect.
- Migrations are embedded Goose SQL in `internal/database/migrations`. Add the next migration; do not rewrite a migration already applied to a persistent database. Test with an isolated temporary database, never the user's local data.
- An API change includes relevant handler tests, `apps/web/src/entities/*` consumers, error/status semantics and Swagger source annotations. Existing `apps/api/docs/docs.go` is a minimal placeholder; do not assume complete generated coverage. Use `make swagger` only when the generator is available and inspect its changes.
- For behavior changes, prefer table-driven service tests and `httptest`; persistence changes need real temporary SQLite tests. A regression test should fail before the fix when practical.
- `make check-api` checks formatting without rewriting files, runs `go vet`, race tests and a CGO-free build. Use a focused `go test ./internal/service -run TestName` during iteration.
