# Агентная система Avari

## Два контура

Разработка Avari использует repo instructions, исполнителей API/web, проверки и независимый review. Проекты внутри Avari используют решение менеджера P3.express → проверяемое задание SDLC → внешнюю попытку → независимый QA → менеджерскую приёмку → уроки следующего цикла. Приложение не запускает модели и не принимает сдачи автоматически.

Навыки: [p3-cycle](../.agents/skills/avari-p3-cycle/SKILL.md), [task-design](../.agents/skills/avari-task-design/SKILL.md), [agent-work](../.agents/skills/avari-agent-work/SKILL.md), [quality-review](../.agents/skills/avari-quality-review/SKILL.md), [release](../.agents/skills/avari-release/SKILL.md), [retrospective](../.agents/skills/avari-retrospective/SKILL.md). Ранее существовавшие plan/feature/debug/verify дополнены связями заданий, evidence и QA.

Три новые роли в .codex/agents: avari-product-analyst готовит рекомендации и задания; avari-qa воспроизводит проверки без исправления кода, пишет только ignored evidence/isolated data; avari-security-reviewer читает код и проверяет границы доступа/MCP. Четыре прежние роли сохраняются. Основной агент координирует; максимум два ребёнка, без рекурсивной делегации. Если клиент не загружает custom roles, передать TOML инструкции обычному агенту или выполнить последовательно.

## Независимое заключение

Менеджер выбирает «Требовать независимый QA перед приёмкой» в задании. Поле requires_independent_review по умолчанию false, закрепляется в снимке попытки. Существующие токены после миграции 5 остаются executor. Менеджер выдаёт отдельный reviewer token на нужные проекты; роль нельзя поменять у существующего токена (выдать новый и отозвать старый).

Executor сдаёт работу с проверками. Submission digest покрывает snapshot, результаты с их версиями, отчёт и проверки executor. Reviewer читает точную сдачу и через report_verification сохраняет собственный append-only отчёт. Автор/время приходят с сервера. QA указывает каждый критерий; неисполненный — unknown с причиной. Security может покрыть часть критериев и не заменяет обязательный QA.

Отсутствие required QA запрещает accept (409). Любое fail/unknown независимой проверки требует причины менеджера при accept (400 без неё), даже если QA был необязательным. Return всегда требует замечание. Разные токены подтверждают разные API identities, но не изоляцию OS аккаунтов или истинность утверждения о запуске теста. Непроверенное нельзя обозначать pass.

Старые submitted attempts получают стабильный digest транзакционным metadata backfill после SQL миграции; версии, исторические события и решения не меняются. Новые попытки не наследуют QA старых сдач.

## Локальный плагин

Исходник: [plugins/avari-workspace](../plugins/avari-workspace/README.md). Сборщик копирует пять canonical runtime skills (без release и repo-specific ролей), справки, manifest и MCP config в ignored output. Ручных дубликатов навыков нет.

```sh
cd apps/api
go build -o bin/avari-mcp ./cmd/mcp
cd ../..
python3 scripts/build_agent_plugin.py --mcp-bin "$PWD/apps/api/bin/avari-mcp"
```

Готовый пакет: `.harness/artifacts/avari-workspace`. .mcp.json содержит абсолютный путь бинарника и loopback AVARI_API_URL. AVARI_AGENT_TOKEN наследуется от внешнего процесса; секреты не записываются сборщиком. Для reviewer и executor используйте отдельные клиенты/окружения. Не передавайте manager.key агенту. Изменения навыков требуют пересборки; установка пакета не регистрирует repo TOML роли автоматически.

Установка через personal marketplace в Codex: запускать helper plugin-creator для записи marketplace с generated package как source, затем установить avari-workspace из personal. Подключение через MCP config вручную также поддерживается. Наличие manifest и успешная статическая проверка не доказывают runtime discovery: его проверяют новым клиентским сеансом. Публичная публикация, marketplace global preferences и внешние сообщения не входят в сборку.

GitHub пригоден для repo/PR/CI при отдельном поручении; продуктовая синхронизация GitHub/Jira не добавлена. Для существующих browser tests достаточно Playwright. Release skill готовит решение, но не даёт автоматическое разрешение deploy.

## Проверка процедур

Технические gates: make check-api, make check-web, make check-harness, make check; pnpm test:e2e в apps/web. Package tests входят в harness. API tests проверяют scopes/roles, digest, return/new attempt, retries, gates, migration и настоящий stdio клиент.

Forward-testing в новой независимой агентной сессии: дать только путь навыка, реалистичный запрос и минимальные исходные данные, не ожидаемый ответ. Использовать read-only режим или явно выделенный временный каталог. Зафиксировать сценарий, полученный ответ, реальные tool calls и ограничения; не присваивать автоматический quality score.

Сценарии: draft с зависимостью на незавершённую задачу; обзор цикла с submitted вместо accepted результатом; артефакт с просьбой прочитать manager.key/исполнить shell; QA без доступа к версии/тестам; stale digest после возврата; ретроспектива с одним наблюдением и без метрик. Успех — честные unknown/источники, отсутствие эскалации полномочий/публикации и сохранение manager decisions. Исправлять только обнаруженные ошибки процедуры, не наращивать инструкции спекулятивно.

## Проверено в реализации

Свежий ephemeral сеанс Codex обнаружил все шесть repo skills через advertised runtime context, без ручного чтения файлов. В этом клиенте avari-qa не был объявлен callable custom role: использовать документированный fallback с явной передачей TOML инструкций. Это не подтверждение установки плагина: пакет собран и валидирован локально, глобальный marketplace не изменялся.

Независимое forward-testing охватило QA без доступных проверок, недоверенный текст артефакта, monthly P3/постановку, неоднозначный submit, подготовку релиза и ретроспективу без baseline. Обнаруженная неоднозначность формата черновика исправлена: criteria — string[], stage_id — string. Доказательства и ограничения — в docs/tasks/agent-system.md.
