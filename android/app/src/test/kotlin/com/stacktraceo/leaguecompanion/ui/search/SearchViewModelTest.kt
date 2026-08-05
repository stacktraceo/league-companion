package com.stacktraceo.leaguecompanion.ui.search

import app.cash.turbine.test
import com.stacktraceo.leaguecompanion.MainDispatcherRule
import com.stacktraceo.leaguecompanion.core.AppError
import com.stacktraceo.leaguecompanion.data.TEST_PUUID
import com.stacktraceo.leaguecompanion.data.remote.ApiErrorMapper
import com.stacktraceo.leaguecompanion.data.repository.FakeLeagueApi
import com.stacktraceo.leaguecompanion.data.repository.FakeSummonerDao
import com.stacktraceo.leaguecompanion.data.repository.SummonerRepository
import com.stacktraceo.leaguecompanion.data.summonerDto
import com.stacktraceo.leaguecompanion.domain.model.Region
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Rule
import org.junit.Test
import java.io.IOException

class SearchViewModelTest {
    @get:Rule
    val mainDispatcher = MainDispatcherRule()

    private val api = FakeLeagueApi()
    private val dao = FakeSummonerDao()
    private val repository = SummonerRepository(api, dao, ApiErrorMapper(Json { ignoreUnknownKeys = true }))

    @Test
    fun `неверный формат не доходит до сети`() =
        runTest {
            val viewModel = SearchViewModel(repository)
            observeState(viewModel)

            viewModel.onInputChange("Faker")
            viewModel.submit()
            advanceUntilIdle()

            // api.summonerResponse не задан: дойди вызов до сети, фейк бросил бы
            // исключение и ошибка была бы другой.
            assertEquals(SearchError.BadFormat, viewModel.state.value.error)
        }

    @Test
    fun `успешное добавление отдаёт puuid событием`() =
        runTest {
            api.summonerResponse = summonerDto()
            val viewModel = SearchViewModel(repository)
            observeState(viewModel)

            viewModel.opened.test {
                viewModel.onInputChange("Hide on bush#KR1")
                viewModel.onRegionChange(Region.KR)
                viewModel.submit()
                advanceUntilIdle()

                assertEquals(TEST_PUUID, awaitItem())
            }
        }

    @Test
    fun `ошибка сети показывается и не стирает ввод`() =
        runTest {
            api.failure = IOException("бэкенд не поднят")
            val viewModel = SearchViewModel(repository)
            observeState(viewModel)

            viewModel.onInputChange("Faker#KR1")
            viewModel.submit()
            advanceUntilIdle()

            assertEquals(SearchError.Failed(AppError.NoNetwork), viewModel.state.value.error)
            // Набирать ник заново из-за упавшего бэкенда - грубо.
            assertEquals("Faker#KR1", viewModel.state.value.input)
        }

    @Test
    fun `правка ввода снимает ошибку`() =
        runTest {
            val viewModel = SearchViewModel(repository)
            observeState(viewModel)

            viewModel.onInputChange("Faker")
            viewModel.submit()
            advanceUntilIdle()
            assertEquals(SearchError.BadFormat, viewModel.state.value.error)

            viewModel.onInputChange("Faker#")
            advanceUntilIdle()

            assertNull(viewModel.state.value.error)
        }

    @Test
    fun `список отслеживаемых приходит из кэша`() =
        runTest {
            api.summonerResponse = summonerDto(riotId = "Hide on bush", tagLine = "KR1")
            repository.track(riotId = "Hide on bush", tagLine = "KR1", region = "kr")

            val viewModel = SearchViewModel(repository)
            observeState(viewModel)
            advanceUntilIdle()

            assertEquals(
                listOf("Hide on bush#KR1"),
                viewModel.state.value.tracked
                    .map { it.displayName },
            )
        }

    @Test
    fun `на пустом вводе кнопка недоступна`() =
        runTest {
            val viewModel = SearchViewModel(repository)
            observeState(viewModel)
            advanceUntilIdle()

            assertFalse(viewModel.state.value.canSubmit)

            viewModel.onInputChange("   ")
            advanceUntilIdle()

            // Пробелы - не ввод: иначе запрос ушёл бы с пустым ником и вернулся 400.
            assertFalse(viewModel.state.value.canSubmit)
        }

    private fun TestScope.observeState(viewModel: SearchViewModel) {
        backgroundScope.launch(UnconfinedTestDispatcher(testScheduler)) { viewModel.state.collect { } }
    }
}
