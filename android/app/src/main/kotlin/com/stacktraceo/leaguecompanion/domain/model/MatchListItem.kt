package com.stacktraceo.leaguecompanion.domain.model

import java.time.Duration
import java.time.Instant

data class MatchListItem(
    val matchId: String,
    val playedAt: Instant,
    val duration: Duration,
    val queueId: Int,
    val championName: String,
    val kills: Int,
    val deaths: Int,
    val assists: Int,
    val kda: Double,
    val win: Boolean,
    val cs: Int,
    val goldEarned: Int,
)
