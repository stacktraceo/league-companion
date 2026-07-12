package com.stacktraceo.leaguecompanion.ui.search

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.stacktraceo.leaguecompanion.R
import com.stacktraceo.leaguecompanion.domain.model.Region
import com.stacktraceo.leaguecompanion.domain.model.Summoner
import com.stacktraceo.leaguecompanion.ui.components.EmptyState
import com.stacktraceo.leaguecompanion.ui.error.asText

/** Экран поиска по SPEC.md 4.2, пункт 1, плюс список уже отслеживаемых саммонеров. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SearchScreen(
    onOpenSummoner: (String) -> Unit,
    modifier: Modifier = Modifier,
    viewModel: SearchViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()

    LaunchedEffect(Unit) {
        viewModel.opened.collect(onOpenSummoner)
    }

    Scaffold(
        modifier = modifier.fillMaxSize(),
        topBar = { TopAppBar(title = { Text(stringResource(R.string.app_name)) }) },
    ) { innerPadding ->
        LazyColumn(
            modifier = Modifier.fillMaxSize().padding(innerPadding).padding(horizontal = 16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
            contentPadding = PaddingValues(vertical = 16.dp),
        ) {
            item(key = "form") {
                SearchForm(
                    state = state,
                    onInputChange = viewModel::onInputChange,
                    onRegionChange = viewModel::onRegionChange,
                    onSubmit = viewModel::submit,
                )
            }

            if (state.tracked.isEmpty()) {
                item(key = "empty") { EmptyState(message = stringResource(R.string.search_empty)) }
            } else {
                item(key = "tracked") {
                    Text(
                        text = stringResource(R.string.search_tracked),
                        style = MaterialTheme.typography.titleMedium,
                    )
                }
                items(items = state.tracked, key = { it.puuid }) { summoner ->
                    TrackedRow(summoner = summoner, onClick = { onOpenSummoner(summoner.puuid) })
                }
            }
        }
    }
}

@Composable
private fun SearchForm(
    state: SearchUiState,
    onInputChange: (String) -> Unit,
    onRegionChange: (Region) -> Unit,
    onSubmit: () -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        OutlinedTextField(
            value = state.input,
            onValueChange = onInputChange,
            label = { Text(stringResource(R.string.search_riot_id)) },
            placeholder = { Text(stringResource(R.string.search_riot_id_hint)) },
            singleLine = true,
            isError = state.error != null,
            modifier = Modifier.fillMaxWidth(),
        )

        Row(
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            RegionPicker(selected = state.region, onSelect = onRegionChange)

            Button(onClick = onSubmit, enabled = state.canSubmit) {
                if (state.submitting) {
                    // Индикатор внутри кнопки, а не поверх экрана: форма остаётся
                    // видимой, и понятно, что именно сейчас делается.
                    CircularProgressIndicator(
                        modifier = Modifier.size(18.dp),
                        strokeWidth = 2.dp,
                        color = MaterialTheme.colorScheme.onPrimary,
                    )
                } else {
                    Text(stringResource(R.string.search_action))
                }
            }
        }

        state.error?.let { error ->
            Text(
                text = error.asText(),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.error,
            )
        }
    }
}

@Composable
private fun SearchError.asText(): String =
    when (this) {
        SearchError.BadFormat -> stringResource(R.string.search_bad_format)
        is SearchError.Failed -> error.asText()
    }

@Composable
private fun RegionPicker(
    selected: Region,
    onSelect: (Region) -> Unit,
) {
    var expanded by remember { mutableStateOf(false) }

    Box {
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
private fun TrackedRow(
    summoner: Summoner,
    onClick: () -> Unit,
) {
    Card(onClick = onClick, modifier = Modifier.fillMaxWidth()) {
        Column(
            modifier = Modifier.padding(12.dp),
            verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            Text(text = summoner.displayName, style = MaterialTheme.typography.titleMedium)
            Text(
                text = stringResource(R.string.profile_region_level, summoner.region.uppercase(), summoner.level),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}
