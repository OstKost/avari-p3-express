# Производственные задачи: REST и MCP

Реализовано локальное личное пространство. Старые `/api/v1/p3` и Action сохраняются; теперь production router требует менеджерскую сессию для управленческих ресурсов. API по умолчанию слушает `127.0.0.1:4820`. База по умолчанию `apps/api/data/p3.db` при запуске из Go-модуля.

## Менеджер

При первом старте создаётся `data/manager.key` с правами 0600. Путь переопределяется `MANAGER_KEY_PATH`. Введите содержимое файла на экране входа. Ключ не является агентским токеном. Не передавайте его агенту. `POST /api/session` принимает `{ "key": "<manager credential>" }`, заголовок `X-Avari-Local: manager`, выдаёт HttpOnly SameSite=Strict cookie на 24 часа. `GET /api/session` проверяет cookie. После перезапуска API необходимо войти снова; UI сохраняет URL и предлагает вход без повторения мутации.

Доступ предназначен для доверенной локальной машины: процессы с доступом к файлам менеджера могут читать его ключ. Приложение разделяет HTTP полномочия, но не является песочницей для недоверенного кода на том же OS аккаунте.

`GET/POST /api/v1/agent-tokens`: менеджер видит метаданные и создаёт `{name,projects:[project_id]}`. Ответ создания `{token:{id,name,projects,revoked},secret}` показывает secret один раз. В SQLite хранится только SHA-256 хеш. `DELETE /api/v1/agent-tokens/{id}` отзывает токен; отзыв не останавливает попытку автоматически, менеджер может её остановить.

## Задачи и попытки

`POST /api/v1/work-tasks` принимает `project_id` и поля задания: `goal`, `expected_result`, `criteria:string[]`, `priority:low|medium|high` (по умолчанию medium), `due_date` (YYYY-MM-DD или пустая строка), `assignee`, `step_ids:string[]`, `stage_id`, `dependencies:string[]`. Пустые массивы сериализуются как `[]`. Создаётся draft, ответ 201 — WorkTask.

`GET /api/v1/work-tasks` возвращает `{data:WorkTask[],total}`. Фильтры: project_id, status, stage_id, step_id, assignee; limit 1–100 (50 по умолчанию), offset >= 0. `GET /api/v1/work-tasks/{id}` возвращает `{task,project,dependencies,linked_steps}`. linked_steps содержит code, name, cycle_id, cycle_number и phase_code, включая предыдущие циклы. Runs, results, reviews и events находятся в task.

`POST /api/v1/work-tasks/{id}/{op}` принимает `version` текущей задачи и поля операции. Успех 200 возвращает весь обновлённый WorkTask. Версия увеличивается ровно на одно событие; бизнес-изменение и событие сохраняются одной транзакцией.

- `edit`: `{version,spec:WorkSpec}`, менеджер, draft/ready. Изменение active задания требует `finish` текущей попытки.
- `ready`: `{version}`, менеджер; цель, ожидаемый результат и непустые критерии обязательны.
- `start`: `{version,idempotency_key}`, ready, без блокера и с done зависимостями. Создаёт один active AgentRun со снимком задания. Автор определяется credential.
- `progress`, `comment`: `{version,run_id,text}` для active попытки.
- `blocker`: `{version,run_id,text}`, пустой text очищает блокер; статус не меняется.
- `result`: `{version,run_id,idempotency_key,result:{description,kind,urls:string[],artifact_version}}`; ссылки только HTTP(S). Генерируемые ID, авторство и время задаёт сервер.
- `submit`: `{version,run_id,idempotency_key,text,checks:[{criterion:0,outcome:pass|fail|unknown,method,source,evidence}]}`. Нужны отчёт, хотя бы один результат и ровно одна проверка на каждый критерий снимка. Получаем in_review / submitted. pass — утверждение исполнителя, а не независимое подтверждение.
- `finish`: `{version,run_id,text}` — обязательная причина остановки. Attempt становится stopped, задача ready; поздние сообщения отклоняются.
- `review`: `{version,run_id,decision:accept|return,text}`, только менеджер, только submitted попытка задачи in_review; возврат требует замечание. accept → done; return → ready. Все предыдущие попытки сохраняются.
- `cancel`: `{version}`, менеджер; переводит задачу в cancelled, активную попытку останавливает.

Для start/result/submit повторите тот же ключ и payload при потере ответа. Успешный повтор возвращает текущее состояние без нового события, даже если версия уже изменилась. Повтор result/submit с другим payload даёт 409. Новая попытка/результат/сдача требует новый ключ. Агент может изменять только собственную active попытку в разрешённом проекте; менеджер может остановить зависшую попытку.

`GET /api/v1/agent-runs`: project_id, task_id, limit, offset. `GET /api/v1/task-results`: project_id, task_id, stage_id, step_id, actor, state=pending|accept|return|cancelled, limit, offset. Результаты сдач возвращаются `{result,state,actor,review,step_ids,stage_id}`; связи берутся из снимка попытки, поэтому редактирование возвращённого задания не переписывает происхождение результата. P3.express, чеклисты и SDLC автоматически не меняются.

Ошибки JSON `{error}`: 400 невалидные данные; 401 отсутствующая/истёкшая сессия или неизвестный/отозванный токен; 403 роль, чужой проект/попытка или недоверенный origin; 404 объект не существует; 409 конфликт состояния, версии или ключа; 500 внутренний сбой. HTTP request bodies и Authorization/cookie не логируются.

## MCP stdio

В `apps/api` выполните `go build -o bin/avari-mcp ./cmd/mcp`. Официальный `github.com/modelcontextprotocol/go-sdk` закреплён на v1.8.0; Go-модуль, CI и Docker используют Go 1.25.

Настройте внешний MCP клиент на бинарник `apps/api/bin/avari-mcp`, передав окружение:

```json
{
  "mcpServers": {
    "avari": {
      "command": "/absolute/path/to/apps/api/bin/avari-mcp",
      "env": {
        "AVARI_API_URL": "http://127.0.0.1:4820",
        "AVARI_AGENT_TOKEN": "<agent credential>"
      }
    }
  }
}
```

MCP обращается только к loopback HTTP API; redirects запрещены. SQLite напрямую не открывает. stdout содержит только протокол, ошибки запуска идут в stderr без credential.

Инструменты: list_projects, get_project_context, list_tasks, get_task, start_task, report_progress, report_blocker, add_comment, add_result, submit_task, finish_run. Списки поддерживают limit/offset и task filters. get_task включает проект, задание, зависимости, точные связи, предыдущие замечания, результаты и историю. Мутации используют task_id, version и run_id (кроме start); result/start/submit требуют idempotency_key. Окончательной приёмки, SQL, shell и изменения управленческих статусов нет. Агентские токены также не могут обращаться к старым P3 ресурсам; read context доступен через `/work-projects`.

## Проверка

`make check-api` включает временную SQLite миграцию, state/concurrency/access/idempotency тесты и настоящий протокольный клиент, который запускает MCP подпроцесс через stdio. `cd apps/web && pnpm exec playwright install chromium && pnpm test:e2e` проверяет браузерную постановку, ручную первую попытку, возврат, сдачу внешним stdio MCP процессом, приёмку, реальный перезапуск API, проекции и 375px в обеих темах. Browser tests используют отдельные порты 14810/14820 и временную базу; штатный сервер не затрагивается. Отчёты и screenshots — `.harness/runs/`.


## Независимый QA (миграция 5)

WorkSpec добавляет requires_independent_review:boolean=false. POST agent-tokens принимает role=executor|reviewer (executor по умолчанию). Старые токены остаются executor; изменение роли требует нового токена. Reviewer может читать проектный контекст, но все work-tasks commands и P3 ресурсы для него запрещены (403).

AgentRun добавляет submission_digest:string и verification_reports:VerificationReport[]. Сдача фиксирует digest по snapshot/results/report/checks; последующие QA и решения не меняют его. Старые submitted attempts получают digest при startup backfill без изменения версии/событий. Task-results возвращает digest и отчёты рядом с result.

POST /api/v1/agent-runs/{run_id}/verification-reports принимает {submission_digest,kind:qa|security,summary,checks:[CriterionCheck],idempotency_key}. Только reviewer с scope проекта и ID, отличным от executor. ID/author/time выдаёт сервер. QA должен указать каждый критерий; security может часть. outcome=pass|fail|unknown, method/source/evidence обязательны. Ответ 201 WorkTask. Отчёт append-only, событие содержит verification_report_id. Повтор идентичного успешного запроса не меняет историю; иной payload/автор для занятого ключа даёт 409. Новый отчёт для закрытой сдачи или неверного digest — 409. Невалидный input — 400; роль/scope/self-review — 403.

Accept обязательного QA без полного QA отчёта по digest — 409. Независимые fail/unknown требуют comment менеджера (400 без него). Право окончательной приёмки по-прежнему только manager; security не заменяет required QA. Внешний агент использует новый report_verification MCP tool с теми же полями и run_id, без manager credential. Полный workflow и пакет: [agent-system.md](agent-system.md).
