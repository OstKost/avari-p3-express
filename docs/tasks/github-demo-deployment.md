# Задача: GitHub CI/CD и демо на VPS

Статус: in-progress

## Результат и границы

Запрос пользователя: тесты в GitHub, релиз из main по тегу версии и развёртывание p3express.avari.dev на VPS 176.53.174.118 с OpenResty Manager. Пользователь разрешил настройку и развёртывание. Главный агент владеет workflows/deployment/docs; API агент выполнил opt-in public HTTPS manager boundary; независимый reviewer проверяет контракт и деплой.

## Приёмка

- [ ] Тег из main проходит все CI gates и развёртывает тот же commit.
- [ ] HTTPS демо загружается, вход по ключу и реальная работа с seeded проектами проходят.
- [x] Ошибка кандидата сохраняет активный релиз/базу; rollback сохраняет обе базы в автоматических тестах.
- [ ] Фактические проверки VPS/GitHub и ограничения записаны.

## Контракт и решения

[ADR 0005](../decisions/0005-versioned-demo-deployment.md), [runbook](../deployment.md). Fresh demo volume на новый релиз, отдельный стабильный credential volume, loopback gateway и two-slot switch. HTTPS origin exact allowlist, ключ обязателен. SHA256 checksums и Docker image IDs проверяются до запуска.

## Проверки

10 deployment + 2 package regression tests PASS: switch/rollback, health failure, post-switch restore, gateway cleanup, idempotency, checksum corruption, archive traversal, sudo env propagation. Local make check-full: 10/10 PASS (.harness/runs/full-lp2c3t1o); последующие affected web/harness PASS. GitHub v0.1.1: все CI jobs PASS, deployment остановлен до запуска на различии classic/containerd IDs. Реальный архив проверен новым packager на VPS: вычисленный manifest digest совпадает с Docker .Id. Certbot dry-run renewal PASS. Независимое review: ошибки sudo env, gateway cleanup, mutable image tags и external version check исправлены. Дополнить успешным runtime после v0.1.2.

## Передача контекста

Следующее: завершить local check-full, независимое ревью; первичная публикация main/v0.1.0 и наблюдение Actions; HTTPS/browser/rollback на VPS. Секреты и полный evidence только в ignored .harness/ и GitHub Secrets.
