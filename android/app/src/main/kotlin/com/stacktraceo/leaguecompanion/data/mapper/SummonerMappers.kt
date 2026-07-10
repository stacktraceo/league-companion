package com.stacktraceo.leaguecompanion.data.mapper

import com.stacktraceo.leaguecompanion.data.local.entity.RankedStatEntity
import com.stacktraceo.leaguecompanion.data.local.entity.SummonerEntity
import com.stacktraceo.leaguecompanion.data.remote.dto.SummonerDto
import com.stacktraceo.leaguecompanion.domain.model.RankedStat
import com.stacktraceo.leaguecompanion.domain.model.Summoner

// Сеть → база → домен. Обратного пути нет: наружу клиент отдаёт только Riot ID,
// тег и регион, и для этого хватает CreateSummonerRequest.

fun SummonerDto.toEntity(): SummonerEntity =
    SummonerEntity(
        puuid = puuid,
        riotId = riotId,
        tagLine = tagLine,
        region = region,
        summonerLevel = summonerLevel,
        profileIconId = profileIconId,
        lastSyncedAt = lastSyncedAt,
        createdAt = createdAt,
    )

fun SummonerDto.toRankedEntities(): List<RankedStatEntity> =
    ranked.map { dto ->
        RankedStatEntity(
            puuid = puuid,
            queueType = dto.queueType,
            tier = dto.tier,
            rank = dto.rank,
            leaguePoints = dto.leaguePoints,
            wins = dto.wins,
            losses = dto.losses,
            updatedAt = dto.updatedAt,
        )
    }

fun SummonerEntity.toDomain(ranked: List<RankedStat>): Summoner =
    Summoner(
        puuid = puuid,
        riotId = riotId,
        tagLine = tagLine,
        region = region,
        level = summonerLevel,
        profileIconId = profileIconId,
        lastSyncedAt = lastSyncedAt,
        ranked = ranked,
    )

fun RankedStatEntity.toDomain(): RankedStat =
    RankedStat(
        queueType = queueType,
        tier = tier,
        rank = rank,
        leaguePoints = leaguePoints,
        wins = wins,
        losses = losses,
    )
