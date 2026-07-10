package com.stacktraceo.leaguecompanion.domain.model

/** Ранг в одной очереди на момент последней синхронизации. */
data class RankedStat(
    val queueType: String,
    val tier: String,
    val rank: String,
    val leaguePoints: Int,
    val wins: Int,
    val losses: Int,
) {
    val games: Int get() = wins + losses

    /** Доля 0..1, как и `winRate` бэкенда, — в проценты переводит форматтер экрана. */
    val winRate: Double get() = if (games == 0) 0.0 else wins.toDouble() / games
}
