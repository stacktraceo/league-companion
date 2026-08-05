package com.stacktraceo.leaguecompanion.domain.model

data class RankedStat(
    val queueType: String,
    val tier: String,
    val rank: String,
    val leaguePoints: Int,
    val wins: Int,
    val losses: Int,
) {
    val games: Int get() = wins + losses

    val winRate: Double get() = if (games == 0) 0.0 else wins.toDouble() / games
}
