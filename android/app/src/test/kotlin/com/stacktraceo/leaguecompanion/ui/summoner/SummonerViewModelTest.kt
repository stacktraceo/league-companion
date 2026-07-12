package com.stacktraceo.leaguecompanion.ui.summoner

import androidx.lifecycle.SavedStateHandle
import com.stacktraceo.leaguecompanion.MainDispatcherRule
import com.stacktraceo.leaguecompanion.core.AppError
import com.stacktraceo.leaguecompanion.data.TEST_PUUID
import com.stacktraceo.leaguecompanion.data.matchItemDto
import com.stacktraceo.leaguecompanion.data.remote.ApiErrorMapper
import com.stacktraceo.leaguecompanion.data.remote.dto.MatchListDto
import com.stacktraceo.leaguecompanion.data.remote.dto.MatchListItemDto
import com.stacktraceo.leaguecompanion.data.repository.FakeLeagueApi
import com.stacktraceo.leaguecompanion.data.repository.FakeMatchDao
import com.stacktraceo.leaguecompanion.data.repository.FakeSummonerDao
import com.stacktraceo.leaguecompanion.data.repository.MatchRepository
import com.stacktraceo.leaguecompanion.data.repository.SummonerRepository
import com.stacktraceo.leaguecompanion.data.summonerDto
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import java.io.IOException

class SummonerViewModelTest {
    @get:Rule
    val mainDispatcher = MainDispatcherRule()

    private val api = FakeLeagueApi()
    private val summonerDao = FakeSummonerDao()
    private val matchDao = FakeMatchDao()
    private val errors = ApiErrorMapper(Json { ignoreUnknownKeys = true })
    private val summoners = SummonerRepository(api, summonerDao, errors)
    private val matches = MatchRepository(api, matchDao, errors)

    @Test
    fun `после обновления виден профиль и лента`() =
        runTest {
            api.summonerResponse = summonerDto()
            api.matchesResponse = page(matchItemDto(matchId = "KR_1"), matchItemDto(matchId = "KR_2"))

            val viewModel = viewModel()
            observeState(viewModel)
            advanceUntilIdle()

            val state = viewModel.state.value
            assertEquals("Faker#KR1", state.summoner?.displayName)
            assertEquals(listOf("KR_2", "KR_1"), state.matches.map { it.matchId })
            assertNull(state.error)
            assertFalse(state.firstLoad)
        }

    @Test
    fun `пустой кэш и упавшая сеть — ошибка без контента`() =
        runTest {
            api.failure = IOException("бэкенд не поднят")

            val viewModel = viewModel()
            observeState(viewModel)
            advanceUntilIdle()

            val state = viewModel.state.value
            assertEquals(AppError.NoNetwork, state.error)
            assertFalse(state.hasContent)
            // Именно этот случай экран показывает ErrorState с кнопкой повтора,
            // а не снекбаром поверх пустоты.
            assertFalse(state.firstLoad)
        }

    @Test
    fun `ошибка обновления при непустом кэше не стирает список`() =
        runTest {
            api.summonerResponse = summonerDto()
            api.matchesResponse = page(matchItemDto(matchId = "KR_1"))

            val viewModel = viewModel()
            observeState(viewModel)
            advanceUntilIdle()

            api.failure = IOException("сеть пропала")
            viewModel.refresh()
            advanceUntilIdle()

            val state = viewModel.state.value
            assertEquals(AppError.NoNetwork, state.error)
            // Главная гарантия офлайн-first: провал обновления — событие, а не
            // повод очистить экран.
            assertTrue(state.hasContent)
            assertEquals(listOf("KR_1"), state.matches.map { it.matchId })
        }

    @Test
    fun `кнопка «ещё» появляется, когда у бэкенда матчей больше показанного`() =
        runTest {
            api.summonerResponse = summonerDto()
            api.matchesResponse = page(matchItemDto(matchId = "KR_1"), total = 40)

            val viewModel = viewModel()
            observeState(viewModel)
            advanceUntilIdle()

            assertTrue(viewModel.state.value.canLoadMore)
        }

    @Test
    fun `когда загружено всё, кнопка «ещё» скрыта`() =
        runTest {
            api.summonerResponse = summonerDto()
            api.matchesResponse = page(matchItemDto(matchId = "KR_1"), total = 1)

            val viewModel = viewModel()
            observeState(viewModel)
            advanceUntilIdle()

            assertFalse(viewModel.state.value.canLoadMore)
        }

    @Test
    fun `догрузка запрашивает следующую страницу от числа уже закэшированных`() =
        runTest {
            api.summonerResponse = summonerDto()
            api.matchesResponse = page(matchItemDto(matchId = "KR_1"), matchItemDto(matchId = "KR_2"), total = 40)

            val viewModel = viewModel()
            observeState(viewModel)
            advanceUntilIdle()

            viewModel.loadMore()
            advanceUntilIdle()

            // Первый запрос — стартовая страница, второй — со сдвигом на то, что уже лежит.
            assertEquals(listOf(20 to 0, 20 to 2), api.matchRequests)
        }

    @Test
    fun `показанная ошибка гасится и не всплывает снова`() =
        runTest {
            api.summonerResponse = summonerDto()
            api.matchesResponse = page(matchItemDto(matchId = "KR_1"))

            val viewModel = viewModel()
            observeState(viewModel)
            advanceUntilIdle()

            api.failure = IOException("сеть пропала")
            viewModel.refresh()
            advanceUntilIdle()
            assertEquals(AppError.NoNetwork, viewModel.state.value.error)

            viewModel.errorShown()
            advanceUntilIdle()

            assertNull(viewModel.state.value.error)
        }

    private fun viewModel() =
        SummonerViewModel(
            savedStateHandle = SavedStateHandle(mapOf("puuid" to TEST_PUUID)),
            summoners = summoners,
            matches = matches,
        )

    private fun page(
        vararg items: MatchListItemDto,
        total: Int = items.size,
    ) = MatchListDto(items = items.toList(), limit = MatchRepository.DEFAULT_PAGE_SIZE, offset = 0, total = total)

    private fun TestScope.observeState(viewModel: SummonerViewModel) {
        backgroundScope.launch(UnconfinedTestDispatcher(testScheduler)) { viewModel.state.collect { } }
    }
}
