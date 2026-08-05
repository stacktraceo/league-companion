package com.stacktraceo.leaguecompanion.data.mapper

import com.stacktraceo.leaguecompanion.data.TEST_PUUID
import com.stacktraceo.leaguecompanion.data.matchDetailJson
import com.stacktraceo.leaguecompanion.data.remote.dto.MatchDetailDto
import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.time.Duration

class MatchDetailMapperTest {
    private val json = Json { ignoreUnknownKeys = true }

    private fun parse(raw: String) = json.decodeFromString<MatchDetailDto>(raw).toDomain(TEST_PUUID)

    @Test
    fun `полторы сотни незнакомых полей не роняют разбор`() {
        // В реальном ответе у участника 155 полей, объявлено полтора десятка.
        val detail = parse(matchDetailJson())

        assertEquals("KR_1", detail.matchId)
        assertEquals(10, detail.teams.sumOf { it.players.size })
    }

    @Test
    fun `длительность с gameEndTimestamp читается как секунды`() {
        val detail = parse(matchDetailJson(gameDuration = 2523))

        assertEquals(Duration.ofSeconds(2523), detail.duration)
    }

    @Test
    fun `длительность без gameEndTimestamp читается как миллисекунды`() {
        // Матчи до патча 11.20. Без этой ветки 2 523 000 «секунд» превратились бы
        // в 700 часов игры - бэкенд нормализует это у себя, но сырой Match-V5
        // проходит мимо той нормализации.
        val detail = parse(matchDetailJson(gameDuration = 2_523_000, gameEndTimestamp = null))

        assertEquals(Duration.ofSeconds(2523), detail.duration)
    }

    @Test
    fun `участники разложены по двум командам`() {
        val detail = parse(matchDetailJson())

        assertEquals(listOf(100, 200), detail.teams.map { it.teamId })
        assertEquals(listOf(5, 5), detail.teams.map { it.players.size })
        assertEquals(listOf(true, false), detail.teams.map { it.win })
    }

    @Test
    fun `объективы и счёт команды переносятся`() {
        val team = parse(matchDetailJson()).teams.first()

        assertEquals(45, team.kills)
        assertEquals(6, team.towers)
        assertEquals(4, team.dragons)
        assertEquals(2, team.barons)
    }

    @Test
    fun `роли выстраиваются в порядке линий, а не как пришли`() {
        val detail = parse(matchDetailJson(positions = listOf("UTILITY", "TOP", "BOTTOM", "JUNGLE", "MIDDLE")))

        assertEquals(
            listOf("TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY"),
            detail.teams
                .first()
                .players
                .map { it.position },
        )
    }

    @Test
    fun `пустая позиция уходит в конец, а не наверх`() {
        // ARAM и старые матчи приходят без teamPosition. indexOf вернул бы -1,
        // и такие строки всплыли бы над топлейнером.
        val detail = parse(matchDetailJson(positions = listOf("", "MIDDLE", "", "TOP", "JUNGLE")))

        assertEquals(
            listOf("TOP", "JUNGLE", "MIDDLE", "", ""),
            detail.teams
                .first()
                .players
                .map { it.position },
        )
    }

    @Test
    fun `свой участник помечен, остальные нет`() {
        val players = parse(matchDetailJson()).teams.flatMap { it.players }

        assertEquals(1, players.count { it.tracked })
        assertTrue(players.first { it.tracked }.puuid == TEST_PUUID)
        assertFalse(players.last().tracked)
    }

    @Test
    fun `CS складывается из миньонов и леса`() {
        // По отдельности ни одно из чисел не совпадает с тем, что показывает клиент.
        val player =
            parse(matchDetailJson())
                .teams
                .first()
                .players
                .first()

        assertEquals(215, player.cs)
    }

    @Test
    fun `имя игрока собирается из Riot ID, а не из чемпиона`() {
        val player =
            parse(matchDetailJson())
                .teams
                .first()
                .players
                .first()

        assertEquals("Player1000#TAG", player.displayName)
    }
}
