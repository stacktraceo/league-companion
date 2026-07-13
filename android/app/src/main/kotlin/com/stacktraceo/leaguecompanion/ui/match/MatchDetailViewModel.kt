package com.stacktraceo.leaguecompanion.ui.match

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.stacktraceo.leaguecompanion.core.AppError
import com.stacktraceo.leaguecompanion.core.AppResult
import com.stacktraceo.leaguecompanion.data.repository.MatchDetailRepository
import com.stacktraceo.leaguecompanion.domain.model.MatchDetail
import com.stacktraceo.leaguecompanion.ui.navigation.MatchRoute
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import javax.inject.Inject

data class MatchDetailUiState(
    val detail: MatchDetail? = null,
    val refreshing: Boolean = false,
    val firstLoad: Boolean = true,
    val error: AppError? = null,
) {
    val hasContent: Boolean get() = detail != null
}

@HiltViewModel
class MatchDetailViewModel
    @Inject
    constructor(
        savedStateHandle: SavedStateHandle,
        private val details: MatchDetailRepository,
    ) : ViewModel() {
        // По именам свойств маршрута, а не через toRoute(): тот разбирает Bundle,
        // которого нет в JVM-тестах (журнал решений, «Дни 11–12»).
        private val matchId: String = checkNotNull(savedStateHandle[MatchRoute::matchId.name])
        private val puuid: String = checkNotNull(savedStateHandle[MatchRoute::puuid.name])

        private val refreshing = MutableStateFlow(false)
        private val firstLoad = MutableStateFlow(true)
        private val error = MutableStateFlow<AppError?>(null)

        val state: StateFlow<MatchDetailUiState> =
            combine(details.observe(matchId, puuid), refreshing, firstLoad, error) { detail, refreshing, firstLoad, error ->
                MatchDetailUiState(detail = detail, refreshing = refreshing, firstLoad = firstLoad, error = error)
            }.stateIn(viewModelScope, SharingStarted.WhileSubscribed(STOP_TIMEOUT_MILLIS), MatchDetailUiState())

        init {
            refresh()
        }

        /**
         * Обновление идёт всегда, даже когда матч уже в кэше: детали неизменяемы, но
         * набор разбираемых полей растёт от версии к версии, а перекачать один матч
         * дешевле, чем объяснять пустые места на экране.
         */
        fun refresh() {
            viewModelScope.launch {
                refreshing.value = true
                try {
                    error.value = (details.refresh(matchId) as? AppResult.Failure)?.error
                } finally {
                    refreshing.value = false
                    firstLoad.value = false
                }
            }
        }

        fun errorShown() {
            error.value = null
        }

        private companion object {
            const val STOP_TIMEOUT_MILLIS = 5_000L
        }
    }
