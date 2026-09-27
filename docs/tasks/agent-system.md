# Задача: агентная система Avari

Статус: done

## Результат и границы
Два контура: разработка Avari и исполнение проектных задач внешними агентами. Шесть новых навыков, три роли, независимый QA с отдельным credential и digest сдачи, локальный плагин. Источник: согласованный план пользователя. Не входит запуск агентов приложением, публичная публикация, синхронизация GitHub/Jira и автоматическая приёмка.

## Приёмка
- [x] Executor сдаёт, отдельный reviewer проверяет, менеджер принимает; данные переживают restart.
- [x] QA обязателен по настройке снимка; отсутствие запрещает accept, fail/unknown требует причину.
- [x] Роли, scopes, повторные вызовы, stale digest и закрытые сдачи защищены сервером.
- [x] Миграция v4→v5 сохраняет старые токены и задачи.
- [x] Навыки и роли проверены; собран локальный пакет без секретов, проведено независимое forward-testing.
- [x] API/web/harness, Playwright и настоящий stdio клиент проходят; ограничения записаны.

## Контракт и порядок работы
WorkSpec.requires_independent_review=false по умолчанию. AgentToken.role=executor|reviewer, default executor. AgentRun.submission_digest и verification_reports. VerificationReport сохраняет автора, run/task, digest, kind=qa|security, summary, checks и idempotency_key. POST /api/v1/agent-runs/{run_id}/verification-reports, MCP report_verification. Reports append-only, только reviewer, не автор попытки. Приёмка только manager. Repo агрегаты и события в одной транзакции. Parent владеет реализацией; дети только read-only review/forward testing.

## Проверки и решения
[ADR 0003](../decisions/0003-independent-verification.md). Логи .harness/runs/.

- make check-api: 4/4 PASS, api-nordbhws; SQLite/race/service/MCP tests.
- make check-web: 2/2 PASS, web-ucp1w4xw.
- make check-harness: 2/2 PASS, harness-tc2dx8r7; package unit tests.
- make check: 8/8 PASS, all-39h82ij7.
- pnpm test:e2e: 1/1 PASS, browser-1790443834303 и qa-browser.log. Настоящие executor/reviewer stdio процессы, обязательный QA, unknown/comment, повтор без дубля, принятие, restart, проекции, обе темы/375px, фокус/Enter, ошибки/пустые состояния. Скриншоты обеих тем просмотрены.
- plugin-validation.log: официальный validator PASS; skill-validation.log: все десять skills valid. Пакет .harness/artifacts/avari-workspace без credential, MCP binary apps/api/bin/avari-mcp.
- runtime-discovery.txt/log: новый ephemeral клиент обнаружил все шесть repo skills; custom avari-qa role не advertised, fallback документирован. Установка плагина в global marketplace не проверялась и не выполнялась.
- Независимый read-only avari-reviewer review_final: существенных findings нет; reviewer не запускал проверки и не менял исходники.
- Независимый forward-testing skills_forward: QA без проверок дал unknown; артефактную эскалацию отверг; P3/SDLC переходы оставил менеджеру; закрытый run не менял; релиз без публикации; AI ускорение без baseline не выдумал. Формат WorkSpec уточнён по наблюдению (criteria строками, stage_id пустая строка). Полный технический сценарий проверен отдельно, это испытание процедур не заменяет runtime/tool tests.

Первый расширенный browser запуск упал из-за неоднозначного test locator (отчёт также виден в истории), исправлен exact locator; повтор PASS. Первоначально официальный validator не запускался без PyYAML; использовано isolated .harness/validator-env, проектные зависимости не менялись.

## Передача контекста
Реализация завершена. Перед публичным выпуском отдельно проверить целевую среду и установить плагин выбранным клиентом; автоматическое обнаружение новых TOML ролей зависит от runtime. Никакой публикации или изменения global preferences не выполнялось.
