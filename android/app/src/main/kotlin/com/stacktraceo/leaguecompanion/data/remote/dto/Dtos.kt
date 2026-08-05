package com.stacktraceo.leaguecompanion.data.remote.dto

import com.stacktraceo.leaguecompanion.data.remote.InstantSerializer
import kotlinx.serialization.Serializable
import java.time.Instant

// Зеркала backend/internal/httpapi/dto.go. Имена полей совпадают с JSON-тегами,
// поэтому @SerialName нигде не нужен - расхождение сразу видно глазами.

@Serializable
data class CreateSummonerRequest(
    val riotId: String,
    val tagLine: String,
    val region: String,
)

@Serializable
data class SummonerDto(
    val puuid: String,
    val riotId: String,
    val tagLine: String,
    val region: String,
    val summonerLevel: Int,
    val profileIconId: Int,
    // null, пока фоновая синхронизация не отработала ни разу.
    @Serializable(with = InstantSerializer::class)
    val lastSyncedAt: Instant? = null,
    @Serializable(with = InstantSerializer::class)
    val createdAt: Instant,
    val ranked: List<RankedDto> = emptyList(),
    // Бэкенд ставит поле только когда оно true (omitempty).
    val stale: Boolean = false,
)

@Serializable
data class RankedDto(
    val queueType: String,
    val tier: String,
    val rank: String,
    val leaguePoints: Int,
    val wins: Int,
    val losses: Int,
    @Serializable(with = InstantSerializer::class)
    val updatedAt: Instant,
)

@Serializable
data class MatchListDto(
    val items: List<MatchListItemDto> = emptyList(),
    val limit: Int,
    val offset: Int,
    val total: Int,
)

@Serializable
data class MatchListItemDto(
    val matchId: String,
    @Serializable(with = InstantSerializer::class)
    val gameCreation: Instant,
    val gameDurationSeconds: Int,
    val queueId: Int,
    val gameVersion: String,
    val championName: String,
    val kills: Int,
    val deaths: Int,
    val assists: Int,
    val kda: Double,
    val win: Boolean,
    val cs: Int,
    val goldEarned: Int,
)

@Serializable
data class StatsDto(
    val periodDays: Int,
    @Serializable(with = InstantSerializer::class)
    val since: Instant,
    val games: Int,
    val wins: Int,
    val losses: Int,
    // Доля 0..1, не проценты - форматирует клиент (журнал решений, «День 7»).
    val winRate: Double,
    val kills: Int,
    val deaths: Int,
    val assists: Int,
    val kda: Double,
    val topChampions: List<ChampionStatsDto> = emptyList(),
)

@Serializable
data class ChampionStatsDto(
    val championName: String,
    val games: Int,
    val wins: Int,
    val winRate: Double,
    val kda: Double,
)

@Serializable
data class SyncAcceptedDto(
    val status: String,
    val puuid: String,
    @Serializable(with = InstantSerializer::class)
    val lastSyncedAt: Instant? = null,
)

@Serializable
data class ApiErrorDto(
    val error: String = "",
    val message: String = "",
)
