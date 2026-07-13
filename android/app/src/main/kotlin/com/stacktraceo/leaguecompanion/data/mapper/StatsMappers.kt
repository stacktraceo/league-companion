package com.stacktraceo.leaguecompanion.data.mapper

import com.stacktraceo.leaguecompanion.data.remote.dto.ChampionStatsDto
import com.stacktraceo.leaguecompanion.data.remote.dto.StatsDto
import com.stacktraceo.leaguecompanion.domain.model.ChampionStats
import com.stacktraceo.leaguecompanion.domain.model.Stats

fun StatsDto.toDomain(): Stats =
    Stats(
        periodDays = periodDays,
        since = since,
        games = games,
        wins = wins,
        losses = losses,
        winRate = winRate,
        kills = kills,
        deaths = deaths,
        assists = assists,
        kda = kda,
        topChampions = topChampions.map { it.toDomain() },
    )

fun ChampionStatsDto.toDomain(): ChampionStats =
    ChampionStats(
        championName = championName,
        games = games,
        wins = wins,
        winRate = winRate,
        kda = kda,
    )
