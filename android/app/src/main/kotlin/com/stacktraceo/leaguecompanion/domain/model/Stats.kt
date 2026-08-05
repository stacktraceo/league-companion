package com.stacktraceo.leaguecompanion.domain.model

import java.time.Instant

data class Stats(
    val periodDays: Int,
    val since: Instant,
    val games: Int,
    val wins: Int,
    val losses: Int,
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
