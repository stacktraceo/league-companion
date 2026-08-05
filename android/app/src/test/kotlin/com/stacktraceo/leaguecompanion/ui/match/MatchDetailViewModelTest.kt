package com.stacktraceo.leaguecompanion.ui.match

import androidx.lifecycle.SavedStateHandle
import com.stacktraceo.leaguecompanion.MainDispatcherRule
import com.stacktraceo.leaguecompanion.core.AppError
import com.stacktraceo.leaguecompanion.data.TEST_PUUID
import com.stacktraceo.leaguecompanion.data.httpException
import com.stacktraceo.leaguecompanion.data.matchDetailJson
import com.stacktraceo.leaguecompanion.data.remote.ApiErrorMapper
import com.stacktraceo.leaguecompanion.data.repository.FakeLeagueApi
import com.stacktraceo.leaguecompanion.data.repository.FakeMatchDetailDao
import com.stacktraceo.leaguecompanion.data.repository.MatchDetailRepository
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

class MatchDetailViewModelTest {
    @get:Rule
    val mainDispatcher = MainDispatcherRule()

    private val json = Json { ignoreUnknownKeys = true }
    private val api = FakeLeagueApi()
    private val dao = FakeMatchDetailDao()
    private val details = MatchDetailRepository(api, dao, json, ApiErrorMapper(json))

    @Test
    fun `после загрузки видны обе команды и свой игрок`() =
        runTest {
            api.matchDetailJson = matchDetailJson(matchId = "KR_1")

            val viewModel = viewModel()
            observeState(viewModel)
            advanceUntilIdle()

            val state = viewModel.state.value
            assertEquals(listOf(100, 200), state.detail?.teams?.map { it.teamId })
            assertEquals(listOf(5, 5), state.detail?.teams?.map { it.players.size })
            assertEquals(
                1,
                state.detail
                    ?.teams
                    ?.flatMap { it.players }
                    ?.count { it.tracked },
            )
            assertNull(state.error)
            assertFalse(state.firstLoad)
        }

    @Test
    fun `пустой кэш и упавшая сеть - ошибка без контента`() =
        runTest {
            api.failure = IOException("бэкенд не поднят")

            val viewModel = viewModel()
            observeState(viewModel)
            advanceUntilIdle()

            val state = viewModel.state.value
            assertEquals(AppError.NoNetwork, state.error)
            assertFalse(state.hasContent)
            // Экран покажет ErrorState с повтором, а не спиннер навсегда.
            assertFalse(state.firstLoad)
        }

    @Test
    fun `матч из кэша виден и без сети`() =
        runTest {
            api.matchDetailJson = matchDetailJson(matchId = "KR_1")
            details.refresh("KR_1")

            api.failure = IOException("режим полёта")
            val viewModel = viewModel()
            observeState(viewModel)
            advanceUntilIdle()

            val state = viewModel.state.value
            // Ошибка обновления есть, но она уходит снекбаром поверх готового экрана.
            assertEquals(AppError.NoNetwork, state.error)
            assertTrue(state.hasContent)
            assertEquals("KR_1", state.detail?.matchId)
        }

    @Test
    fun `не синхронизированный матч отличается от «нет сети»`() =
        runTest {
            api.failure = httpException(code = 404, body = """{"error":"match_not_found","message":"матч не найден"}""")

            val viewModel = viewModel()
            observeState(viewModel)
            advanceUntilIdle()

            assertEquals(AppError.MatchNotFound, viewModel.state.value.error)
        }

    @Test
    fun `показанная ошибка гасится и не всплывает снова`() =
        runTest {
            api.matchDetailJson = matchDetailJson(matchId = "KR_1")

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
        MatchDetailViewModel(
            savedStateHandle = SavedStateHandle(mapOf("matchId" to "KR_1", "puuid" to TEST_PUUID)),
            details = details,
        )

    private fun TestScope.observeState(viewModel: MatchDetailViewModel) {
        backgroundScope.launch(UnconfinedTestDispatcher(testScheduler)) { viewModel.state.collect { } }
    }
}
