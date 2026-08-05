package com.stacktraceo.leaguecompanion.ui.format

import androidx.compose.runtime.Composable
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import com.stacktraceo.leaguecompanion.R
import java.time.Duration
import java.time.Instant

sealed interface Ago {
    data object JustNow : Ago

    data class Minutes(
        val value: Int,
    ) : Ago

    data class Hours(
        val value: Int,
    ) : Ago

    data class Days(
        val value: Int,
    ) : Ago
}

fun ago(
    moment: Instant,
    now: Instant = Instant.now(),
): Ago {
    val elapsed = Duration.between(moment, now)

    // Отрицательная разница - не выдумка: часы устройства могут отставать от бэкенда,
    // и матч «из будущего» должен читаться как только что, а не как «-3 минуты назад».
    if (elapsed.isNegative || elapsed.toMinutes() < 1) return Ago.JustNow

    val minutes = elapsed.toMinutes()
    if (minutes < MINUTES_IN_HOUR) return Ago.Minutes(minutes.toInt())

    val hours = elapsed.toHours()
    if (hours < HOURS_IN_DAY) return Ago.Hours(hours.toInt())

    return Ago.Days(elapsed.toDays().toInt())
}

@Composable
fun Ago.asText(): String =
    when (this) {
        Ago.JustNow -> stringResource(R.string.ago_just_now)
        is Ago.Minutes -> pluralStringResource(R.plurals.ago_minutes, value, value)
        is Ago.Hours -> pluralStringResource(R.plurals.ago_hours, value, value)
        is Ago.Days -> pluralStringResource(R.plurals.ago_days, value, value)
    }

private const val MINUTES_IN_HOUR = 60
private const val HOURS_IN_DAY = 24
