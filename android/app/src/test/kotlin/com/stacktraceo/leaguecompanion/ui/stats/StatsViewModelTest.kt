package com.stacktraceo.leaguecompanion.ui.stats

import androidx.lifecycle.SavedStateHandle
import com.stacktraceo.leaguecompanion.MainDispatcherRule
import com.stacktraceo.leaguecompanion.core.AppError
import com.stacktraceo.leaguecompanion.data.TEST_PUUID
import com.stacktraceo.leaguecompanion.data.championStatsDto
import com.stacktraceo.leaguecompanion.data.remote.ApiErrorMapper
import com.stacktraceo.leaguecompanion.data.repository.FakeLeagueApi
import com.stacktraceo.leaguecompanion.data.repository.StatsRepository
import com.stacktraceo.leaguecompanion.data.statsDto
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

class StatsViewModelTest {
    @get:Rule
    val mainDispatcher = MainDispatcherRule()

    private val api = FakeLeagueApi()
    private val repository = StatsRepository(api, ApiErrorMapper(Json { ignoreUnknownKeys = true }))

    @Test
    fun `статистика приходит целиком, включая топ чемпионов`() =
        runTest {
            api.statsResponse = statsDto(topChampions = listOf(championStatsDto(), championStatsDto(championName = "Ahri", games = 3)))

            val viewModel = viewModel()
            advanceUntilIdle()

            val stats = viewModel.state.value.stats
            assertEquals(20, stats?.games)
            assertEquals(11, stats?.wins)
            assertEquals(9, stats?.losses)
            assertEquals(listOf("Yone", "Ahri"), stats?.topChampions?.map { it.championName })
            assertFalse(viewModel.state.value.loading)
            assertNull(viewModel.state.value.error)
        }

    @Test
    fun `период по умолчанию — тридцать дней из SPEC`() =
        runTest {
            api.statsResponse = statsDto()

            viewModel()
            advanceUntilIdle()

            assertEquals(listOf("30d"), api.statsRequests)
        }

    @Test
    fun `ноль игр за период — это пусто, а не ошибка`() =
        runTest {
            api.statsResponse =
                statsDto(
                    games = 0,
                    wins = 0,
                    losses = 0,
                    winRate = 0.0,
                    kills = 0,
                    deaths = 0,
                    assists = 0,
                    kda = 0.0,
                    topChampions = emptyList(),
                )

            val viewModel = viewModel()
            advanceUntilIdle()

            // Экран покажет EmptyState, а не нули и пустой график.
            assertTrue(
                viewModel.state.value.stats
                    ?.isEmpty == true,
            )
            assertNull(viewModel.state.value.error)
        }

    @Test
    fun `офлайн статистика честно падает — локально её не посчитать`() =
        runTest {
            api.failure = IOException("режим полёта")

            val viewModel = viewModel()
            advanceUntilIdle()

            assertEquals(AppError.NoNetwork, viewModel.state.value.error)
            assertNull(viewModel.state.value.stats)
            assertFalse(viewModel.state.value.loading)
        }

    @Test
    fun `повтор после ошибки сбрасывает её и подтягивает данные`() =
        runTest {
            api.failure = IOException("бэкенд не поднят")
            val viewModel = viewModel()
            advanceUntilIdle()
            assertEquals(AppError.NoNetwork, viewModel.state.value.error)

            api.failure = null
            api.statsResponse = statsDto()
            viewModel.load()
            advanceUntilIdle()

            assertNull(viewModel.state.value.error)
            assertEquals(
                20,
                viewModel.state.value.stats
                    ?.games,
            )
        }

    private fun viewModel() =
        StatsViewModel(
            savedStateHandle = SavedStateHandle(mapOf("puuid" to TEST_PUUID)),
            repository = repository,
        )
}
