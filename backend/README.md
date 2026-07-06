# League Companion — backend

Go-бэкенд личного трекера статистики League of Legends: тянет данные из Riot Games API,
складывает в PostgreSQL и отдаёт Android-клиенту. Техзадание — в `../SPEC.md`,
состояние работ — в `../PROGRESS.md`.

Статус: закрыты вехи «Дни 1–2» (скелет, конфиг, миграции, клиент к Riot API),
«Дни 3–4» (ограничитель частоты, ретраи, кэш) и «Дни 5–6» (репозитории, синхронизация,
REST API). Периодического тикера ещё нет — матчи подтягиваются при добавлении
саммонера; тикер и агрегация статистики придут в вехе «День 7».

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

## REST API

Все `/api/v1/*` требуют заголовок `X-API-Key` со значением `CLIENT_API_KEY`
(CLAUDE.md, отклонение 3). `/healthz` — единственное исключение: его дёргают
мониторинг и `docker-compose`, знать секрет им незачем.

| Метод | Путь | Что делает |
|---|---|---|
| `POST` | `/api/v1/summoners` | Добавляет саммонера по Riot ID. `201` — добавлен впервые, `200` — уже отслеживался |
| `GET` | `/api/v1/summoners/{puuid}` | Профиль и ранги по очередям |
| `GET` | `/api/v1/summoners/{puuid}/matches?limit=20&offset=0` | Лента матчей глазами этого саммонера, `limit` 1..100 |
| `GET` | `/api/v1/matches/{matchId}` | Полные детали матча: обе команды, все 10 участников |

### Добавить саммонера

`riotId` — игровое имя **без** тега, тег передаётся отдельно в `tagLine`;
`region` — platform-регион (`ru`, `euw1`, `kr`, …), не regional-маршрут вроде `europe`.

```bash
curl -X POST localhost:8080/api/v1/summoners \
  -H "X-API-Key: $CLIENT_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"riotId":"Hide on bush","tagLine":"KR1","region":"kr"}'
```

```json
{
  "puuid": "...",
  "riotId": "Hide on bush",
  "tagLine": "KR1",
  "region": "kr",
  "summonerLevel": 782,
  "profileIconId": 6,
  "lastSyncedAt": null,
  "createdAt": "2026-07-29T10:00:00Z",
  "ranked": [
    {
      "queueType": "RANKED_SOLO_5x5",
      "tier": "CHALLENGER",
      "rank": "I",
      "leaguePoints": 1123,
      "wins": 240,
      "losses": 180,
      "updatedAt": "2026-07-29T10:00:00Z"
    }
  ]
}
```

Профиль и ранг тянутся синхронно — три запроса к Riot, около полутора секунд.
Матчи (ещё два десятка запросов) уходят в фоновый runner: в логах видно
`фоновая синхронизация завершена`, после чего `lastSyncedAt` перестаёт быть `null`.

### Профиль и лента матчей

Оба `GET`'а читают только Postgres и в Riot не ходят — иначе в 200 мс (SPEC.md 3.6)
не уложиться. Свежесть обеспечивает фоновая синхронизация.

```bash
curl -H "X-API-Key: $CLIENT_API_KEY" localhost:8080/api/v1/summoners/$PUUID
curl -H "X-API-Key: $CLIENT_API_KEY" \
  "localhost:8080/api/v1/summoners/$PUUID/matches?limit=5&offset=0"
```

```json
{
  "items": [
    {
      "matchId": "KR_7000000001",
      "gameCreation": "2026-07-28T21:14:00Z",
      "gameDurationSeconds": 1500,
      "queueId": 420,
      "gameVersion": "14.1.556.1234",
      "championName": "Ahri",
      "kills": 11, "deaths": 3, "assists": 9,
      "kda": 6.666666666666667,
      "win": true,
      "cs": 201,
      "goldEarned": 14320
    }
  ],
  "limit": 5,
  "offset": 0,
  "total": 20
}
```

`total` — сколько матчей сохранено всего, а не размер страницы: по нему клиент
рисует пагинацию.

### Детали матча

Отдаётся исходный ответ Match-V5 из `matches.raw_data` как есть (CLAUDE.md,
отклонение 1): там уже лежат предметы, руны и спеллы, а `match_participants`
хранит строки только для отслеживаемых саммонеров.

```bash
curl -H "X-API-Key: $CLIENT_API_KEY" localhost:8080/api/v1/matches/KR_7000000001
```

### Ошибки

Единый формат `{"error", "message"}`, где `error` — машинный код:

| Код HTTP | `error` | Когда |
|---|---|---|
| `400` | `invalid_body` | Тело не разбирается или содержит лишние поля |
| `400` | `invalid_request` | Пустой `riotId`/`tagLine`/`region`, тег внутри `riotId`, неизвестный регион |
| `400` | `invalid_pagination` | `limit` вне `1..100`, отрицательный `offset`, не число |
| `400` | `invalid_region` | Регион не прошёл маппинг уже в клиенте Riot |
| `401` | `unauthorized` | Нет или неверный `X-API-Key` |
| `404` | `summoner_not_found` | Riot не знает такой Riot ID либо саммонер не отслеживается |
| `404` | `match_not_found` | Матча нет в базе — появится после синхронизации |
| `429` | `rate_limited` | Упёрлись в лимит Riot; в `Retry-After` — срок, названный Riot |
| `502` | `riot_unauthorized` | Riot отклонил **наш** ключ (скорее всего протух) — это не про клиента, поэтому не `401` |
| `502` | `riot_unavailable` | Riot ответил `5xx` или неожиданным статусом |
| `504` | `riot_timeout` | Riot не ответил вовремя |
| `500` | `internal_error` | Ошибка базы; детали уходят только в лог |

## Ручная проверка Riot API

Клиент к Riot можно прогнать отдельно от сервиса — утилита обходит все пять
эндпоинтов из SPEC.md 3.2 и печатает результат. Удобно, чтобы отличить протухший
ключ от ошибки в коде:

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
cmd/server      точка входа: конфиг → миграции → пул БД → клиент Riot → фоновый runner → HTTP
cmd/riotcheck   CLI ручной проверки Riot API
internal/config     разбор и валидация переменных окружения
internal/riot       клиент Riot API: routing, DTO, типизированные ошибки, кэш и ретраи
internal/ratelimit  ограничитель частоты запросов к Riot
internal/cache      кэш ответов Riot: Redis и in-memory
internal/domain     доменные модели и мапперы из DTO
internal/storage    пул pgx, вшитые миграции, репозитории
internal/syncer     синхронизация Riot → Postgres и фоновый исполнитель
internal/httpapi    роутер, middleware, auth, хендлеры и DTO ответов
```

Зависимость строго в одну сторону: `httpapi` знает про `syncer` и `storage`,
но не про `riot`; форма ответов Riot дальше пакета `riot` не протекает.
JSON-теги живут в `httpapi/dto.go`, а не на доменных структурах — формат API
не должен диктоваться формой таблиц.

### Синхронизация

`syncer.Service` синхронизирует одного саммонера и ничего не знает про HTTP —
его дёргают и хендлер добавления, и фоновый исполнитель:

- match id, уже лежащие в базе, отфильтровываются до похода в Riot: детали матча
  весят под 140 КБ, повторно тянуть их незачем;
- участники матча пишутся только для отслеживаемых саммонеров — на
  `match_participants` стоит FK на `summoners`;
- несохранившийся матч логируется и не роняет остальной прогон; протухший ключ
  и отменённый контекст прерывают прогон сразу, потому что продолжать бессмысленно.

`syncer.Runner` — очередь фиксированного размера (64) и три воркера. `Enqueue`
никогда не блокирует вызывающего: переполненная очередь означает WARN и отброшенную
задачу, а не ожидание в HTTP-хендлере. При остановке сначала закрывается HTTP
(чтобы не появлялись новые задачи), затем дожидаются активные синхронизации —
и только потом закрывается пул БД.

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
Хендлеры тестируются на фейковых репозиториях, поэтому `go test ./...` зелёный
без всякой инфраструктуры.

Исключение — тесты репозиториев: им нужен живой Postgres, поэтому они смотрят
на `TEST_DATABASE_URL` и без него пропускаются.

```bash
docker run --rm -d --name lc-test-pg -p 55432:5432 \
  -e POSTGRES_USER=league -e POSTGRES_PASSWORD=league -e POSTGRES_DB=league_test postgres:16

TEST_DATABASE_URL='postgres://league:league@localhost:55432/league_test?sslmode=disable' \
  go test ./internal/storage/ -v
```

Миграции они применяют сами и убирают за собой данные между тестами.
