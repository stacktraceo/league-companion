package com.stacktraceo.leaguecompanion.data.mapper

import com.stacktraceo.leaguecompanion.data.local.entity.MatchEntity
import com.stacktraceo.leaguecompanion.data.local.entity.MatchParticipantEntity
import com.stacktraceo.leaguecompanion.data.local.view.MatchListItemView
import com.stacktraceo.leaguecompanion.data.remote.dto.MatchListItemDto
import com.stacktraceo.leaguecompanion.domain.model.MatchListItem
import java.time.Duration

fun MatchListItemDto.toMatchEntity(): MatchEntity =
    MatchEntity(
        matchId = matchId,
        gameCreation = gameCreation,
        gameDurationSeconds = gameDurationSeconds,
        queueId = queueId,
        gameVersion = gameVersion,
    )

fun MatchListItemDto.toParticipantEntity(puuid: String): MatchParticipantEntity =
    MatchParticipantEntity(
        matchId = matchId,
        puuid = puuid,
        championName = championName,
        kills = kills,
        deaths = deaths,
        assists = assists,
        win = win,
        cs = cs,
        goldEarned = goldEarned,
        kda = kda,
    )

fun MatchListItemView.toDomain(): MatchListItem =
    MatchListItem(
        matchId = matchId,
        playedAt = gameCreation,
        // Бэкенд отдаёт секунды (game_duration), Duration избавляет экран от
        // самодельного деления на 60 и от вопроса, что там за число.
        duration = Duration.ofSeconds(gameDurationSeconds.toLong()),
        queueId = queueId,
        championName = championName,
        kills = kills,
        deaths = deaths,
        assists = assists,
        kda = kda,
        win = win,
        cs = cs,
        goldEarned = goldEarned,
    )
