# League Companion — backend

Go-бэкенд личного трекера статистики League of Legends: тянет данные из Riot Games API,
складывает в PostgreSQL и отдаёт Android-клиенту. Техзадание — в `../SPEC.md`,
состояние работ — в `../PROGRESS.md`.

Статус: закрыта веха «Дни 1–2» — скелет, конфиг, миграции, клиент к Riot API.
REST-хендлеров, rate limiter'а, кэша и sync worker'а ещё нет.

## Требования

- Go 1.25+
- PostgreSQL 16
- Redis (понадобится с вехи «Дни 3–4»)
- Ключ Riot API с https://developer.riotgames.com

## Переменные окружения

Все переменные читаются из окружения; локально удобно держать их в `backend/.env`
(файл в `.gitignore`, шаблон — `.env.example`).

| Переменная | Обяз. | По умолчанию | Назначение |
|---|---|---|---|
| `RIOT_API_KEY` | да | — | Development Key Riot. **Протухает раз в 24 часа**, перевыпускается вручную |
| `DATABASE_URL` | да | — | DSN PostgreSQL, например `postgres://league:league@localhost:5432/league_companion?sslmode=disable` |
| `CLIENT_API_KEY` | да | — | Shared secret для заголовка `X-API-Key` от Android-клиента |
| `REDIS_ADDR` | нет | `localhost:6379` | Адрес Redis |
| `HTTP_PORT` | нет | `8080` | Порт REST API |
| `LOG_LEVEL` | нет | `info` | `debug` / `info` / `warn` / `error` |
| `RIOT_HTTP_TIMEOUT` | нет | `10s` | Таймаут одного запроса к Riot |

Секреты не логируются: `Config` реализует `slog.LogValuer` и вырезает ключи, а из
`DATABASE_URL` — пароль.

## Запуск

```bash
cp .env.example .env          # и подставить свои значения

# Postgres на время разработки (полноценный docker-compose появится в вехе «День 8»)
docker run --rm -d --name lc-postgres -p 5432:5432 \
  -e POSTGRES_USER=league -e POSTGRES_PASSWORD=league -e POSTGRES_DB=league_companion \
  postgres:16

go run ./cmd/server
```

Миграции вшиты в бинарник и применяются автоматически при старте — CLI `migrate`
ставить не нужно.

Проверка, что сервис жив (health-check намеренно вне `/api/v1`, поэтому не требует
`X-API-Key`):

```bash
curl -i localhost:8080/healthz
# HTTP/1.1 200 OK
# X-Request-Id: ...
# {"status":"ok"}
```

Если база недоступна — `503` и единый формат ошибки:
`{"error":"database_unavailable","message":"база данных недоступна"}`.

## Ручная проверка Riot API

REST-хендлеров ещё нет, поэтому клиент к Riot проверяется отдельной утилитой —
она прогоняет все пять эндпоинтов из SPEC.md 3.2 и печатает результат:

```bash
go run ./cmd/riotcheck -region ru -riot-id "GameName#TAG"
go run ./cmd/riotcheck -region euw1 -riot-id "GameName#TAG" -count 10 -v
```

```
[1/5] account-v1     → puuid ...
[2/5] summoner-v4    → уровень 412, иконка 5678
[3/5] league-v4      → 1 очередей
                       RANKED_SOLO_5x5: GOLD II, 47 LP (63W/58L)
[4/5] match-v5 ids   → 5 шт.
[5/5] match-v5       → EUW1_7000000001
                       сырой JSON: 118432 байт (пойдёт в matches.raw_data)
```

Ошибка `401/403` почти всегда означает протухший ключ — утилита скажет об этом явно.

## Структура

```
cmd/server      точка входа сервиса: конфиг → миграции → пул БД → HTTP
cmd/riotcheck   CLI ручной проверки Riot API
internal/config     разбор и валидация переменных окружения
internal/riot       клиент Riot API: routing, DTO, типизированные ошибки
internal/domain     доменные модели и мапперы из DTO
internal/storage    пул pgx и вшитые миграции
internal/httpapi    роутер, middleware, health-check
```

### Region routing

Riot использует два несовместимых вида роутинга, и перепутать их легко
(риск из SPEC.md 7). Весь маппинг сосредоточен в `internal/riot/routing.go`:

- `PlatformHost("ru")` → `ru.api.riotgames.com` — Summoner-V4, League-V4
- `MatchRoute("ru")` → `europe` — Match-V5
- `AccountRoute("ru")` → `europe` — Account-V1

Match- и Account-маршруты расходятся на SEA-платформах (`oc1`, `sg2`, `ph2`, `th2`,
`tw2`, `vn2`): Match-V5 обслуживается на `sea`, Account-V1 там не публикуется,
поэтому для него используется `asia`.

### Схема БД

Схема — из SPEC.md 3.3 с двумя отклонениями по CLAUDE.md:

- `matches.raw_data JSONB NOT NULL` — полный JSON ответа Match-V5. Позволяет
  добавлять поля (предметы, руны, спеллы) без повторных запросов к Riot и отдавать
  всех 10 участников матча.
- `match_participants` хранит строки только для отслеживаемых саммонеров (на таблице
  стоит FK на `summoners`); полный состав обеих команд берётся из `raw_data`.

## Разработка

```bash
gofmt -l .
go vet ./...
golangci-lint run     # конфиг в .golangci.yml
go test ./...
go test ./... -race   # на Windows требует CGO и gcc; в Linux/докере работает как есть
```

TDD обязателен для region routing, rate limiter и агрегации статистики (CLAUDE.md).
