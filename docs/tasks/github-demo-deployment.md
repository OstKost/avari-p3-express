# Задача: GitHub CI/CD и демо на VPS

Статус: done

## Результат и границы

Запрос пользователя: тесты в GitHub, релиз из main по тегу версии и развёртывание p3express.avari.dev на VPS 176.53.174.118 с OpenResty Manager. Пользователь разрешил настройку и развёртывание. Главный агент владеет workflows/deployment/docs; API агент выполнил opt-in public HTTPS manager boundary; независимый reviewer проверяет контракт и деплой.

## Приёмка

- [x] Тег из main проходит все CI gates и развёртывает тот же commit.
- [x] HTTPS демо загружается, вход по ключу и реальная работа с seeded проектами проходят.
- [x] Ошибка кандидата сохраняет активный релиз/базу; rollback сохраняет обе базы в автоматических тестах.
- [x] Фактические проверки VPS/GitHub и ограничения записаны.

## Контракт и решения

[ADR 0005](../decisions/0005-versioned-demo-deployment.md), [runbook](../deployment.md). Fresh demo volume на новый релиз, отдельный стабильный credential volume, loopback gateway и two-slot switch. HTTPS origin exact allowlist, ключ обязателен. SHA256 checksums и Docker image IDs проверяются до запуска.

## Проверки

10 deployment + 2 package regression tests PASS: switch/rollback, health failure, post-switch restore, gateway cleanup, idempotency, checksum corruption, archive traversal, sudo env propagation. Local make check-full: 10/10 PASS (.harness/runs/full-lp2c3t1o); последующие affected web/harness PASS. GitHub v0.1.1: все CI jobs PASS, deployment остановлен до запуска на различии classic/containerd IDs. Реальный архив проверен новым packager на VPS: вычисленный manifest digest совпадает с Docker .Id. Certbot dry-run renewal PASS. Независимое review: ошибки sudo env, gateway cleanup, mutable image tags и external version check исправлены. GitHub [v0.1.2](https://github.com/OstKost/avari-p3-express/actions/runs/36347159158) и [v0.1.3](https://github.com/OstKost/avari-p3-express/actions/runs/36347478719): все CI + Docker build + SSH deployment + external health/version checks PASS. Оба тега на commit 8688fb33a982b3154e1027b3306d235e7172161c — проверен rollback при разных web versions на одном commit.

Live Chromium на обоих релизах: неверный ключ отклонён, правильный вход; Secure/HttpOnly/SameSiteStrict cookie; 3 проекта; keyboard checklist mutation сохраняется после reload; чужой Origin отклонён; 375px без horizontal overflow; desktop в обеих темах; pageerrors отсутствуют. Evidence .harness/deploy/browser-v0.1.2.json, browser-evidence.json, live-*.png.

VPS: v0.1.2 → v0.1.3 fresh DB → rollback v0.1.2 с сохранённым comment ID → rollback v0.1.3 со своей базой; повтор active apply не пересоздаёт данные. Evidence .harness/deploy/marker-v0.1.2.json, marker-v0.1.3.json. Активно v0.1.3, previous v0.1.2.

Параллельный main CI обнаружил macOS race: SIGKILL потомков не означает немедленный выход; lock assertion не ослаблен. Runner теперь ждёт absence/zombie всех tracked descendants до возврата (bounded 5s). Независимое review без findings, local focused runner tests 6/6 и affected make check-harness 2/2 PASS (.harness/runs/harness-tiji6b3k).

## Передача контекста

Демо работает по HTTPS, CI/CD и Environment secrets настроены, следующий выпуск — новый тег из main по runbook. Секреты и полный evidence только в ignored .harness/ и GitHub Secrets. Ограничения: отдельный OpenResty include вне UI Manager; архивы/volumes сохраняются без автоматического удаления; SIGKILL/power-loss между gateway reload/state требует operator reconciliation (ADR 0005).
