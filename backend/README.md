# League Companion — backend

Go-бэкенд личного трекера статистики League of Legends: тянет данные из Riot Games API,
складывает в PostgreSQL и отдаёт Android-клиенту. Техзадание — в `../SPEC.md`,
состояние работ — в `../PROGRESS.md`.

Статус: закрыты вехи «Дни 1–2» (скелет, конфиг, миграции, клиент к Riot API) и
«Дни 3–4» (ограничитель частоты, ретраи, кэш). REST-хендлеров и sync worker'а ещё нет,
поэтому лимитер и кэш пока подключены только в `cmd/riotcheck` — в сервис они
придут вместе с хендлерами.

## Требования

- Go 1.25+
- PostgreSQL 16
- Redis — необязателен, без него кэш работает в памяти процесса
- Ключ Riot API с https://developer.riotgames.com

## Переменные окружения

Все переменные читаются из окружения; локально удобно держать их в `.env`
(файл в `.gitignore`, шаблон — `backend/.env.example`).

`.env` ищется в двух местах: `backend/.env` и `.env` в корне репозитория —
загружаются оба, при совпадении ключа выигрывает `backend/.env`. Корень поддержан
потому, что `docker-compose.yml` из вехи «День 8» подхватывает `.env` именно оттуда.
Уже заданные переменные окружения приоритетнее файлов, так что в докере и CI ничего
переопределять не нужно.

| Переменная | Обяз. | По умолчанию | Назначение |
|---|---|---|---|
| `RIOT_API_KEY` | да | — | Development Key Riot. **Протухает раз в 24 часа**, перевыпускается вручную |
| `DATABASE_URL` | да | — | DSN PostgreSQL, например `postgres://league:league@localhost:5432/league_companion?sslmode=disable` |
| `CLIENT_API_KEY` | да | — | Shared secret для заголовка `X-API-Key` от Android-клиента |
| `REDIS_ADDR` | нет | `localhost:6379` | Адрес Redis. Недоступен — кэш работает в памяти |
| `HTTP_PORT` | нет | `8080` | Порт REST API |
| `LOG_LEVEL` | нет | `info` | `debug` / `info` / `warn` / `error` |
| `RIOT_HTTP_TIMEOUT` | нет | `10s` | Таймаут одного запроса к Riot |

Секреты не логируются: `Config` реализует `slog.LogValuer` и вырезает ключи, а из
`DATABASE_URL` — пароль.

## Запуск

```bash
cp backend/.env.example .env  # в корень репозитория, и подставить свои значения

# Postgres и Redis на время разработки (полноценный docker-compose — веха «День 8»)
docker run --rm -d --name lc-postgres -p 5432:5432 \
  -e POSTGRES_USER=league -e POSTGRES_PASSWORD=league -e POSTGRES_DB=league_companion \
  postgres:16
docker run --rm -d --name lc-redis -p 6379:6379 redis:7

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

# несколько проходов подряд — видно работу кэша
go run ./cmd/riotcheck -region ru -riot-id "GameName#TAG" -repeat 3
go run ./cmd/riotcheck -region ru -riot-id "GameName#TAG" -redis none   # без Redis
```

```
Кэш: redis

=== проход 1 из 2 ===
[1/5] account-v1     → puuid ...  [615ms]
[2/5] summoner-v4    → уровень 412, иконка 5678  [539ms]
[3/5] league-v4      → 1 очередей  [306ms]
                       RANKED_SOLO_5x5: GOLD II, 47 LP (63W/58L)
[4/5] match-v5 ids   → 5 шт.  [394ms]
[5/5] match-v5       → EUW1_7000000001  [664ms]
                       сырой JSON: 118432 байт (пойдёт в matches.raw_data)

=== проход 2 из 2 ===
[1/5] account-v1     → puuid ...  [2ms]     ← из кэша
[2/5] summoner-v4    → уровень 412, иконка 5678  [1ms]
[3/5] league-v4      → 1 очередей  [1ms]
[4/5] match-v5 ids   → 5 шт.  [1ms]
[5/5] match-v5       → EUW1_7000000001  [577ms]   ← детали матча не кэшируются
```

Ошибка `401/403` почти всегда означает протухший ключ — утилита скажет об этом явно.

## Структура

```
cmd/server      точка входа сервиса: конфиг → миграции → пул БД → HTTP
cmd/riotcheck   CLI ручной проверки Riot API
internal/config     разбор и валидация переменных окружения
internal/riot       клиент Riot API: routing, DTO, типизированные ошибки, кэш и ретраи
internal/ratelimit  ограничитель частоты запросов к Riot
internal/cache      кэш ответов Riot: Redis и in-memory
internal/domain     доменные модели и мапперы из DTO
internal/storage    пул pgx и вшитые миграции
internal/httpapi    роутер, middleware, health-check
```

### Лимиты Riot, ретраи и кэш

Personal Development Key даёт 20 запросов/сек и 100 запросов/2 минуты, причём оба
лимита действуют одновременно и на весь ключ, а не на пользователя (SPEC.md 3.2).
Поэтому `ratelimit.Limiter` должен быть **один на процесс** — его делят HTTP-хендлеры
и будущий sync worker; отдельный лимитер на каждый клиент означал бы, что лимит
не соблюдается.

Считаем скользящим окном, а не токен-бакетом `golang.org/x/time/rate`: бакет с
`burst = N` доливает токены внутри того же окна и, выпустив 20 запросов в t=0.9с,
выпустит 21-й уже в t=0.95с — по счётчику Riot это 21 запрос за одну секунду.
Берст после простоя при этом сохраняется, так что первая синхронизация саммонера
не растягивается.

Повторы: на `429` выдерживается ровно `Retry-After`, названный Riot; на `5xx` —
экспонента с джиттером (3 попытки, база 500 мс, потолок 30 с). `404` и протухший
ключ не повторяются. Пауза длиннее оставшегося времени контекста не начинается.

Кэшируются только «мелкие» ответы:

| Вызов | TTL |
|---|---|
| Account-V1 (Riot ID → PUUID) | 24 ч |
| Summoner-V4 | 10 мин |
| League-V4 | 5 мин |
| Match-V5, список id | 60 с |
| Match-V5, детали матча | не кэшируются |

Детали матча неизменяемы и целиком ложатся в `matches.raw_data` — Postgres и есть
их кэш, дублировать сотни килобайт в Redis незачем.

Кэш необязателен: недоступный Redis или пустой `REDIS_ADDR` дают предупреждение
в лог и переключение на кэш в памяти процесса. Ошибка кэша в рантайме не валит
запрос — он просто идёт в Riot.

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

Тесты не требуют ни Postgres, ни Redis: кэш проверяется на `miniredis`, а лимитер
и backoff — на подменённых часах, поэтому двухминутное окно Riot проверяется мгновенно.
