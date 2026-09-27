# Демо: GitHub Actions → VPS

Адрес: https://p3express.avari.dev. Сервер user@176.53.174.118, приложение /opt/avari-p3-express. Вход по ключу менеджера. Секрет не входит в репозиторий, build args, release artifact или демо-базу.

## Выпуск

Из main после изменений:

```sh
git push origin main
git tag -a v0.1.1 -m 'Demo v0.1.1'
git push origin v0.1.1
```

Только тег vMAJOR.MINOR.PATCH из истории main запускает deployment. Сначала Linux/macOS harness, API race tests, lint/typecheck/build и Chromium UI/MCP тесты, затем Docker сборка и SSH deployment. Прогресс — Actions → Versioned demo deployment, Environment demo. Теги неизменяемы; исправление выпускается следующей версией.

Environment demo допускает только теги v*. Workflow дополнительно проверяет формат версии и ancestry main. Environment demo содержит DEMO_SSH_PRIVATE_KEY и DEMO_KNOWN_HOSTS. Ключ VPS закреплён, StrictHostKeyChecking=yes. При смене host key сначала проверить его вне CI, затем обновить Secret. CI key можно отозвать удалением строки avari-p3-github-demo из ~/.ssh/authorized_keys.

## Вход и состояние

Получить ключ на своём терминале (не отправлять в логи/issue):

```sh
ssh user@176.53.174.118 'sudo docker run --rm -v p3express_demo_credentials:/credentials:ro alpine:3.21 cat /credentials/manager.key'
```

Проверить состояние:

```sh
curl -f https://p3express.avari.dev/healthz
curl -f https://p3express.avari.dev/version.json
ssh user@176.53.174.118 'cat /opt/avari-p3-express/state.json'
```

Каждый новый релиз создаёт свежие 3 проекта/21 задачу. Прежний volume сохраняет пользовательские изменения. Повтор того же активного релиза не пересоздаёт данные. manifest лежит в releases/<version>-<run>/fixture.json. Порт gateway 14800, сменяемые web 14801/14802 привязаны к loopback; API доступен только в compose network.

## Откат

Сначала посмотреть state.json. Запустить deploy.py из текущего проверенного архива:

```sh
ssh user@176.53.174.118 'cd /opt/avari-p3-express && release=$(python3 -c "import json; print(json.load(open(\"state.json\"))[\"current\"][\"release\"])") && python3 "$release/deploy.py" rollback'
```

Откат запускает предыдущую версию с сохранённой базой, проверяет её, переключает gateway и останавливает текущую. Повтор rollback переключает обратно. Ошибка до переключения оставляет активный релиз; ошибка проверки после переключения восстанавливает старый upstream/state. После SIGKILL/отказа питания вручную сверить upstream.inc/state.json/version.json до повторного деплоя.

## OpenResty и сертификат

Конфигурация: deployments/demo/openresty.conf → /opt/om/nginx/conf/sites/p3express-demo.conf. Это отдельный include, не запись в UI OpenResty Manager; не создавать вторую конфигурацию домена. Проверка/reload:

```sh
sudo openresty -p /opt/om/nginx/ -t
sudo openresty -p /opt/om/nginx/ -s reload
sudo certbot renew --dry-run --cert-name p3express.avari.dev
```

Certbot timer обновляет сертификат, scoped hook deployments/demo/renew-certificate.sh перезагружает OpenResty. Остальные сертификаты Manager обслуживает сам Manager.

Архивы/volumes сохраняются без автоматической очистки. Перед ручным удалением проверить current/previous в state.json, labels io.avari.demo=true и резервную копию; credentials volume никогда не удалять ради сброса демо. Для полного обновления демо выпускать новый тег. Детали гарантий и ограничений: [ADR 0005](decisions/0005-versioned-demo-deployment.md).
