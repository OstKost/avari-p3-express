# Карта проекта для агента

Код реализует личное веб-приложение для управления проектами по методологии P3.express v2 со связкой со стадиями SDLC и AI-SDLC.

## Где искать

- Запуск и сборка зависимостей: `apps/api/cmd/server/main.go`.
- Маршруты и healthcheck: `apps/api/internal/handler/router.go`; P3 handler: `p3_handler.go`; HTTP DTO и ошибки: `dto.go` рядом.
- Доменные сущности и контракты: `apps/api/internal/domain/p3.go`, `p3_template.go`, `errors.go`; use cases: `internal/service/p3_service.go`.
- SQL и репозиторий: `internal/repository/sqlite/p3_repository.go`; подключение и embedded Goose: `internal/database/`.
- Корень UI: `apps/web/src/App.tsx`; layout: `src/app/p3/Shell.tsx`; страницы: `src/features/p3/`.
- TypeScript-контракт, запросы и кеш: `src/entities/p3/{types,api,queries}.ts`; Axios: `src/shared/api/client.ts`.
- Стили и компоненты: `src/index.css`, `src/shared/components/`; локальные предпочтения: `src/shared/store/`.
- Спецификации продукта: [_init/INDEX.md](../_init/INDEX.md). Общие визуальные правила — `11-ui-spec.md`.

## Границы, которые важно сохранять

HTTP → service → repository; интерфейсы и ошибки находятся в domain. Транспорт и SQL не должны проникать в бизнес-правила. На UI данные сервера проходят через TanStack Query, локальные предпочтения — через Zustand.

Контракт Go и TypeScript пока синхронизируется вручную. При изменении JSON проверять обе стороны, nullable/optional поля, HTTP status и ошибки.

SQLite работает через `modernc.org/sqlite`; production build без CGO. Пул ограничен одним пишущим соединением с включённым WAL режимом (`PRAGMA journal_mode=WAL`, `foreign_keys=ON`). История изменений фиксируется транзакционно через триггеры `p3_activity`.

## Что ещё не доказано

Локальные Go-тесты, lint и сборка не доказывают production readiness. На момент внедрения harness отсутствует отдельный web test runner; нет доказательств авторизации, нагрузочной устойчивости или полного E2E покрытия. CI и Docker используют разные версии инструментов; отдельная нормализация toolchain полезна перед релизом.

Для долговременного изменения архитектуры использовать [ADR](decisions/TEMPLATE.md); для разработки среза — [задачу](tasks/TEMPLATE.md).


## Производственные задачи и локальный доступ

`internal/domain/work.go` описывает WorkTask/AgentRun/TaskResult и repository контракт; `internal/service/work_service.go` владеет переходами, зависимостями, снимками и приёмкой. `repository/sqlite/work_repository.go` сохраняет агрегаты и аудит одной транзакцией в таблицах миграции 4. Старые Action, циклы, чеклисты и прогресс не изменяются автоматически.

HTTP `work_handler.go` и stdio `internal/mcpbridge` используют один сервис через HTTP (MCP SQLite не открывает). ManagerSession UI требует отдельный persistent manager.key; агентские хешированные токены имеют проектные scopes и не получают право приёмки. UI сущности и запросы находятся в `entities/work`, взаимодействия — в `features/work`; TanStack Query обновляет видимые списки каждые пять секунд. Контракт и ограничения: [work-api.md](work-api.md), решение: [ADR 0002](decisions/0002-agent-work.md).
