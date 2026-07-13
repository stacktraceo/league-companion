package com.stacktraceo.leaguecompanion.ui.summoner

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.stacktraceo.leaguecompanion.R
import com.stacktraceo.leaguecompanion.ui.components.EmptyState
import com.stacktraceo.leaguecompanion.ui.components.ErrorState
import com.stacktraceo.leaguecompanion.ui.components.LoadingBox
import com.stacktraceo.leaguecompanion.ui.error.asText

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SummonerScreen(
    onBack: () -> Unit,
    onOpenMatch: (String) -> Unit,
    onOpenStats: () -> Unit,
    modifier: Modifier = Modifier,
    viewModel: SummonerViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val snackbarHostState = remember { SnackbarHostState() }

    // Текст ошибки берётся здесь, в композиции: asText() читает ресурсы и внутри
    // LaunchedEffect вызван быть не может.
    val errorText = state.error?.asText()

    // Снекбар — только когда контент уже есть. Если показывать нечего, ошибка
    // занимает середину экрана и предлагает повтор, а не мигает и исчезает.
    LaunchedEffect(errorText, state.hasContent) {
        if (errorText != null && state.hasContent) {
            snackbarHostState.showSnackbar(errorText)
            viewModel.errorShown()
        }
    }

    Scaffold(
        modifier = modifier.fillMaxSize(),
        snackbarHost = { SnackbarHost(snackbarHostState) },
        topBar = {
            TopAppBar(
                title = { Text(state.summoner?.displayName ?: stringResource(R.string.summoner_title_fallback)) },
                // Текстом, а не иконкой: material-icons в зависимостях нет, и тянуть
                // целый пакет ради одной стрелки при отказе от картинок в этой вехе
                // незачем — слово ещё и доступнее для screen reader'а.
                navigationIcon = {
                    TextButton(onClick = onBack) {
                        Text(stringResource(R.string.action_back))
                    }
                },
                actions = {
                    TextButton(onClick = onOpenStats, enabled = state.hasContent) {
                        Text(stringResource(R.string.action_stats))
                    }
                },
            )
        },
    ) { innerPadding ->
        Box(modifier = Modifier.padding(innerPadding)) {
            when {
                !state.hasContent && state.firstLoad -> LoadingBox()

                !state.hasContent && errorText != null ->
                    ErrorState(message = errorText, onRetry = viewModel::refresh)

                !state.hasContent -> EmptyState(message = stringResource(R.string.summoner_missing))

                else ->
                    Content(
                        state = state,
                        onRefresh = viewModel::refresh,
                        onLoadMore = viewModel::loadMore,
                        onOpenMatch = onOpenMatch,
                    )
            }
        }
    }
}

@Composable
private fun Content(
    state: SummonerUiState,
    onRefresh: () -> Unit,
    onLoadMore: () -> Unit,
    onOpenMatch: (String) -> Unit,
) {
    PullToRefreshBox(
        isRefreshing = state.refreshing,
        onRefresh = onRefresh,
        modifier = Modifier.fillMaxSize(),
    ) {
        LazyColumn(
            modifier = Modifier.fillMaxSize().padding(horizontal = 16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
            contentPadding = PaddingValues(vertical = 16.dp),
        ) {
            state.summoner?.let { summoner ->
                item(key = "profile") { ProfileHeader(summoner) }
            }

            if (state.matches.isEmpty()) {
                // После POST /summoners бэкенд отвечает раньше, чем синхронизация
                // догрузит матчи, — пустая лента здесь ожидаемое состояние.
                item(key = "empty") { EmptyState(message = stringResource(R.string.summoner_no_matches)) }
            }

            items(items = state.matches, key = { it.matchId }) { match ->
                MatchCard(match = match, onClick = { onOpenMatch(match.matchId) })
            }

            if (state.canLoadMore) {
                item(key = "more") {
                    Box(modifier = Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                        OutlinedButton(onClick = onLoadMore, enabled = !state.refreshing) {
                            Text(stringResource(R.string.action_load_more))
                        }
                    }
                }
            }
        }
    }
}
