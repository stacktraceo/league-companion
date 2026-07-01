-- Схема из SPEC.md 3.3 с отклонениями из CLAUDE.md.

-- Отслеживаемые саммонеры.
CREATE TABLE summoners (
    puuid           TEXT PRIMARY KEY,
    riot_id         TEXT NOT NULL,           -- GameName из Riot ID
    tag_line        TEXT NOT NULL,           -- TagLine из Riot ID
    region          TEXT NOT NULL,           -- platform routing: ru, euw1, ...
    summoner_level  INT,
    profile_icon_id INT,
    last_synced_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Ранговая статистика: снапшот на момент последней синхронизации.
CREATE TABLE ranked_stats (
    puuid         TEXT NOT NULL REFERENCES summoners (puuid) ON DELETE CASCADE,
    queue_type    TEXT NOT NULL,             -- RANKED_SOLO_5x5, RANKED_FLEX_SR, ...
    tier          TEXT,
    rank          TEXT,
    league_points INT,
    wins          INT,
    losses        INT,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (puuid, queue_type)
);

-- Матчи. Общие для всех участников, не дублируются между саммонерами.
--
-- raw_data (CLAUDE.md, отклонение 1) — полный JSON ответа Match-V5, пишется при
-- первой синхронизации. Позволяет расширять набор полей (предметы, руны, спеллы)
-- без повторных запросов к Riot и отдавать всех 10 участников в
-- GET /api/v1/matches/{matchId}, хотя match_participants хранит только
-- отслеживаемых саммонеров.
CREATE TABLE matches (
    match_id      TEXT PRIMARY KEY,
    game_creation TIMESTAMPTZ NOT NULL,
    game_duration INT NOT NULL,              -- секунды
    queue_id      INT NOT NULL,
    game_version  TEXT NOT NULL,
    raw_data      JSONB NOT NULL
);

-- Участие конкретного отслеживаемого саммонера в матче.
-- FK на summoners сознательный: строки заводятся только для тех, кого мы трекаем.
CREATE TABLE match_participants (
    match_id      TEXT NOT NULL REFERENCES matches (match_id) ON DELETE CASCADE,
    puuid         TEXT NOT NULL REFERENCES summoners (puuid) ON DELETE CASCADE,
    champion_name TEXT NOT NULL,
    kills         INT NOT NULL,
    deaths        INT NOT NULL,
    assists       INT NOT NULL,
    win           BOOLEAN NOT NULL,
    cs            INT NOT NULL,
    gold_earned   INT NOT NULL,
    PRIMARY KEY (match_id, puuid)
);

CREATE INDEX idx_match_participants_puuid ON match_participants (puuid);
CREATE INDEX idx_matches_game_creation ON matches (game_creation);
