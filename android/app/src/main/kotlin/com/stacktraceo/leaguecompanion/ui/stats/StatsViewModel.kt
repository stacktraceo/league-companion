package com.stacktraceo.leaguecompanion.ui.stats

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.stacktraceo.leaguecompanion.core.AppError
import com.stacktraceo.leaguecompanion.core.AppResult
import com.stacktraceo.leaguecompanion.data.repository.StatsRepository
import com.stacktraceo.leaguecompanion.domain.model.Stats
import com.stacktraceo.leaguecompanion.ui.navigation.StatsRoute
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class StatsUiState(
    val stats: Stats? = null,
    val loading: Boolean = true,
    val error: AppError? = null,
)

@HiltViewModel
class StatsViewModel
    @Inject
    constructor(
        savedStateHandle: SavedStateHandle,
        private val repository: StatsRepository,
    ) : ViewModel() {
        private val puuid: String = checkNotNull(savedStateHandle[StatsRoute::puuid.name])

        private val mutableState = MutableStateFlow(StatsUiState())
        val state: StateFlow<StatsUiState> = mutableState.asStateFlow()

        init {
            load()
        }

        fun load() {
            viewModelScope.launch {
                mutableState.update { it.copy(loading = true, error = null) }

                when (val result = repository.stats(puuid)) {
                    is AppResult.Success -> mutableState.update { it.copy(stats = result.value, loading = false) }
                    is AppResult.Failure -> mutableState.update { it.copy(loading = false, error = result.error) }
                }
            }
        }
    }
