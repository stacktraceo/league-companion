package com.stacktraceo.leaguecompanion.ui.summoner

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Card
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.stacktraceo.leaguecompanion.R
import com.stacktraceo.leaguecompanion.domain.model.RankedStat
import com.stacktraceo.leaguecompanion.domain.model.Summoner
import com.stacktraceo.leaguecompanion.ui.format.formatWinRate
import com.stacktraceo.leaguecompanion.ui.image.DataDragon
import com.stacktraceo.leaguecompanion.ui.image.RemoteIcon

/** Профиль по SPEC.md 4.2: уровень, ранг (тир + LP), общий W/L. */
@Composable
fun ProfileHeader(
    summoner: Summoner,
    modifier: Modifier = Modifier,
) {
    Card(modifier = modifier.fillMaxWidth()) {
        Column(
            modifier = Modifier.padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Row(
                horizontalArrangement = Arrangement.spacedBy(12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                RemoteIcon(
                    url = DataDragon.profileIconUrl(summoner.profileIconId),
                    contentDescription = stringResource(R.string.profile_icon_description),
                    modifier = Modifier.size(PROFILE_ICON_SIZE),
                    shape = CircleShape,
                )

                Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Text(text = summoner.displayName, style = MaterialTheme.typography.headlineSmall)
                    Text(
                        text = stringResource(R.string.profile_region_level, summoner.region.uppercase(), summoner.level),
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }

            if (summoner.ranked.isEmpty()) {
                // Неранкед — это не ошибка и не пустой экран: у саммонера просто нет
                // рейтинговых игр в текущем сезоне.
                Text(
                    text = stringResource(R.string.profile_unranked),
                    style = MaterialTheme.typography.bodyMedium,
                )
            } else {
                summoner.ranked.forEach { RankedRow(it) }
            }
        }
    }
}

@Composable
private fun RankedRow(ranked: RankedStat) {
    Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Text(
            text = stringResource(R.string.profile_queue_rank, queueLabel(ranked.queueType), ranked.tier, ranked.rank, ranked.leaguePoints),
            style = MaterialTheme.typography.titleMedium,
        )
        Text(
            text = stringResource(R.string.profile_record, ranked.wins, ranked.losses, formatWinRate(ranked.winRate)),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

/**
 * Очереди Riot приходят машинными именами (`RANKED_SOLO_5x5`). Незнакомую отдаём
 * как есть: Riot заводит новые очереди, и показать её сырым именем лучше, чем
 * спрятать строку целиком.
 */
@Composable
private fun queueLabel(queueType: String): String =
    when (queueType) {
        "RANKED_SOLO_5x5" -> stringResource(R.string.queue_solo)
        "RANKED_FLEX_SR" -> stringResource(R.string.queue_flex)
        else -> queueType
    }

private val PROFILE_ICON_SIZE = 56.dp
