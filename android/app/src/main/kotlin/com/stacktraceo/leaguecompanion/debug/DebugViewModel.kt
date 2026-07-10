package com.stacktraceo.leaguecompanion.debug

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.stacktraceo.leaguecompanion.core.AppError
import com.stacktraceo.leaguecompanion.core.AppResult
import com.stacktraceo.leaguecompanion.data.repository.MatchRepository
import com.stacktraceo.leaguecompanion.data.repository.SummonerRepository
import com.stacktraceo.leaguecompanion.domain.model.MatchListItem
import com.stacktraceo.leaguecompanion.domain.model.Region
import com.stacktraceo.leaguecompanion.domain.model.Summoner
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.flatMapLatest
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * Экран-времянка для проверки связки Android ↔ бэкенд живьём. Выбрасывается вместе
 * с пакетом `debug` в вехе «Дни 11–12», когда появятся настоящие экраны.
 *
 * Тексты здесь английские, но лежат прямо в коде, а не в strings.xml: тащить
 * времянку в ресурсы — значит потом вычищать её оттуда.
 */
@HiltViewModel
class DebugViewModel
    @Inject
    constructor(
        private val summoners: SummonerRepository,
        private val matches: MatchRepository,
    ) : ViewModel() {
        private val form = MutableStateFlow(DebugForm())
        private val selected = MutableStateFlow<String?>(null)
        private val visibleMatches = MutableStateFlow(MatchRepository.DEFAULT_PAGE_SIZE)
        private val busy = MutableStateFlow(false)
        private val message = MutableStateFlow<String?>(null)

        @OptIn(ExperimentalCoroutinesApi::class)
        private val summonerFlow =
            selected.flatMapLatest { puuid ->
                if (puuid == null) flowOf(null) else summoners.observe(puuid)
            }

        @OptIn(ExperimentalCoroutinesApi::class)
        private val feedFlow =
            combine(selected, visibleMatches) { puuid, limit -> puuid to limit }
                .flatMapLatest { (puuid, limit) ->
                    if (puuid == null) flowOf(emptyList()) else matches.observeFeed(puuid, limit)
                }

        val state: StateFlow<DebugUiState> =
            combine(form, summonerFlow, feedFlow, busy, message) { form, summoner, feed, busy, message ->
                DebugUiState(
                    form = form,
                    summoner = summoner,
                    matches = feed,
                    busy = busy,
                    message = message,
                )
            }.stateIn(viewModelScope, SharingStarted.WhileSubscribed(STOP_TIMEOUT_MILLIS), DebugUiState())

        init {
            // При запуске подхватываем то, что уже лежит в кэше: именно так проверяется
            // офлайн — приложение перезапускается без сети и всё равно показывает профиль.
            viewModelScope.launch {
                val tracked = summoners.observeTracked().first { it.isNotEmpty() }
                selected.value = tracked.first().puuid
            }
        }

        fun onRiotIdChange(value: String) = form.update { it.copy(riotId = value) }

        fun onTagLineChange(value: String) = form.update { it.copy(tagLine = value) }

        fun onRegionChange(value: Region) = form.update { it.copy(region = value) }

        fun track() =
            act {
                val current = form.value
                when (val result = summoners.track(current.riotId.trim(), current.tagLine.trim(), current.region.code)) {
                    is AppResult.Success -> {
                        selected.value = result.value
                        "Tracked. Matches arrive after the first sync."
                    }

                    is AppResult.Failure -> result.error.text()
                }
            }

        fun refreshProfile() =
            withSelected { puuid ->
                when (val result = summoners.refresh(puuid)) {
                    is AppResult.Success -> "Profile updated"
                    is AppResult.Failure -> result.error.text()
                }
            }

        fun requestSync() =
            withSelected { puuid ->
                when (val result = summoners.requestSync(puuid)) {
                    is AppResult.Success -> "Sync accepted, last sync: ${result.value ?: "never"}"
                    is AppResult.Failure -> result.error.text()
                }
            }

        fun loadMatches() =
            withSelected { puuid ->
                when (val result = matches.refresh(puuid, offset = 0)) {
                    is AppResult.Success -> "Loaded ${result.value.loaded} of ${result.value.total}"
                    is AppResult.Failure -> result.error.text()
                }
            }

        fun untrack() =
            withSelected { puuid ->
                summoners.untrack(puuid)
                selected.value = null
                "Removed from cache"
            }

        private fun withSelected(block: suspend (String) -> String) =
            act {
                val puuid = selected.value
                if (puuid == null) "Nothing tracked yet" else block(puuid)
            }

        private fun act(block: suspend () -> String) {
            viewModelScope.launch {
                busy.value = true
                message.value =
                    try {
                        block()
                    } finally {
                        busy.value = false
                    }
            }
        }

        private companion object {
            const val STOP_TIMEOUT_MILLIS = 5_000L
        }
    }

data class DebugForm(
    val riotId: String = "",
    val tagLine: String = "",
    val region: Region = Region.DEFAULT,
)

data class DebugUiState(
    val form: DebugForm = DebugForm(),
    val summoner: Summoner? = null,
    val matches: List<MatchListItem> = emptyList(),
    val busy: Boolean = false,
    val message: String? = null,
)

/**
 * Текст ошибки. Различия здесь — весь смысл разбора ошибок: 401 чинится в
 * local.properties, а 502 — на бэкенде, и одинаковая формулировка отправила бы
 * искать не там.
 */
private fun AppError.text(): String =
    when (this) {
        AppError.NoNetwork -> "Backend unreachable — is it running on the address in LC_BASE_URL?"
        AppError.Unauthorized -> "Wrong X-API-Key — LC_API_KEY must match CLIENT_API_KEY on the backend"
        AppError.NotFound -> "Not found"
        AppError.RiotUnavailable -> "Backend cannot reach Riot — its key may have expired"
        is AppError.RateLimited -> "Rate limited, retry in ${retryAfterSeconds}s"
        is AppError.BadRequest -> message
        is AppError.Server -> "Backend error $statusCode"
        is AppError.Unknown -> description
    }
