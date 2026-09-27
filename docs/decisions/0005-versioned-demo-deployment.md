# ADR: версионное демо на VPS

Дата: 2026-09-27. Статус: accepted.

## Решение

GitHub Actions запускает существующие проверки и Chromium сценарии для main/PR. Релизный тег vMAJOR.MINOR.PATCH обязан указывать на commit, достижимый из main; reusable CI проверяет этот же commit до сборки Docker образов. Environment demo хранит отдельный SSH ключ и закреплённый host key VPS. Образы Linux amd64 передаются архивом с SHA256SUMS и проверенными config/OCI manifest digests; registry credentials не нужны. Docker classic и containerd используют разные ID: пакет содержит config digest и digests OCI manifests, а сервер сохраняет точный загруженный runtime ID для Compose/rollback. Старый classic archive без OCI manifests допускается только при совпадении config ID; неожиданный ID отклоняется.

Деплой готовит отдельную SQLite базу в новом Docker volume и запускает API/web на свободном из двух локальных портов. После проверки /healthz и /version.json отдельный gateway переключается на кандидата. Предыдущие контейнеры останавливаются, данные сохраняются; явный rollback возвращает их без генерации. Повтор активной версии сохраняет изменения демо. Деплой сериализован GitHub concurrency и серверным flock. SIGTERM и обычные ошибки выполняют rollback; SIGKILL/отказ питания между reload и сохранением state требуют сверки state.json и upstream.inc оператором: полной атомарности двух ресурсов не обещаем.

OpenResty Manager обслуживает существующие сайты; отдельный include p3express-demo.conf проксирует HTTPS на gateway. Сертификат этого сайта обслуживает Certbot с scoped deploy hook. Include не зарегистрирован в базе/UI Manager, поэтому редактировать его нужно как инфраструктурный файл, а не создавать дубликат сайта в UI.

PUBLIC_MANAGER_ORIGIN включает вход по ключу только на точном HTTPS origin/Host. Secure cookie и проверка Origin сохраняют защиту браузерных изменений. Ключ отделён от сменяемых демо-данных. Публичная публикация без аутентификации не включается.

## Последствия

Новый релиз сбрасывает демо-сценарии; прежние изменения остаются в старом volume. Архивы и volumes автоматически не чистятся: оператор планирует удаление после проверки отката. Rootless Docker не требуется: deployment user использует уже разрешённый sudo docker. Отдельный SSH ключ ограничен forwarding/PTY, но даёт shell и sudo пользователя VPS; для более строгой изоляции нужен отдельный ограниченный deploy user.
