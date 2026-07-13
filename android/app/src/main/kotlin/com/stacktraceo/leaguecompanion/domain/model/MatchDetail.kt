package com.stacktraceo.leaguecompanion.domain.model

import java.time.Duration
import java.time.Instant

/** Полные детали матча: обе команды и все десять участников (SPEC.md 4.2, пункт 4). */
data class MatchDetail(
    val matchId: String,
    val playedAt: Instant,
    val duration: Duration,
    val queueId: Int,
    val gameMode: String,
    val teams: List<TeamSide>,
)

data class TeamSide(
    val teamId: Int,
    val win: Boolean,
    /** Счёт команды — `objectives.champion.kills` из ответа Riot. */
    val kills: Int,
    val towers: Int,
    val dragons: Int,
    val barons: Int,
    val players: List<MatchPlayer>,
)

data class MatchPlayer(
    val puuid: String,
    val riotId: String,
    val tagLine: String,
    val championName: String,
    val level: Int,
    val position: String,
    val kills: Int,
    val deaths: Int,
    val assists: Int,
    val cs: Int,
    val gold: Int,
    val damage: Int,
    val visionScore: Int,
    /** Тот, чей профиль открыт: на экране из десяти строк иначе не найти себя. */
    val tracked: Boolean,
) {
    val displayName: String get() = if (riotId.isEmpty()) championName else "$riotId#$tagLine"
}
