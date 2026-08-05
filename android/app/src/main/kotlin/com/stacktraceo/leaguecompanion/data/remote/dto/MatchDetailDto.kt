package com.stacktraceo.leaguecompanion.data.remote.dto

import kotlinx.serialization.Serializable

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
    val gameCreation: Long,
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
    // Riot ID участника есть прямо в матче - отдельный запрос за именами не нужен.
    val riotIdGameName: String = "",
    val riotIdTagline: String = "",
    val championName: String = "",
    val champLevel: Int = 0,
    val teamId: Int = 0,
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
