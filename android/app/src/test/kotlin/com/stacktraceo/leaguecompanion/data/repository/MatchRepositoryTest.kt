package com.stacktraceo.leaguecompanion.data.repository

import com.stacktraceo.leaguecompanion.core.AppError
import com.stacktraceo.leaguecompanion.core.AppResult
import com.stacktraceo.leaguecompanion.data.TEST_PUUID
import com.stacktraceo.leaguecompanion.data.matchItemDto
import com.stacktraceo.leaguecompanion.data.remote.ApiErrorMapper
import com.stacktraceo.leaguecompanion.data.remote.dto.MatchListDto
import com.stacktraceo.leaguecompanion.data.remote.dto.MatchListItemDto
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.IOException
import java.time.Instant

class MatchRepositoryTest {
    private val api = FakeLeagueApi()
    private val dao = FakeMatchDao()
    private val repository = MatchRepository(api, dao, ApiErrorMapper(Json { ignoreUnknownKeys = true }))

    @Test
    fun `страница из сети появляется в ленте, свежие матчи сверху`() =
        runTest {
            api.matchesResponse =
                page(
                    matchItemDto(matchId = "KR_1", gameCreation = Instant.parse("2026-07-08T10:00:00Z")),
                    matchItemDto(matchId = "KR_3", gameCreation = Instant.parse("2026-07-09T18:00:00Z")),
                    matchItemDto(matchId = "KR_2", gameCreation = Instant.parse("2026-07-09T09:00:00Z")),
                )

            val result = repository.refresh(TEST_PUUID)

            assertEquals(AppResult.Success(MatchPage(loaded = 3, total = 3)), result)
            val feed = repository.observeFeed(TEST_PUUID).first()
            assertEquals(listOf("KR_3", "KR_2", "KR_1"), feed.map { it.matchId })
        }

    @Test
    fun `повторная загрузка той же страницы не удваивает ленту`() =
        runTest {
            api.matchesResponse = page(matchItemDto(matchId = "KR_1"), matchItemDto(matchId = "KR_2"))

            repository.refresh(TEST_PUUID)
            repository.refresh(TEST_PUUID)

            // Ключи строк берутся из данных, поэтому вторая загрузка обновляет
            // те же строки. Сгенерируй мы id при вставке - лента росла бы вдвое
            // с каждым pull-to-refresh.
            assertEquals(2, repository.cachedCount(TEST_PUUID))
            assertEquals(listOf("KR_2", "KR_1"), repository.observeFeed(TEST_PUUID).first().map { it.matchId })
        }

    @Test
    fun `упавшая сеть оставляет кэш нетронутым`() =
        runTest {
            api.matchesResponse = page(matchItemDto(matchId = "KR_1"))
            repository.refresh(TEST_PUUID)

            api.failure = IOException("бэкенд не поднят")
            val result = repository.refresh(TEST_PUUID)

            assertEquals(AppResult.Failure(AppError.NoNetwork), result)
            assertEquals(listOf("KR_1"), repository.observeFeed(TEST_PUUID).first().map { it.matchId })
        }

    @Test
    fun `лента отдаёт не больше запрошенного окна`() =
        runTest {
            api.matchesResponse =
                page(
                    matchItemDto(matchId = "KR_1", gameCreation = Instant.parse("2026-07-09T10:00:00Z")),
                    matchItemDto(matchId = "KR_2", gameCreation = Instant.parse("2026-07-09T11:00:00Z")),
                    matchItemDto(matchId = "KR_3", gameCreation = Instant.parse("2026-07-09T12:00:00Z")),
                    total = 40,
                )
            repository.refresh(TEST_PUUID)

            val feed = repository.observeFeed(TEST_PUUID, limit = 2).first()

            assertEquals(listOf("KR_3", "KR_2"), feed.map { it.matchId })
        }

    @Test
    fun `лента чужого саммонера не подмешивается`() =
        runTest {
            api.matchesResponse = page(matchItemDto(matchId = "KR_1"))
            repository.refresh(TEST_PUUID)

            // Один и тот же матч может принадлежать двум отслеживаемым саммонерам -
            // разделяет их participant, а не матч.
            assertTrue(repository.observeFeed("puuid-другой").first().isEmpty())
        }

    @Test
    fun `окно передаётся бэкенду как есть`() =
        runTest {
            api.matchesResponse = page(total = 100)

            repository.refresh(TEST_PUUID, limit = 20, offset = 40)

            assertEquals(listOf(20 to 40), api.matchRequests)
        }

    private fun page(
        vararg items: MatchListItemDto,
        total: Int = items.size,
    ) = MatchListDto(items = items.toList(), limit = MatchRepository.DEFAULT_PAGE_SIZE, offset = 0, total = total)
}
