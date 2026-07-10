package com.stacktraceo.leaguecompanion.data

import com.stacktraceo.leaguecompanion.data.remote.dto.MatchListItemDto
import com.stacktraceo.leaguecompanion.data.remote.dto.RankedDto
import com.stacktraceo.leaguecompanion.data.remote.dto.SummonerDto
import java.time.Instant

// Общие заготовки ответов бэкенда. Значения ничего не значат, кроме тех, что тест
// проверяет явно, — поэтому у всего есть дефолт, и в тесте видно только важное.

const val TEST_PUUID = "puuid-faker"

fun summonerDto(
    puuid: String = TEST_PUUID,
    riotId: String = "Faker",
    tagLine: String = "KR1",
    region: String = "kr",
    summonerLevel: Int = 742,
    profileIconId: Int = 6,
    lastSyncedAt: Instant? = Instant.parse("2026-07-09T12:00:00Z"),
    createdAt: Instant = Instant.parse("2026-07-01T09:00:00Z"),
    ranked: List<RankedDto> = listOf(rankedDto()),
) = SummonerDto(
    puuid = puuid,
    riotId = riotId,
    tagLine = tagLine,
    region = region,
    summonerLevel = summonerLevel,
    profileIconId = profileIconId,
    lastSyncedAt = lastSyncedAt,
    createdAt = createdAt,
    ranked = ranked,
)

fun rankedDto(
    queueType: String = "RANKED_SOLO_5x5",
    tier: String = "CHALLENGER",
    rank: String = "I",
    leaguePoints: Int = 1337,
    wins: Int = 120,
    losses: Int = 80,
    updatedAt: Instant = Instant.parse("2026-07-09T12:00:00Z"),
) = RankedDto(
    queueType = queueType,
    tier = tier,
    rank = rank,
    leaguePoints = leaguePoints,
    wins = wins,
    losses = losses,
    updatedAt = updatedAt,
)

fun matchItemDto(
    matchId: String = "KR_1",
    gameCreation: Instant = Instant.parse("2026-07-09T10:00:00Z"),
    gameDurationSeconds: Int = 1830,
    queueId: Int = 420,
    gameVersion: String = "26.13.1",
    championName: String = "Azir",
    kills: Int = 9,
    deaths: Int = 2,
    assists: Int = 11,
    kda: Double = 10.0,
    win: Boolean = true,
    cs: Int = 254,
    goldEarned: Int = 16_400,
) = MatchListItemDto(
    matchId = matchId,
    gameCreation = gameCreation,
    gameDurationSeconds = gameDurationSeconds,
    queueId = queueId,
    gameVersion = gameVersion,
    championName = championName,
    kills = kills,
    deaths = deaths,
    assists = assists,
    kda = kda,
    win = win,
    cs = cs,
    goldEarned = goldEarned,
)
