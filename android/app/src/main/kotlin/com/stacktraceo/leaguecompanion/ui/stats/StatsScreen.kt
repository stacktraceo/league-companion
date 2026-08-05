package com.stacktraceo.leaguecompanion.ui.stats

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Card
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.stacktraceo.leaguecompanion.R
import com.stacktraceo.leaguecompanion.domain.model.ChampionStats
import com.stacktraceo.leaguecompanion.domain.model.Stats
import com.stacktraceo.leaguecompanion.ui.components.EmptyState
import com.stacktraceo.leaguecompanion.ui.components.ErrorState
import com.stacktraceo.leaguecompanion.ui.components.LoadingBox
import com.stacktraceo.leaguecompanion.ui.error.asText
import com.stacktraceo.leaguecompanion.ui.format.formatKda
import com.stacktraceo.leaguecompanion.ui.format.formatWinRate

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun StatsScreen(
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
    viewModel: StatsViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val errorText = state.error?.asText()

    Scaffold(
        modifier = modifier.fillMaxSize(),
        topBar = {
            TopAppBar(
                title = { Text(stringResource(R.string.stats_title)) },
                navigationIcon = {
                    TextButton(onClick = onBack) { Text(stringResource(R.string.action_back)) }
                },
            )
        },
    ) { innerPadding ->
        Box(modifier = Modifier.padding(innerPadding)) {
            val stats = state.stats

            when {
                state.loading && stats == null -> LoadingBox()

                errorText != null && stats == null ->
                    ErrorState(message = errorText, onRetry = viewModel::load)

                // Ноль игр - это не ошибка и не пустой график: саммонер просто не
                // играл в этот период.
                stats == null || stats.isEmpty -> EmptyState(message = stringResource(R.string.stats_empty))

                else -> Content(stats)
            }
        }
    }
}

@Composable
private fun Content(stats: Stats) {
    LazyColumn(
        modifier = Modifier.fillMaxSize().padding(horizontal = 16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
        contentPadding = PaddingValues(vertical = 16.dp),
    ) {
        item(key = "summary") { Summary(stats) }

        if (stats.topChampions.isNotEmpty()) {
            item(key = "top") {
                Text(
                    text = stringResource(R.string.stats_top),
                    style = MaterialTheme.typography.titleMedium,
                )
            }

            // Полоски сравниваются с самым частым чемпионом, а не с общим числом игр:
            // иначе при пяти чемпионах все столбики были бы одинаково короткими.
            val maxGames = stats.topChampions.maxOf { it.games }

            items(stats.topChampions) { champion ->
                ChampionBar(champion = champion, maxGames = maxGames)
            }
        }
    }
}

@Composable
private fun Summary(stats: Stats) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(
            modifier = Modifier.padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            Text(
                text = stringResource(R.string.stats_period, stats.periodDays),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Text(
                text = stringResource(R.string.stats_winrate, formatWinRate(stats.winRate)),
                style = MaterialTheme.typography.headlineSmall,
            )
            Text(
                text = pluralStringResource(R.plurals.stats_games, stats.games, stats.games, stats.wins, stats.losses),
                style = MaterialTheme.typography.bodyMedium,
            )
            Text(
                text = stringResource(R.string.stats_kda, formatKda(stats.kda), stats.kills, stats.deaths, stats.assists),
                style = MaterialTheme.typography.bodyMedium,
            )
        }
    }
}

@Composable
private fun ChampionBar(
    champion: ChampionStats,
    maxGames: Int,
) {
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            Text(text = champion.championName, style = MaterialTheme.typography.bodyLarge)
            Text(
                text =
                    pluralStringResource(
                        R.plurals.stats_champion_line,
                        champion.games,
                        champion.games,
                        formatWinRate(champion.winRate),
                        formatKda(champion.kda),
                    ),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }

        Box(
            modifier =
                Modifier
                    .fillMaxWidth()
                    .height(8.dp)
                    .clip(RoundedCornerShape(4.dp))
                    .background(MaterialTheme.colorScheme.surfaceVariant),
        ) {
            Box(
                modifier =
                    Modifier
                        .fillMaxWidth(fraction = champion.games.toFloat() / maxGames)
                        .fillMaxHeight()
                        .background(MaterialTheme.colorScheme.primary),
            )
        }
    }
}
