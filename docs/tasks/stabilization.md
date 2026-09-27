# Задача: стабилизация демо и проверка системы

Статус: done

## Результат и границы

Реализован запрос пользователя: безопасные демонстрационные данные и воспроизводимая проверка P3, SDLC и производственных задач. Новые команды `make check-smoke`, `make check-full`, `make check-browser`. Рабочая база не используется. Существующие HTTP/JSON контракты и продуктовые миграции не менялись.

## Приёмка

- [x] Поколение содержит SQLite и manifest; один atomic current.json переключает набор. Реальные SIGKILL до/после commit point оставляют старый/новый согласованный набор.
- [x] Seed не дублирует и проверяет версии/связи/SQLite; несовпадение данных требует явного reset. Ошибки до commit сохраняют прежнюю базу.
- [x] API наследует flock. SIGKILL wrapper не разрешает reset/clean живой базы. SIGINT/SIGTERM завершают API; ошибки сборки/bind освобождают lock.
- [x] Время подпроцессов ограничено. Таймаут gate завершает и отдельные sessions потомков; регрессия проверяет listening API, освобождение lock и закрытый порт.
- [x] Общий генератор поддерживает demo/smoke. Manifest v2 содержит profile/date/fixture version, семантические ID и ожидаемые состояния.
- [x] Два reset имеют одинаковое нормализованное содержимое. Проверены 3 проекта/21 задача, связи, исходные состояния и независимые синтетические executor/reviewer/manager.
- [x] Реальный HTTP проверяет manifest, точный checklist→step→phase прогресс и сохранение отдельного project.progress. Приёмка задачи не меняет управленческий прогресс (существующий service/browser workflow).
- [x] SQLite backup/restore сохраняет committed WAL, связи и изменения; восстановленный набор запускается через API. Credentials не входят в backup.
- [x] P3 браузер: вход→проект→A01→чеклист клавиатурой на 375px→прогресс→network error/retry→перезапуск→сохранение.
- [x] P3 браузер: новый цикл B сохраняет прежние шаги и реально связанные производственные задачи; закрытие блокера меняет счётчик и сохраняется после restart.
- [x] Прежний сквозной путь task→return→MCP resubmit→independent QA→acceptance сохранён; узкая ширина и обе темы, empty/error/success, session/restart.
- [x] Два одновременных smoke проходят с независимыми базами/портами.
- [x] Три полных прогона подряд без retries; финальная проверка отсутствия серверов/занятых тестовых портов.
- [x] CI явно устанавливает Go для harness и проверяет POSIX сценарии на Linux/macOS; browser CI использует общий gate. Локальный запуск CI не заменяет удалённый CI.

## Контракт и порядок работы

Родитель владеет генератором, Python-тестами, runner, Makefile, CI, конфигурацией браузера, адаптацией существующего work.spec и документацией. Независимый avari-web владел только новым tests/p3.spec.ts; avari-reviewer выполнил read-only проверку.

CLI: seed/reset/clean/api/paths/backup/restore, --profile demo|smoke, --date YYYY-MM-DD, --dir, --to, --from. paths возвращает db/manifest/manager_key. SQLite поколение содержит demo_fixture с неизменяемой копией manifest. Изменения сервиса не меняют описание исходной фикстуры. Ключ менеджера отдельный, старые поколения сохраняются до clean.

Решение и последствия: [ADR 0004](../decisions/0004-demo-generations.md). Пользовательские инструкции: [демо](../demo.md).

## Проверки и решения

Базовый срез до реализации: make check 8/8, существующий browser 1/1. Реализация добавила meaningful regression тесты, не ослабляя существующие gates.

Адресные проверки: 13 demo-тестов (signals, crash publication, corrupt/symlink/ownership, lock conflicts, WAL backup/restore, HTTP manifest, timeout cleanup), логи `.harness/runs/stabilization/demo-tests-3.log`. В первых расширенных прогонах исправлены ошибки самих новых assertions: task response использует envelope и audit verification_report_id непостоянен.

Независимый reviewer обнаружил оставшийся API при прерывании gate: исправлена уборка дерева потомков, добавлена runtime регрессия; повторный review без concrete findings.

Два одновременных `make check-smoke`: `.harness/runs/smoke-q7wrb4b9/` и `.harness/runs/smoke-mfldrfb3/`, оба 2/2 PASS (13 Python regression tests + 3 browser P3 flows). Нет retries.

Три последовательных `make check-full`: `.harness/runs/full-w93940yu/`, `.harness/runs/full-q73kif8n/`, `.harness/runs/full-mrwt_fii/` — каждый 10/10 PASS (21 Python harness/regression tests, Go format/vet/race/CGO-free server+fixture builds, web lint/type/build, 4 Playwright flows).

Финальная уборка: проверены все записанные browser API/Web порты — listener отсутствует; временные browser каталоги удалены, тестовые server процессы отсутствуют. Сводка: `.harness/runs/stabilization/acceptance.json`. Логи gate содержат versions, fixture date/version, команды, exit code и времена; browser runtime metadata содержит порты, screenshots/HTML report без trace. Временные базы и credentials удаляются.

## Ограничения и передача контекста

Локально проверяется macOS. Linux/macOS CI настроен, но удалённый CI в этой сессии не запускался. Native Windows не поддерживается (fcntl); WSL/Linux поддерживаются по POSIX модели.

Legacy `.demo/demo.db` сохранена; создано новое активное поколение `.demo/generations/805f14f11ad04f7b91f60062da14acae`. Сам reset не удаляет старую базу. Удаление всех поколений — explicit clean. Реальные тесты используют только временные каталоги.

Все критерии выполнены. Следующий пользовательский шаг: `make demo-api` и `make dev-web`; регрессии воспроизводятся `make check-smoke` / `make check-full`. Пользователь принимает результат; commits/push/deploy не выполнялись.
