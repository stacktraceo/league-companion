package com.stacktraceo.leaguecompanion.domain.model

import java.time.Instant

/**
 * Агрегация за период (SPEC.md 3.4). Считается на бэкенде и не пересчитывается
 * здесь: своя формула была бы третьей копией правила «при нуле смертей KDA = K+A»
 * и рано или поздно разошлась бы с лентой и с `/stats`.
 */
data class Stats(
    val periodDays: Int,
    val since: Instant,
    val games: Int,
    val wins: Int,
    val losses: Int,
    /** Доля 0..1, как и везде в проекте. */
    val winRate: Double,
    val kills: Int,
    val deaths: Int,
    val assists: Int,
    val kda: Double,
    val topChampions: List<ChampionStats>,
) {
    val isEmpty: Boolean get() = games == 0
}

data class ChampionStats(
    val championName: String,
    val games: Int,
    val wins: Int,
    val winRate: Double,
    val kda: Double,
)
