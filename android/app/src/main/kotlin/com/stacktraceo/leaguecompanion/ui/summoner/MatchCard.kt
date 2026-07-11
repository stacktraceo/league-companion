package com.stacktraceo.leaguecompanion.ui.summoner

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.stacktraceo.leaguecompanion.R
import com.stacktraceo.leaguecompanion.domain.model.MatchListItem
import com.stacktraceo.leaguecompanion.ui.format.ago
import com.stacktraceo.leaguecompanion.ui.format.asText
import com.stacktraceo.leaguecompanion.ui.format.formatDuration
import com.stacktraceo.leaguecompanion.ui.format.formatKda
import com.stacktraceo.leaguecompanion.ui.format.formatScore
import com.stacktraceo.leaguecompanion.ui.theme.LossColor
import com.stacktraceo.leaguecompanion.ui.theme.WinColor

/**
 * Карточка матча по SPEC.md 4.2: результат цветом, чемпион, KDA, CS, длительность,
 * «сколько прошло».
 *
 * Цвет — не единственный признак исхода: рядом стоит слово Victory/Defeat. Иначе
 * при дальтонизме зелёная и красная карточки различались бы только оттенком, а
 * исход матча — то, что экран обязан сообщать с одного взгляда.
 */
@Composable
fun MatchCard(
    match: MatchListItem,
    modifier: Modifier = Modifier,
) {
    Card(
        modifier = modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = if (match.win) WinColor else LossColor),
    ) {
        Column(
            modifier = Modifier.padding(12.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    text = match.championName,
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.Bold,
                    color = Color.White,
                )
                Text(
                    text = stringResource(if (match.win) R.string.match_victory else R.string.match_defeat),
                    style = MaterialTheme.typography.labelLarge,
                    color = Color.White,
                )
            }

            Text(
                text =
                    stringResource(
                        R.string.match_score,
                        formatScore(match.kills, match.deaths, match.assists),
                        formatKda(match.kda),
                    ),
                style = MaterialTheme.typography.bodyMedium,
                color = Color.White,
            )

            Text(
                text =
                    stringResource(
                        R.string.match_details,
                        formatDuration(match.duration),
                        match.cs,
                        ago(match.playedAt).asText(),
                    ),
                style = MaterialTheme.typography.bodySmall,
                color = Color.White.copy(alpha = SECONDARY_ALPHA),
            )
        }
    }
}

private const val SECONDARY_ALPHA = 0.8f
