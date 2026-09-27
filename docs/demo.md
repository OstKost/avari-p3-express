# Демонстрационные данные и проверки

Из корня репозитория:

```sh
make demo-seed
make demo-api    # localhost:4820; остановите обычный API на этом порту
# В другом терминале:
make dev-web    # localhost:4810
```

Ключ входа менеджера — `.demo/manager.key` после запуска API. Если браузер помнит обычный API, выйдите и войдите с демо-ключом. Рабочая `apps/api/data/p3.db` не используется; DB_PATH из окружения не влияет на заполнение.

После остановки API:

```sh
make demo-reset  # заменить демо новым набором
make demo-clean  # удалить все поколения и legacy данные; сохранить ключ и посторонние файлы
make demo-test   # сбои, сигналы, integrity, reproducibility, WAL restore и HTTP
```

Данные лежат в `.demo/generations/<id>/`, активное поколение выбирает `.demo/current.json`. Путь базы и manifest:

```sh
python3 scripts/demo.py paths
```

Команда возвращает JSON с `db`, `manifest`, `manager_key`. Не используйте старый путь `.demo/demo.db` и не запускайте API напрямую с демо-базой: блокировка действует в оболочке и её дочернем API. При SIGKILL оболочки API остаётся работать и удерживать lock; остановите PID, напечатанный как `Demo API PID`, прежде чем reset/clean.

## Сценарии и фикстуры

Профиль `demo` (по умолчанию): три проекта, чеклисты, ссылки/комментарии, цикл B, блокер AI-пилота, SDLC, просроченные действия, статья и 21 задача. Задачи используют разные демонстрационные роли исполнителя, reviewer и менеджера; принятые результаты имеют синтетический независимый QA. Токены не создаются — для MCP выдайте их на экране доступа.

Профиль `smoke`: один проект, две задачи (ready/in_review), те же P3/SDLC основы. Оба профиля используют реальные сервисы. Все отчёты, проверки и example.com ссылки явно **синтетические**.

```sh
python3 scripts/demo.py seed --dir /tmp/avari-my-test --profile smoke --date 2026-09-27
python3 scripts/demo.py paths --dir /tmp/avari-my-test
python3 scripts/demo.py api --dir /tmp/avari-my-test
# После остановки API:
python3 scripts/demo.py reset --dir /tmp/avari-my-test --profile smoke --date 2026-09-27
python3 scripts/demo.py clean --dir /tmp/avari-my-test
```

Каталог должен отсутствовать или принадлежать инструменту. Native Windows не поддерживается; используйте Linux/WSL. Для параллельных API нужны разные каталоги и PORT.

Manifest v2 содержит fixture_version, profile, reference_date, семантические ключи проектов shop/ai/crm, задач `0/ready`, шагов `0/A01`, `0/B01` и ожидаемые исходные состояния задач. UUID и время аудита меняются при reset; нормализованное содержание воспроизводимо. По умолчанию дата 2026-09-27; для живого показа задайте нужную дату явно. Общий project.progress хранится отдельно; чеклист пересчитывает шаг и фазу, а приёмка задачи сама не меняет управленческий прогресс.

Повторный seed проверяет исправность существующего набора и не добавляет записи. Несовпадение профиля/даты, версий, ID или manifest даёт ошибку и требует явного reset. Legacy база из прежней реализации сохраняется до clean; для перехода выполните reset.

## Сброс и восстановление

Новый набор полностью готовится, проверяется и синхронизируется перед атомарным переключением current.json. Сбой до переключения оставляет прежний набор; после переключения доступен новый набор целиком. Ошибка после commit может вернуть nonzero exit с исправным новым набором — выполните paths. Старые и незавершённые поколения удаляются только clean. Reset/clean удаляют изменения в выбранном демо; рабочая база не затрагивается.

```sh
python3 scripts/demo.py backup --to /tmp/avari-demo-backup
python3 scripts/demo.py restore --from /tmp/avari-demo-backup
```

Остановите демо-API перед backup/restore. Копирование выполняет SQLite backup API, включая committed WAL страницы. Не копируйте один .db у работающей базы. Backup-каталог должен быть новым/пустым owned каталогом; повторное заполнение отвергается. Restore создаёт поколение, сохраняет существующий ключ менеджера; Ключ менеджера и plaintext токены не копируются. Таблица agent_tokens со scopes и хешами сохраняется как часть базы.

## Команды проверки

```sh
make check-smoke   # демо regression tests + 3 P3 browser flows
make check-full    # harness + Go/race/CGO=0 + lint/type/build + 4 browser flows
make check-browser # только browser flows
make check         # прежний технический gate, без браузера
```

Требуются Go 1.25, Python 3.11+, Node 20, pnpm 9 и зависимости `apps/web` (`pnpm install --frozen-lockfile`). Chromium установить один раз: `cd apps/web && pnpm exec playwright install chromium` (Linux CI: `--with-deps`). Проверки не устанавливают зависимости автоматически.

Браузерные прогоны выбирают независимые временные каталоги и свободные порты. Retries отключены. Для явно заданных AVARI_TEST_API_PORT/WEB_PORT ответственность за свободные порты у вызывающего. Логи, версии и summary сохраняются в `.harness/runs/`; browser artifacts включают снимки и HTML-отчёт. Trace отключён, временные базы/ключи удаляются; ключи и токены не включаются в evidence.
