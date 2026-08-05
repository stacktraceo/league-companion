package com.stacktraceo.leaguecompanion.data.remote.dto

import kotlinx.serialization.Serializable

/**
 * Подмножество ответа Match-V5.
 *
 * Единственное место, где клиент разбирает схему **Riot**, а не зеркало
 * `backend/internal/httpapi/dto.go`: `GET /api/v1/matches/{matchId}` отдаёт
 * `matches.raw_data` без пересборки (DECISIONS.md, отклонение 1).
 *
 * У участника в реальном ответе 155 полей — объявлены те, что показывает экран;
 * остальные молча отбрасывает `ignoreUnknownKeys` из `NetworkModule`. Добавить
 * предметы или руны потом можно, не трогая ни бэкенд, ни кэш: они уже приезжают.
 */
@Serializable
data class MatchDetailDto(
    val metadata: MatchMetadataDto,
    val info: MatchInfoDto,
)

@Serializable
data class MatchMetadataDto(
    val matchId: String,
)

@Serializable
data class MatchInfoDto(
    /** Epoch millis. */
    val gameCreation: Long,
    /**
     * Секунды у матчей с патча 11.20 и позже, миллисекунды у более старых.
     * Отличаются по наличию [gameEndTimestamp] — см. `toDomain`.
     */
    val gameDuration: Long,
    val gameEndTimestamp: Long? = null,
    val gameMode: String = "",
    val queueId: Int = 0,
    val gameVersion: String = "",
    val teams: List<TeamDto> = emptyList(),
    val participants: List<MatchParticipantDto> = emptyList(),
)

@Serializable
data class TeamDto(
    val teamId: Int,
    val win: Boolean = false,
    val objectives: ObjectivesDto = ObjectivesDto(),
)

@Serializable
data class ObjectivesDto(
    val champion: ObjectiveDto = ObjectiveDto(),
    val tower: ObjectiveDto = ObjectiveDto(),
    val dragon: ObjectiveDto = ObjectiveDto(),
    val baron: ObjectiveDto = ObjectiveDto(),
    val inhibitor: ObjectiveDto = ObjectiveDto(),
    val riftHerald: ObjectiveDto = ObjectiveDto(),
)

@Serializable
data class ObjectiveDto(
    val first: Boolean = false,
    val kills: Int = 0,
)

@Serializable
data class MatchParticipantDto(
    val puuid: String = "",
    // Riot ID участника есть прямо в матче — отдельный запрос за именами не нужен.
    val riotIdGameName: String = "",
    val riotIdTagline: String = "",
    val championName: String = "",
    val champLevel: Int = 0,
    val teamId: Int = 0,
    /** TOP, JUNGLE, MIDDLE, BOTTOM, UTILITY; пусто в ARAM и у старых матчей. */
    val teamPosition: String = "",
    val win: Boolean = false,
    val kills: Int = 0,
    val deaths: Int = 0,
    val assists: Int = 0,
    val totalMinionsKilled: Int = 0,
    val neutralMinionsKilled: Int = 0,
    val goldEarned: Int = 0,
    val totalDamageDealtToChampions: Int = 0,
    val visionScore: Int = 0,
)
