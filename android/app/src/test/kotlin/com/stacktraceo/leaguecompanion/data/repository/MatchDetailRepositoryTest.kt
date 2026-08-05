package com.stacktraceo.leaguecompanion.data.repository

import com.stacktraceo.leaguecompanion.core.AppError
import com.stacktraceo.leaguecompanion.core.AppResult
import com.stacktraceo.leaguecompanion.data.TEST_PUUID
import com.stacktraceo.leaguecompanion.data.matchDetailJson
import com.stacktraceo.leaguecompanion.data.remote.ApiErrorMapper
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import java.io.IOException

class MatchDetailRepositoryTest {
    private val json = Json { ignoreUnknownKeys = true }
    private val api = FakeLeagueApi()
    private val dao = FakeMatchDetailDao()
    private val repository = MatchDetailRepository(api, dao, json, ApiErrorMapper(json))

    @Test
    fun `до первой загрузки кэш пуст`() =
        runTest {
            assertNull(repository.observe("KR_1", TEST_PUUID).first())
        }

    @Test
    fun `загруженный матч читается из кэша`() =
        runTest {
            api.matchDetailJson = matchDetailJson(matchId = "KR_1")

            assertEquals(AppResult.Success(Unit), repository.refresh("KR_1"))

            val detail = repository.observe("KR_1", TEST_PUUID).first()
            assertEquals("KR_1", detail?.matchId)
            assertEquals(10, detail?.teams?.sumOf { it.players.size })
        }

    @Test
    fun `упавшая сеть не трогает уже сохранённые детали`() =
        runTest {
            api.matchDetailJson = matchDetailJson(matchId = "KR_1")
            repository.refresh("KR_1")

            api.failure = IOException("сеть пропала")
            val result = repository.refresh("KR_1")

            assertEquals(AppResult.Failure(AppError.NoNetwork), result)
            // Ради этого детали и кладутся в Room: открытый однажды матч остаётся
            // доступен офлайн.
            assertEquals("KR_1", repository.observe("KR_1", TEST_PUUID).first()?.matchId)
        }

    @Test
    fun `битая строка кэша читается как «ничего нет», а не роняет подписку`() =
        runTest {
            api.matchDetailJson = "{ это не json"
            repository.refresh("KR_1")

            // Исключение здесь убило бы Flow, и экран уже не починился бы сам.
            assertNull(repository.observe("KR_1", TEST_PUUID).first())
        }

    @Test
    fun `свой участник определяется тем puuid, с которым читают`() =
        runTest {
            api.matchDetailJson = matchDetailJson(matchId = "KR_1", trackedPuuid = "puuid-другой")
            repository.refresh("KR_1")

            val mine = repository.observe("KR_1", "puuid-другой").first()
            val alien = repository.observe("KR_1", TEST_PUUID).first()

            assertEquals(1, mine?.teams?.flatMap { it.players }?.count { it.tracked })
            // Один и тот же матч может лежать в кэше для нескольких саммонеров -
            // подсветка не должна быть вшита в сохранённые данные.
            assertEquals(0, alien?.teams?.flatMap { it.players }?.count { it.tracked })
        }
}
