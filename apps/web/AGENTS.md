# Web scope

- Run pnpm commands in `apps/web`; root Make targets handle this automatically. Keep the existing pnpm lockfile; do not introduce a second package manager or discard frozen-lockfile failures.
- `src/app` is application layout, `features` owns user interactions, `entities` owns domain types/API/query hooks, and `shared` contains reusable UI and utilities. Follow existing paths before adding abstractions.
- Server state belongs to TanStack Query; Zustand holds local preferences. Keep query keys and invalidation consistent with new mutations. Forms use React Hook Form + Zod; transport uses `shared/api/client.ts`.
- Match Go JSON names, optional fields, pagination and HTTP error behavior. Client validation supplements server validation. `VITE_*` values ship to browsers and must not contain secrets.
- Reuse shared components and Tailwind styles. Verify loading, empty, success and failure states; keyboard/focus behavior, labels, narrow screens and both existing themes when affected.
- `make check-web` runs lint and TypeScript/Vite build. Playwright E2E is configured separately through `pnpm test:e2e`; it is not part of check-web. Do not report a build as UI tests. Add targeted behavior tests when the feature justifies a runner; a text/style-only change does not require a new test stack.
- For UI work, inspect the running flow with available browser tooling. Save useful evidence under ignored `.harness/` and summarize exact interactions checked. If the browser is unavailable, state the remaining manual check.
