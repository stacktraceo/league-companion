package com.stacktraceo.leaguecompanion.ui.search

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.stacktraceo.leaguecompanion.core.AppError
import com.stacktraceo.leaguecompanion.core.AppResult
import com.stacktraceo.leaguecompanion.data.repository.SummonerRepository
import com.stacktraceo.leaguecompanion.domain.model.Region
import com.stacktraceo.leaguecompanion.domain.model.RiotId
import com.stacktraceo.leaguecompanion.domain.model.Summoner
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import javax.inject.Inject

sealed interface SearchError {
    data object BadFormat : SearchError

    data class Failed(
        val error: AppError,
    ) : SearchError
}

data class SearchUiState(
    val input: String = "",
    val region: Region = Region.DEFAULT,
    val tracked: List<Summoner> = emptyList(),
    val submitting: Boolean = false,
    val error: SearchError? = null,
) {
    val canSubmit: Boolean get() = input.isNotBlank() && !submitting
}

@HiltViewModel
class SearchViewModel
    @Inject
    constructor(
        private val summoners: SummonerRepository,
    ) : ViewModel() {
        private val input = MutableStateFlow("")
        private val region = MutableStateFlow(Region.DEFAULT)
        private val submitting = MutableStateFlow(false)
        private val error = MutableStateFlow<SearchError?>(null)

        // Переход - событие, а не состояние: положи мы puuid в state, возврат назад
        // тут же уводил бы обратно на профиль, потому что поле никуда не делось.
        private val openEvents = Channel<String>(Channel.BUFFERED)
        val opened: Flow<String> = openEvents.receiveAsFlow()

        val state: StateFlow<SearchUiState> =
            combine(input, region, summoners.observeTracked(), submitting, error) { input, region, tracked, submitting, error ->
                SearchUiState(
                    input = input,
                    region = region,
                    tracked = tracked,
                    submitting = submitting,
                    error = error,
                )
            }.stateIn(viewModelScope, SharingStarted.WhileSubscribed(STOP_TIMEOUT_MILLIS), SearchUiState())

        fun onInputChange(value: String) {
            input.value = value
            // Ошибка снимается при первом же исправлении, иначе она висит поверх
            // уже правильного ввода и выглядит так, будто ничего не изменилось.
            error.value = null
        }

        fun onRegionChange(value: Region) {
            region.value = value
            error.value = null
        }

        fun submit() {
            val riotId = RiotId.parse(input.value)
            if (riotId == null) {
                error.value = SearchError.BadFormat
                return
            }

            viewModelScope.launch {
                submitting.value = true
                error.value = null
                try {
                    // Форма при ошибке не очищается: `404` чаще всего значит опечатку
                    // или не тот регион, и заставлять набирать ник заново - грубо.
                    when (val result = summoners.track(riotId.gameName, riotId.tagLine, region.value.code)) {
                        is AppResult.Success -> openEvents.send(result.value)
                        is AppResult.Failure -> error.value = SearchError.Failed(result.error)
                    }
                } finally {
                    submitting.value = false
                }
            }
        }

        private companion object {
            const val STOP_TIMEOUT_MILLIS = 5_000L
        }
    }
