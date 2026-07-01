# Техническое задание: League Companion

Личный трекер статистики League of Legends. Go-бэкенд, интегрирующийся с Riot Games API, + Android-клиент на Kotlin/Jetpack Compose.

## 1. Общее описание

### 1.1 Проблема

Клиент League of Legends не даёт удобной аналитики по истории матчей: нет трендов винрейта, статистики по чемпионам за период, истории KDA. League Companion закрывает это: отслеживает выбранных саммонеров, синхронизирует их матчи в фоне и отдаёт агрегированную статистику через API и мобильное приложение.

### 1.2 Пользовательский сценарий

1. Пользователь вводит Riot ID (GameName#TagLine) и регион в приложении.
2. Приложение отправляет запрос на бэкенд → бэкенд резолвит PUUID, подтягивает профиль, ранг и последние матчи через Riot API.
3. Саммонер добавляется в список отслеживаемых — бэкенд фоново синхронизирует его новые матчи.
4. Пользователь видит: профиль (уровень, ранг, LP), ленту матчей (победа/поражение, KDA, чемпион), агрегированную статистику (винрейт, топ чемпионы) — при этом данные кэшированы локально и доступны офлайн.

### 1.3 Термины

| Термин | Значение |
|---|---|
| PUUID | Уникальный идентификатор игрока в системах Riot |
| Riot ID | Игровое имя вида GameName#TagLine |
| Summoner | Игровой аккаунт на конкретном регионе |
| Match-V5 | Версия Riot API для истории матчей |
| Regional routing | Riot использует разные базовые URL для account/match (europe/americas/asia) и для summoner/league (ru/euw1/eun1 и т.д.) |

## 2. Архитектура системы

```
┌────────────────────┐        ┌─────────────────────────────────┐        ┌────────────────┐
│    Android App      │ HTTPS  │            Go Backend            │ HTTPS  │   Riot Games    │
│  (Compose / MVVM)   │───────▶│  ┌────────────┐  ┌─────────────┐ │───────▶│      API        │
│                      │◀───────│  │  REST API  │  │ Rate Limiter│ │◀───────│                 │
│  Room (офлайн-кэш)   │        │  │ (chi/gin)  │  │             │ │        └─────────────────┘
└──────────────────────┘        │  └─────┬──────┘  └──────┬──────┘ │
                                 │  ┌─────▼─────────────────▼─────┐ │
                                 │  │   Sync Worker (goroutine)    │ │
                                 │  │   тикер, фоновая синхро      │ │
                                 │  └─────┬─────────────────┬─────┘ │
                                 │  ┌─────▼──────┐   ┌──────▼─────┐ │
                                 │  │ PostgreSQL │   │   Cache    │ │
                                 │  │            │   │(Redis/in-  │ │
                                 │  │            │   │ memory)    │ │
                                 │  └────────────┘   └────────────┘ │
                                 └───────────────────────────────────┘
```

## 3. Backend (Go)

### 3.1 Технологический стек

| Компонент | Технология |
|---|---|
| Язык | Go 1.22+ |
| HTTP-роутер | chi или gin |
| БД | PostgreSQL 16 |
| Миграции | golang-migrate |
| Кэш | Redis (или sync.Map + TTL, если хочется без внешней зависимости) |
| Rate limiting | golang.org/x/time/rate (два лимитера: per-second и per-2-min) |
| Конфигурация | env-переменные (envconfig или ручной парсинг) |
| Логирование | log/slog (структурные логи) |
| HTTP-клиент к Riot | net/http + retry/backoff обёртка |
| Тесты | стандартный testing, testify для ассертов |
| Контейнеризация | Docker + docker-compose (backend + postgres + redis) |

### 3.2 Интеграция с Riot API

Region routing — важно не перепутать два вида роутинга:

- **Platform routing** (ru, euw1, eun1, na1...) — для Summoner-V4, League-V4
- **Regional routing** (europe, americas, asia) — для Account-V1, Match-V5

Используемые эндпоинты Riot API:

| Эндпоинт | Назначение |
|---|---|
| `GET /riot/account/v1/accounts/by-riot-id/{gameName}/{tagLine}` | Riot ID → PUUID |
| `GET /lol/summoner/v4/summoners/by-puuid/{puuid}` | Профиль: уровень, иконка |
| `GET /lol/league/v4/entries/by-puuid/{puuid}` | Ранг, LP, W/L |
| `GET /lol/match/v5/matches/by-puuid/{puuid}/ids?start=0&count=20` | Список ID матчей |
| `GET /lol/match/v5/matches/{matchId}` | Детали матча (все участники) |

**Rate limiting.** Personal Development Key: 20 запросов/сек, 100 запросов/2 минуты — оба лимита действуют одновременно и глобально (на весь бэкенд, не на пользователя). Реализация: composite limiter — обёртка над двумя `rate.Limiter`, перед каждым запросом к Riot берётся токен у обоих, дожидаясь более строгого.

**Обработка 429.** Riot возвращает заголовок `Retry-After` — клиент обязан его уважать и делать backoff, а не долбить дальше.

**Важное ограничение:** Development API key протухает каждые 24 часа и перевыпускается вручную на developer.riotgames.com — заложить это в конфиг как переменную окружения, которую легко обновлять без пересборки.

### 3.3 Схема БД

```sql
-- отслеживаемые саммонеры
CREATE TABLE summoners (
    puuid            TEXT PRIMARY KEY,
    riot_id          TEXT NOT NULL,
    tag_line         TEXT NOT NULL,
    region           TEXT NOT NULL,
    summoner_level   INT,
    profile_icon_id  INT,
    last_synced_at   TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ранговая статистика (снапшот на момент последней синхронизации)
CREATE TABLE ranked_stats (
    puuid        TEXT NOT NULL REFERENCES summoners(puuid),
    queue_type   TEXT NOT NULL,   -- RANKED_SOLO_5x5 и т.п.
    tier         TEXT,
    rank         TEXT,
    league_points INT,
    wins         INT,
    losses       INT,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (puuid, queue_type)
);

-- матчи (общие для всех участников, не дублируются)
CREATE TABLE matches (
    match_id       TEXT PRIMARY KEY,
    game_creation  TIMESTAMPTZ NOT NULL,
    game_duration  INT NOT NULL,   -- секунды
    queue_id       INT NOT NULL,
    game_version   TEXT NOT NULL
);

-- участие конкретного саммонера в матче
CREATE TABLE match_participants (
    match_id      TEXT NOT NULL REFERENCES matches(match_id),
    puuid         TEXT NOT NULL REFERENCES summoners(puuid),
    champion_name TEXT NOT NULL,
    kills         INT NOT NULL,
    deaths        INT NOT NULL,
    assists       INT NOT NULL,
    win           BOOLEAN NOT NULL,
    cs            INT NOT NULL,
    gold_earned   INT NOT NULL,
    PRIMARY KEY (match_id, puuid)
);

CREATE INDEX idx_match_participants_puuid ON match_participants(puuid);
CREATE INDEX idx_matches_game_creation ON matches(game_creation);
```

### 3.4 REST API

| Метод | Путь | Описание |
|---|---|---|
| POST | `/api/v1/summoners` | Body: `{riotId, tagLine, region}`. Резолвит PUUID, создаёт запись, ставит в очередь на первую синхронизацию |
| GET | `/api/v1/summoners/{puuid}` | Профиль + текущий ранг |
| GET | `/api/v1/summoners/{puuid}/matches?limit=20&offset=0` | Список матчей с пагинацией |
| GET | `/api/v1/summoners/{puuid}/stats?period=30d` | Агрегация: винрейт, средний KDA, топ-5 чемпионов по количеству игр |
| GET | `/api/v1/matches/{matchId}` | Полные детали матча — все участники, обе команды |
| POST | `/api/v1/summoners/{puuid}/sync` | Принудительная синхронизация (сама тоже под rate limit, не чаще раза в N минут) |

Формат ошибок — единый JSON: `{"error": "summoner_not_found", "message": "..."}`, коды: 404 (не найден), 429 (не удалось — Riot API лимит), 502 (Riot API недоступен, отдаём последние закэшированные данные с флагом `"stale": true`, если они есть).

### 3.5 Фоновая синхронизация

Отдельная горутина с `time.Ticker` (например, раз в 10 минут):

1. Забирает список отслеживаемых puuid из `summoners`.
2. Для каждого — через worker pool (ограниченное число одновременных горутин, например 3) запрашивает новые матчи через Match-V5, сравнивая с уже сохранёнными `match_id`.
3. Новые матчи и участников сохраняет в БД.
4. Обновляет `ranked_stats` и `last_synced_at`.

Важно: воркер тоже обязан идти через общий rate limiter — он делит лимит Riot API с обработчиками входящих HTTP-запросов.

### 3.6 Нефункциональные требования

- Ответ для закэшированных данных — до 200 мс.
- Все обращения к Riot API — через единый клиент с ретраями (экспоненциальный backoff на 429/503).
- Конфиг через переменные окружения: `RIOT_API_KEY`, `DATABASE_URL`, `REDIS_ADDR`, `HTTP_PORT`.
- Структурные логи (slog) с уровнями, request-id на каждый входящий запрос.
- Юнит-тесты минимум на: rate limiter, агрегацию статистики, маппинг Riot DTO → доменные модели.
- `docker-compose up` поднимает backend + Postgres + Redis одной командой.

## 4. Android

### 4.1 Технологический стек

| Компонент | Технология |
|---|---|
| Язык | Kotlin |
| UI | Jetpack Compose |
| Архитектура | MVVM |
| Состояние | StateFlow / collectAsState |
| Сеть | Retrofit + OkHttp (logging interceptor) |
| Локальное хранилище | Room |
| DI | Hilt (или ручной, если хочется проще для MVP) |
| Асинхронность | Kotlin Coroutines |

### 4.2 Экраны

1. **Поиск** — поле ввода GameName#TagLine, выбор региона (dropdown), кнопка «Найти» → вызывает POST /summoners.
2. **Профиль** — иконка, уровень, ранг (тир + LP), общий W/L.
3. **История матчей** — LazyColumn, карточка матча: результат (цвет — зелёный/красный), иконка чемпиона, KDA, CS, время игры, «N минут/часов назад».
4. **Детали матча** — обе команды, все участники, статы каждого (по тапу на карточку из списка).
5. **Статистика** (опционально, если останется время) — винрейт за период, топ чемпионы (простой bar chart).

### 4.3 Слои приложения

```
UI (Compose) → ViewModel (StateFlow) → Repository → { Retrofit (network), Room (cache) }
```

Repository реализует offline-first: сначала отдаёт закэшированные данные из Room, затем в фоне обновляет из сети и обновляет UI при получении свежих данных.

Room-сущности — зеркалят подмножество бэкенд-схемы, нужное для UI: `SummonerEntity`, `MatchEntity`, `MatchParticipantEntity`.

### 4.4 Нефункциональные требования

- Приложение открывается и показывает последние закэшированные данные даже без сети.
- Явные состояния загрузки/ошибки/пустого списка на каждом экране (не просто спиннер на весь экран).
- Обработка ошибок сети — понятные сообщения пользователю, а не сырой стектрейс.

## 5. План разработки (2 недели / ~14 вечеров)

| Дни | Задачи |
|---|---|
| 1–2 | Скелет Go-проекта, конфиг, миграции БД, клиент к Riot API (Account/Summoner/League/Match), ручная проверка через curl |
| 3–4 | Rate limiter + кэш, интеграция в клиент |
| 5–6 | REST-хендлеры: добавление саммонера, список матчей с пагинацией |
| 7 | Фоновый sync worker, эндпоинт статистики |
| 8 | Docker/docker-compose, README, базовые тесты |
| 9–10 | Скелет Android-проекта, сетевой слой (Retrofit), Room, Repository |
| 11–12 | Compose-экраны: поиск, профиль, список матчей |
| 13 | Экран деталей матча, (опционально) статистика |
| 14 | Полировка, состояния ошибок/загрузки, сквозное тестирование, README |

## 6. Definition of Done (MVP)

- [ ] Backend поднимается через `docker-compose up`, эндпоинты работают и покрыты rate-limit'ом
- [ ] Можно добавить саммонера по Riot ID и получить его профиль + последние матчи
- [ ] Фоновая синхронизация реально работает — новые матчи подтягиваются без ручного вызова /sync
- [ ] Android-приложение подключается к бэкенду, показывает профиль и ленту матчей
- [ ] Данные видны офлайн (из Room) после первой загрузки
- [ ] Есть README у обоих репозиториев: как поднять, какие переменные окружения нужны
- [ ] Юнит-тесты на rate limiter и агрегацию статистики зелёные

## 7. Риски и ограничения

- Протухание dev-ключа Riot каждые 24 часа — потребует ручного обновления во время разработки, заложить это как известное ограничение MVP.
- Region routing — легко перепутать platform vs regional роутинг, стоит вынести в отдельную функцию маппинга с юнит-тестом.
- Скоуп-крип — детали матча (предметы, руны, спеллы) можно легко утянуть в бесконечную детализацию; для MVP ограничиться базовыми полями (KDA, CS, чемпион, золото).
- Riot API ToS — использование строго личное/некоммерческое под Development Key, не для публичного продакшена.
