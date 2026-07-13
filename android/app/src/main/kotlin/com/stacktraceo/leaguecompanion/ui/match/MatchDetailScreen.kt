package com.stacktraceo.leaguecompanion.ui.match

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.stacktraceo.leaguecompanion.R
import com.stacktraceo.leaguecompanion.domain.model.MatchPlayer
import com.stacktraceo.leaguecompanion.domain.model.TeamSide
import com.stacktraceo.leaguecompanion.ui.components.ErrorState
import com.stacktraceo.leaguecompanion.ui.components.LoadingBox
import com.stacktraceo.leaguecompanion.ui.error.asText
import com.stacktraceo.leaguecompanion.ui.format.formatDuration
import com.stacktraceo.leaguecompanion.ui.format.formatKda
import com.stacktraceo.leaguecompanion.ui.format.formatScore
import com.stacktraceo.leaguecompanion.ui.theme.LossColor
import com.stacktraceo.leaguecompanion.ui.theme.WinColor

/** Детали матча: обе команды и все десять участников (SPEC.md 4.2, пункт 4). */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MatchDetailScreen(
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
    viewModel: MatchDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsStateWithLifecycle()
    val snackbarHostState = remember { SnackbarHostState() }
    val errorText = state.error?.asText()

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
                title = { Text(state.detail?.let { formatDuration(it.duration) } ?: stringResource(R.string.match_title)) },
                navigationIcon = {
                    TextButton(onClick = onBack) { Text(stringResource(R.string.action_back)) }
                },
            )
        },
    ) { innerPadding ->
        Box(modifier = Modifier.padding(innerPadding)) {
            when {
                !state.hasContent && state.firstLoad -> LoadingBox()

                !state.hasContent && errorText != null ->
                    ErrorState(message = errorText, onRetry = viewModel::refresh)

                else ->
                    LazyColumn(
                        modifier = Modifier.fillMaxSize().padding(horizontal = 16.dp),
                        verticalArrangement = Arrangement.spacedBy(12.dp),
                        contentPadding = PaddingValues(vertical = 16.dp),
                    ) {
                        state.detail?.teams?.forEach { team ->
                            item(key = "team-${team.teamId}") { TeamHeader(team) }
                            playerRows(team)
                        }
                    }
            }
        }
    }
}

private fun LazyListScope.playerRows(team: TeamSide) {
    team.players.forEach { player ->
        item(key = "${team.teamId}-${player.puuid}") { PlayerRow(player) }
    }
}

@Composable
private fun TeamHeader(team: TeamSide) {
    Row(
        modifier =
            Modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(8.dp))
                .background(if (team.win) WinColor else LossColor)
                .padding(12.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = stringResource(if (team.win) R.string.match_victory else R.string.match_defeat),
            style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold,
            color = Color.White,
        )
        val objectives =
            listOf(
                pluralStringResource(R.plurals.match_kills, team.kills, team.kills),
                pluralStringResource(R.plurals.match_towers, team.towers, team.towers),
                pluralStringResource(R.plurals.match_dragons, team.dragons, team.dragons),
                pluralStringResource(R.plurals.match_barons, team.barons, team.barons),
            ).joinToString(" · ")

        Text(
            text = objectives,
            style = MaterialTheme.typography.bodySmall,
            color = Color.White,
        )
    }
}

@Composable
private fun PlayerRow(player: MatchPlayer) {
    // Свой участник выделяется фоном: на экране из десяти строк иначе себя не найти.
    val background =
        if (player.tracked) {
            MaterialTheme.colorScheme.primary.copy(alpha = TRACKED_ALPHA)
        } else {
            Color.Transparent
        }

    Column(
        modifier =
            Modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(8.dp))
                .background(background)
                .padding(horizontal = 8.dp, vertical = 6.dp),
        verticalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            Text(
                text = "${player.championName} · ${player.displayName}",
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = if (player.tracked) FontWeight.Bold else FontWeight.Normal,
            )
            Text(
                text = stringResource(R.string.match_player_level, player.level),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }

        Text(
            text =
                stringResource(
                    R.string.match_player_line,
                    formatScore(player.kills, player.deaths, player.assists),
                    formatKda(player.kda),
                    player.cs,
                    player.gold,
                ),
            style = MaterialTheme.typography.bodySmall,
            fontFamily = FontFamily.Monospace,
        )

        Text(
            text = stringResource(R.string.match_player_impact, player.damage, player.visionScore),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            fontFamily = FontFamily.Monospace,
        )
    }
}

private const val TRACKED_ALPHA = 0.22f
