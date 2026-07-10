package com.stacktraceo.leaguecompanion.data.repository

import com.stacktraceo.leaguecompanion.core.AppError
import com.stacktraceo.leaguecompanion.core.AppResult
import com.stacktraceo.leaguecompanion.data.TEST_PUUID
import com.stacktraceo.leaguecompanion.data.rankedDto
import com.stacktraceo.leaguecompanion.data.remote.ApiErrorMapper
import com.stacktraceo.leaguecompanion.data.summonerDto
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import java.io.IOException

class SummonerRepositoryTest {
    private val api = FakeLeagueApi()
    private val dao = FakeSummonerDao()
    private val repository = SummonerRepository(api, dao, ApiErrorMapper(Json { ignoreUnknownKeys = true }))

    @Test
    fun `track кладёт профиль и ранг в кэш и возвращает puuid`() =
        runTest {
            api.summonerResponse = summonerDto()

            val result = repository.track(riotId = "Faker", tagLine = "KR1", region = "kr")

            assertEquals(AppResult.Success(TEST_PUUID), result)
            val cached = repository.observe(TEST_PUUID).first()
            assertEquals("Faker#KR1", cached?.displayName)
            assertEquals(listOf("RANKED_SOLO_5x5"), cached?.ranked?.map { it.queueType })
        }

    @Test
    fun `подписка пуста, пока саммонер не добавлен`() =
        runTest {
            assertNull(repository.observe("puuid-которого-нет").first())
        }

    @Test
    fun `упавшая сеть не стирает то, что уже лежит в кэше`() =
        runTest {
            api.summonerResponse = summonerDto()
            repository.track(riotId = "Faker", tagLine = "KR1", region = "kr")

            api.failure = IOException("бэкенд не поднят")
            val result = repository.refresh(TEST_PUUID)

            assertEquals(AppResult.Failure(AppError.NoNetwork), result)
            // Ради этого чтение и отделено от обновления: провал refresh — событие,
            // а не повод очистить экран.
            assertEquals("Faker#KR1", repository.observe(TEST_PUUID).first()?.displayName)
        }

    @Test
    fun `исчезнувшая очередь уходит из кэша, а не остаётся навсегда`() =
        runTest {
            api.summonerResponse =
                summonerDto(
                    ranked = listOf(rankedDto(queueType = "RANKED_SOLO_5x5"), rankedDto(queueType = "RANKED_FLEX_SR")),
                )
            repository.track(riotId = "Faker", tagLine = "KR1", region = "kr")

            // Новый сезон: флекс из ответа пропал. Если бы ранг досыпался, а не
            // замещался, экран профиля показывал бы прошлогодний дивизион.
            api.summonerResponse = summonerDto(ranked = listOf(rankedDto(queueType = "RANKED_SOLO_5x5")))
            repository.refresh(TEST_PUUID)

            val cached = repository.observe(TEST_PUUID).first()
            assertEquals(listOf("RANKED_SOLO_5x5"), cached?.ranked?.map { it.queueType })
        }

    @Test
    fun `обновление профиля переносит новый уровень`() =
        runTest {
            api.summonerResponse = summonerDto(summonerLevel = 742)
            repository.track(riotId = "Faker", tagLine = "KR1", region = "kr")

            api.summonerResponse = summonerDto(summonerLevel = 743)
            repository.refresh(TEST_PUUID)

            assertEquals(743, repository.observe(TEST_PUUID).first()?.level)
        }
}
