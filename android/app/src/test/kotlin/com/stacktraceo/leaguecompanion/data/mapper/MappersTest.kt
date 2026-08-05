package com.stacktraceo.leaguecompanion.data.mapper

import com.stacktraceo.leaguecompanion.data.local.view.MatchListItemView
import com.stacktraceo.leaguecompanion.data.matchItemDto
import com.stacktraceo.leaguecompanion.data.rankedDto
import com.stacktraceo.leaguecompanion.data.summonerDto
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.time.Duration
import java.time.Instant

class MappersTest {
    @Test
    fun `профиль доезжает от DTO до домена без потерь`() {
        val dto = summonerDto()

        val summoner = dto.toEntity().toDomain(dto.toRankedEntities().map { it.toDomain() })

        assertEquals("puuid-faker", summoner.puuid)
        assertEquals("Faker#KR1", summoner.displayName)
        assertEquals("kr", summoner.region)
        assertEquals(742, summoner.level)
        assertEquals(Instant.parse("2026-07-09T12:00:00Z"), summoner.lastSyncedAt)
    }

    @Test
    fun `профиль без синхронизации сохраняет пустой lastSyncedAt`() {
        // POST /summoners отвечает 201 до того, как отработает фоновая синхронизация:
        // «профиль есть, матчей ещё нет» - обычное состояние, а не сбой разбора.
        val dto = summonerDto(lastSyncedAt = null, ranked = emptyList())

        val summoner = dto.toEntity().toDomain(emptyList())

        assertNull(summoner.lastSyncedAt)
        assertTrue(summoner.ranked.isEmpty())
    }

    @Test
    fun `ранг переносится по очередям, винрейт - доля`() {
        val dto =
            summonerDto(
                ranked =
                    listOf(
                        rankedDto(queueType = "RANKED_SOLO_5x5", wins = 60, losses = 40),
                        rankedDto(queueType = "RANKED_FLEX_SR", wins = 0, losses = 0),
                    ),
            )

        val ranked = dto.toRankedEntities().map { it.toDomain() }

        assertEquals(listOf("RANKED_SOLO_5x5", "RANKED_FLEX_SR"), ranked.map { it.queueType })
        assertEquals(0.6, ranked[0].winRate, 1e-9)
        // Свежая очередь без игр: делить не на что, и это не повод падать.
        assertEquals(0.0, ranked[1].winRate, 1e-9)
    }

    @Test
    fun `строка ленты разъезжается в матч и участие с ключами из данных`() {
        val dto = matchItemDto(matchId = "KR_777")

        val match = dto.toMatchEntity()
        val participant = dto.toParticipantEntity("puuid-1")

        assertEquals("KR_777", match.matchId)
        assertEquals(1830, match.gameDurationSeconds)
        assertEquals("KR_777", participant.matchId)
        assertEquals("puuid-1", participant.puuid)
        assertEquals("Azir", participant.championName)
        // KDA берётся у бэкенда, а не считается заново.
        assertEquals(10.0, participant.kda, 1e-9)
    }

    @Test
    fun `длительность из кэша приходит как Duration, а не как голые секунды`() {
        val view = matchListItemView(gameDurationSeconds = 1830)

        val item = view.toDomain()

        assertEquals(Duration.ofMinutes(30).plusSeconds(30), item.duration)
        assertEquals(Instant.parse("2026-07-09T10:00:00Z"), item.playedAt)
    }

    private fun matchListItemView(gameDurationSeconds: Int) =
        MatchListItemView(
            matchId = "KR_1",
            gameCreation = Instant.parse("2026-07-09T10:00:00Z"),
            gameDurationSeconds = gameDurationSeconds,
            queueId = 420,
            gameVersion = "26.13.1",
            championName = "Azir",
            kills = 9,
            deaths = 2,
            assists = 11,
            kda = 10.0,
            win = true,
            cs = 254,
            goldEarned = 16_400,
        )
}
