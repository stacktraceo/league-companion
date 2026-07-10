package com.stacktraceo.leaguecompanion.debug

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.stacktraceo.leaguecompanion.domain.model.MatchListItem
import com.stacktraceo.leaguecompanion.domain.model.Region
import com.stacktraceo.leaguecompanion.domain.model.Summoner
import com.stacktraceo.leaguecompanion.ui.theme.LossColor
import com.stacktraceo.leaguecompanion.ui.theme.WinColor
import java.time.ZoneId
import java.time.format.DateTimeFormatter

/**
 * Времянка: проверить руками, что запрос уходит, ответ раскладывается по Room и
 * лента переживает офлайн. Выбрасывается вместе с пакетом в вехе «Дни 11–12»,
 * поэтому здесь нет ни навигации, ни ресурсов, ни превью.
 */
@Composable
fun DebugScreen(
    modifier: Modifier = Modifier,
    viewModel: DebugViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()

    LazyColumn(
        modifier = modifier.fillMaxSize().padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        item {
            TrackForm(
                form = state.form,
                enabled = !state.busy,
                onRiotIdChange = viewModel::onRiotIdChange,
                onTagLineChange = viewModel::onTagLineChange,
                onRegionChange = viewModel::onRegionChange,
                onTrack = viewModel::track,
            )
        }

        item {
            Actions(
                enabled = !state.busy,
                onRefresh = viewModel::refreshProfile,
                onSync = viewModel::requestSync,
                onLoadMatches = viewModel::loadMatches,
                onUntrack = viewModel::untrack,
            )
        }

        if (state.busy) {
            item { CircularProgressIndicator() }
        }

        state.message?.let { message ->
            item { Text(text = message, style = MaterialTheme.typography.bodyMedium) }
        }

        state.summoner?.let { summoner ->
            item { SummonerCard(summoner) }
        }

        item { HorizontalDivider() }

        item {
            Text(
                text = "Matches: ${state.matches.size}",
                style = MaterialTheme.typography.titleMedium,
            )
        }

        items(items = state.matches, key = { it.matchId }) { match ->
            MatchRow(match)
        }
    }
}

@Composable
private fun TrackForm(
    form: DebugForm,
    enabled: Boolean,
    onRiotIdChange: (String) -> Unit,
    onTagLineChange: (String) -> Unit,
    onRegionChange: (Region) -> Unit,
    onTrack: () -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        OutlinedTextField(
            value = form.riotId,
            onValueChange = onRiotIdChange,
            label = { Text("Riot ID") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )
        OutlinedTextField(
            value = form.tagLine,
            onValueChange = onTagLineChange,
            label = { Text("Tag line") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )

        Row(
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            RegionPicker(selected = form.region, onSelect = onRegionChange)
            Button(onClick = onTrack, enabled = enabled && form.riotId.isNotBlank() && form.tagLine.isNotBlank()) {
                Text("Track")
            }
        }
    }
}

@Composable
private fun RegionPicker(
    selected: Region,
    onSelect: (Region) -> Unit,
) {
    var expanded by remember { mutableStateOf(false) }

    Column {
        OutlinedButton(onClick = { expanded = true }) {
            Text(selected.code.uppercase())
        }
        DropdownMenu(expanded = expanded, onDismissRequest = { expanded = false }) {
            Region.entries.forEach { region ->
                DropdownMenuItem(
                    text = { Text("${region.code.uppercase()} — ${region.label}") },
                    onClick = {
                        onSelect(region)
                        expanded = false
                    },
                )
            }
        }
    }
}

@Composable
private fun Actions(
    enabled: Boolean,
    onRefresh: () -> Unit,
    onSync: () -> Unit,
    onLoadMatches: () -> Unit,
    onUntrack: () -> Unit,
) {
    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        OutlinedButton(onClick = onRefresh, enabled = enabled) { Text("Profile") }
        OutlinedButton(onClick = onSync, enabled = enabled) { Text("Sync") }
        OutlinedButton(onClick = onLoadMatches, enabled = enabled) { Text("Matches") }
        OutlinedButton(onClick = onUntrack, enabled = enabled) { Text("Forget") }
    }
}

@Composable
private fun SummonerCard(summoner: Summoner) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(
            modifier = Modifier.padding(12.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Text(text = summoner.displayName, style = MaterialTheme.typography.titleMedium)
            Text(text = "${summoner.region.uppercase()} · level ${summoner.level}")
            // Пустое значение здесь — не ошибка, а состояние «синхронизация ещё не
            // доходила до конца»; лента при нём тоже пуста.
            Text(text = "Last sync: ${summoner.lastSyncedAt?.let(TIMESTAMP::format) ?: "never"}")

            summoner.ranked.forEach { ranked ->
                val division = "${ranked.tier} ${ranked.rank}, ${ranked.leaguePoints} LP"
                val record = "${ranked.wins}W ${ranked.losses}L"
                Text(text = "${ranked.queueType}: $division · $record", style = MaterialTheme.typography.bodySmall)
            }
        }
    }
}

@Composable
private fun MatchRow(match: MatchListItem) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = if (match.win) WinColor else LossColor),
    ) {
        Column(modifier = Modifier.padding(12.dp)) {
            val score = "${match.championName} · ${match.kills}/${match.deaths}/${match.assists} · KDA ${"%.2f".format(match.kda)}"
            val details = "${TIMESTAMP.format(match.playedAt)} · ${formatDuration(match)} · ${match.cs} CS · ${match.goldEarned} gold"

            Text(text = score, style = MaterialTheme.typography.bodyMedium)
            Text(text = details, style = MaterialTheme.typography.bodySmall, fontFamily = FontFamily.Monospace)
        }
    }
}

// Duration.toMinutesPart появился в Java 9, а minSdk 26 даёт java.time уровня Java 8 —
// поэтому вручную.
private fun formatDuration(match: MatchListItem): String {
    val total = match.duration.seconds
    return "%d:%02d".format(total / 60, total % 60)
}

private val TIMESTAMP: DateTimeFormatter =
    DateTimeFormatter.ofPattern("dd.MM HH:mm").withZone(ZoneId.systemDefault())
