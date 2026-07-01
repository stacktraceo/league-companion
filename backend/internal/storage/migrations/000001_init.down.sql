-- Порядок обратный зависимостям внешних ключей.
DROP INDEX IF EXISTS idx_matches_game_creation;
DROP INDEX IF EXISTS idx_match_participants_puuid;

DROP TABLE IF EXISTS match_participants;
DROP TABLE IF EXISTS matches;
DROP TABLE IF EXISTS ranked_stats;
DROP TABLE IF EXISTS summoners;
