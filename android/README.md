# League Companion — Android

Клиент к бэкенду из `../backend`. Kotlin, Jetpack Compose, MVVM, Room, Retrofit, Hilt.

## Настройка перед первым запуском

`local.properties` не коммитится и создаётся вручную (Android Studio сама пропишет
туда `sdk.dir` при открытии проекта):

```properties
sdk.dir=C\:\\Users\\<имя>\\AppData\\Local\\Android\\Sdk
LC_BASE_URL=http://10.0.2.2:8080/
LC_API_KEY=<то же значение, что CLIENT_API_KEY в корневом .env>
```

Оба значения уезжают в `BuildConfig` (см. `app/build.gradle.kts`) — в коде их нет и
быть не должно. Если `local.properties` отсутствует, значения берутся из переменных
окружения с теми же именами, иначе — из дефолтов; так собирается CI.

`10.0.2.2` — адрес хост-машины **изнутри эмулятора**. `localhost` там означает сам
эмулятор, и запрос не дойдёт. Для устройства по USB нужен IP машины в локальной сети
и `adb reverse tcp:8080 tcp:8080`.

`LC_API_KEY` обязан совпадать с `CLIENT_API_KEY` бэкенда: несовпадение даёт `401`, и
экран честно скажет, что дело в ключе приложения, а не в саммонере.

## Сборка и проверки

```bash
./gradlew :app:assembleDebug
./gradlew :app:ktlintCheck      # ktlintFormat — автоисправление
./gradlew :app:testDebugUnitTest
./gradlew :app:lintDebug
```

Те же четыре команды гоняет CI (job `android` в `.github/workflows/ci.yml`).

### Если юнит-тесты падают с `Could not find or load main class`

Gradle подставляет `PATH` в `-Djava.library.path` тест-воркера. Одна запись `PATH` с
незакрытой двойной кавычкой ломает разбор командной строки Windows, и воркер падает
ещё до JUnit — при том что `assembleDebug` проходит. Лечится чисткой самой
переменной среды; разово помогает `set "PATH=%PATH:"=%"` перед вызовом `gradlew.bat`.

## Что где лежит

```
data/remote      Retrofit, DTO (зеркала backend/internal/httpapi/dto.go), разбор ошибок
data/local       Room: сущности — зеркала таблиц бэкенда, DAO, конвертеры
data/mapper      сеть → база → домен
data/repository  offline-first: чтение из Room, запись из сети
domain/model     модели для UI
debug            времянка для проверки связки живьём, уедет вместе с настоящими экранами
```

Правило одно: **UI читает только из Room**, сеть пишет в Room. Поэтому офлайн
получается по построению, а не веткой `if` в экране, и провал обновления не стирает
уже показанное.

## Живая проверка

1. В корне репозитория: `docker compose up -d` (нужны `RIOT_API_KEY` и `CLIENT_API_KEY`
   в `.env`).
2. Запустить приложение на эмуляторе, ввести Riot ID, тег и регион → **Track**.
3. Профиль появляется сразу, `Last sync: never`, лента пуста — это ожидаемое
   состояние: `POST /summoners` отвечает `201` до синхронизации. Через несколько
   секунд **Sync** → **Matches**.
4. Офлайн: выключить сеть у эмулятора и перезапустить приложение — профиль и лента
   остаются на месте, они читаются из Room.
