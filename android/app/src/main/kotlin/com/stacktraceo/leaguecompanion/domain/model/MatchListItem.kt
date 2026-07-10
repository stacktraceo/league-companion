package com.stacktraceo.leaguecompanion.domain.model

import java.time.Duration
import java.time.Instant

/**
 * Строка ленты матчей.
 *
 * `kda` не вычисляется здесь: значение приходит с бэкенда, где оно считается по
 * правилу «при нуле смертей — K+A» (domain.MatchParticipant.KDA). Своя формула на
 * клиенте разошлась бы с экраном статистики на первом же матче без смертей.
 */
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
