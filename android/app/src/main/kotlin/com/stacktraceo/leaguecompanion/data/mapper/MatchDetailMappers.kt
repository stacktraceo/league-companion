package com.stacktraceo.leaguecompanion.data.mapper

import com.stacktraceo.leaguecompanion.data.remote.dto.MatchDetailDto
import com.stacktraceo.leaguecompanion.data.remote.dto.MatchParticipantDto
import com.stacktraceo.leaguecompanion.data.remote.dto.TeamDto
import com.stacktraceo.leaguecompanion.domain.model.MatchDetail
import com.stacktraceo.leaguecompanion.domain.model.MatchPlayer
import com.stacktraceo.leaguecompanion.domain.model.TeamSide
import java.time.Duration
import java.time.Instant

/** Порядок ролей на экране. Неизвестная позиция уходит в конец. */
private val ROLE_ORDER = listOf("TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY")

fun MatchDetailDto.toDomain(trackedPuuid: String): MatchDetail =
    MatchDetail(
        matchId = metadata.matchId,
        playedAt = Instant.ofEpochMilli(info.gameCreation),
        duration = gameDuration(),
        queueId = info.queueId,
        gameMode = info.gameMode,
        teams =
            info.teams.map { team ->
                team.toDomain(
                    players = info.participants.filter { it.teamId == team.teamId },
                    trackedPuuid = trackedPuuid,
                )
            },
    )

/**
 * До патча 11.20 Riot отдавал длительность в миллисекундах, позже — в секундах;
 * отличить можно только по наличию `gameEndTimestamp`.
 *
 * Бэкенд это уже умеет (`domain.MatchFromRiot`), но сырой ответ Match-V5 проходит
 * мимо той нормализации: `GET /matches/{matchId}` отдаёт `raw_data` как есть.
 * Без повторения здесь старый матч показал бы длительность в тысячу раз больше.
 */
private fun MatchDetailDto.gameDuration(): Duration =
    if (info.gameEndTimestamp == null) {
        Duration.ofMillis(info.gameDuration)
    } else {
        Duration.ofSeconds(info.gameDuration)
    }

private fun TeamDto.toDomain(
    players: List<MatchParticipantDto>,
    trackedPuuid: String,
): TeamSide =
    TeamSide(
        teamId = teamId,
        win = win,
        kills = objectives.champion.kills,
        towers = objectives.tower.kills,
        dragons = objectives.dragon.kills,
        barons = objectives.baron.kills,
        players =
            players
                .sortedBy { participant ->
                    // indexOf вернёт -1 для пустой или незнакомой позиции — сдвигаем
                    // её в конец, сохраняя исходный порядок таких строк (sortedBy
                    // устойчив).
                    ROLE_ORDER.indexOf(participant.teamPosition).takeIf { it >= 0 } ?: ROLE_ORDER.size
                }.map { it.toDomain(trackedPuuid) },
    )

private fun MatchParticipantDto.toDomain(trackedPuuid: String): MatchPlayer =
    MatchPlayer(
        puuid = puuid,
        riotId = riotIdGameName,
        tagLine = riotIdTagline,
        championName = championName,
        level = champLevel,
        position = teamPosition,
        kills = kills,
        deaths = deaths,
        assists = assists,
        // CS — это миньоны плюс лесные монстры; по отдельности ни одно из чисел
        // не совпадает с тем, что показывает клиент игры.
        cs = totalMinionsKilled + neutralMinionsKilled,
        gold = goldEarned,
        damage = totalDamageDealtToChampions,
        visionScore = visionScore,
        tracked = puuid == trackedPuuid,
    )
