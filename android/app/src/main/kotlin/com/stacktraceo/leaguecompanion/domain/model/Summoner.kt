package com.stacktraceo.leaguecompanion.domain.model

import java.time.Instant

data class Summoner(
    val puuid: String,
    val riotId: String,
    val tagLine: String,
    val region: String,
    val level: Int,
    val profileIconId: Int,
    val lastSyncedAt: Instant?,
    val ranked: List<RankedStat>,
) {
    val displayName: String get() = "$riotId#$tagLine"
}
