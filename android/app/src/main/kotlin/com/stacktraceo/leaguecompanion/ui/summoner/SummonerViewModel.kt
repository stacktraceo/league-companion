package com.stacktraceo.leaguecompanion.ui.summoner

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.navigation.toRoute
import com.stacktraceo.leaguecompanion.core.AppError
import com.stacktraceo.leaguecompanion.core.AppResult
import com.stacktraceo.leaguecompanion.data.repository.MatchRepository
import com.stacktraceo.leaguecompanion.data.repository.SummonerRepository
import com.stacktraceo.leaguecompanion.domain.model.MatchListItem
import com.stacktraceo.leaguecompanion.domain.model.Summoner
import com.stacktraceo.leaguecompanion.ui.navigation.SummonerRoute
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.async
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.flatMapLatest
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * Состояние экрана — один класс с флагами, а не sealed-иерархия.
 *
 * Офлайн-first по своей природе комбинирует: данные уже есть, обновление идёт, а
 * прошлая попытка упала — всё одновременно. В sealed-варианте пришлось бы заводить
 * состояние на каждое сочетание, и «показать список, но с ошибкой сверху»
 * превратилось бы в отдельный случай вместо двух независимых полей.
 */
data class SummonerUiState(
    val summoner: Summoner? = null,
    val matches: List<MatchListItem> = emptyList(),
    /** Идёт обновление. При непустом кэше это индикатор, а не заслонка. */
    val refreshing: Boolean = false,
    /** Ни одного ответа ещё не было — только в этом случае уместен спиннер по центру. */
    val firstLoad: Boolean = true,
    val error: AppError? = null,
    val canLoadMore: Boolean = false,
) {
    val hasContent: Boolean get() = summoner != null
}

@HiltViewModel
class SummonerViewModel
    @Inject
    constructor(
        savedStateHandle: SavedStateHandle,
        private val summoners: SummonerRepository,
        private val matches: MatchRepository,
    ) : ViewModel() {
        private val puuid = savedStateHandle.toRoute<SummonerRoute>().puuid

        private val visible = MutableStateFlow(MatchRepository.DEFAULT_PAGE_SIZE)
        private val refreshing = MutableStateFlow(false)
        private val firstLoad = MutableStateFlow(true)
        private val error = MutableStateFlow<AppError?>(null)

        /** Сколько матчей у бэкенда всего; null — пока ни один ответ не пришёл. */
        private val total = MutableStateFlow<Int?>(null)

        @OptIn(ExperimentalCoroutinesApi::class)
        private val feed = visible.flatMapLatest { limit -> matches.observeFeed(puuid, limit) }

        // Флаги собираются в один поток отдельно: у combine типизированные перегрузки
        // кончаются на пяти источниках, а их шесть. Вложенный combine честнее, чем
        // подглядывание в .value изнутри — от последнего состояние не пересчиталось бы
        // при изменении total, и кнопка «ещё» появлялась бы с опозданием на такт.
        private val progress =
            combine(refreshing, firstLoad, error, total) { refreshing, firstLoad, error, total ->
                Progress(refreshing = refreshing, firstLoad = firstLoad, error = error, total = total)
            }

        val state: StateFlow<SummonerUiState> =
            combine(summoners.observe(puuid), feed, progress) { summoner, feed, progress ->
                SummonerUiState(
                    summoner = summoner,
                    matches = feed,
                    refreshing = progress.refreshing,
                    firstLoad = progress.firstLoad,
                    error = progress.error,
                    canLoadMore = progress.total?.let { feed.size < it } == true,
                )
            }.stateIn(viewModelScope, SharingStarted.WhileSubscribed(STOP_TIMEOUT_MILLIS), SummonerUiState())

        init {
            refresh()
        }

        /**
         * Профиль и первая страница матчей тянутся параллельно: последовательно это
         * было бы два круга по сети ради данных, которые не зависят друг от друга.
         * Ошибка любой из половин показывается пользователю, но кэш не трогает —
         * этим и куплено то, что провал обновления не очищает экран.
         */
        fun refresh() {
            viewModelScope.launch {
                refreshing.value = true
                try {
                    val profile = async { summoners.refresh(puuid) }
                    val page = async { matches.refresh(puuid, offset = 0) }

                    val pageResult = page.await()
                    if (pageResult is AppResult.Success) total.value = pageResult.value.total

                    error.value = firstFailure(profile.await(), pageResult)
                } finally {
                    refreshing.value = false
                    firstLoad.value = false
                }
            }
        }

        fun loadMore() {
            viewModelScope.launch {
                refreshing.value = true
                try {
                    val cached = matches.cachedCount(puuid)
                    // Окно растёт сразу: если фон уже успел положить матчи сверх него,
                    // они появятся, не дожидаясь ответа сети.
                    visible.update { maxOf(it, cached) + MatchRepository.DEFAULT_PAGE_SIZE }

                    when (val result = matches.refresh(puuid, offset = cached)) {
                        is AppResult.Success -> {
                            total.value = result.value.total
                            error.value = null
                        }

                        is AppResult.Failure -> error.value = result.error
                    }
                } finally {
                    refreshing.value = false
                }
            }
        }

        /** Ошибка показана — снекбар не должен всплывать снова на каждом пересоставе. */
        fun errorShown() {
            error.value = null
        }

        private fun firstFailure(vararg results: AppResult<*>): AppError? =
            results.filterIsInstance<AppResult.Failure>().firstOrNull()?.error

        private companion object {
            const val STOP_TIMEOUT_MILLIS = 5_000L
        }
    }

private data class Progress(
    val refreshing: Boolean,
    val firstLoad: Boolean,
    val error: AppError?,
    val total: Int?,
)
