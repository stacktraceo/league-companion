package com.stacktraceo.leaguecompanion.ui.format

import java.time.Duration
import java.util.Locale

// Чистые форматтеры: ничего не знают ни про Compose, ни про ресурсы, поэтому
// проверяются обычными юнит-тестами. Локаль везде задана явно - на устройстве с
// арабской или деванагари локалью Locale.getDefault() дал бы другие цифры, а
// «5/10/9» и «1.40» должны читаться одинаково всюду.

fun formatDuration(duration: Duration): String {
    val total = duration.seconds
    val hours = total / SECONDS_IN_HOUR
    val minutes = (total % SECONDS_IN_HOUR) / SECONDS_IN_MINUTE
    val seconds = total % SECONDS_IN_MINUTE

    return if (hours > 0) {
        String.format(Locale.ROOT, "%d:%02d:%02d", hours, minutes, seconds)
    } else {
        String.format(Locale.ROOT, "%d:%02d", minutes, seconds)
    }
}

fun formatScore(
    kills: Int,
    deaths: Int,
    assists: Int,
): String = "$kills / $deaths / $assists"

fun formatKda(kda: Double): String = String.format(Locale.ROOT, "%.2f", kda)

fun formatWinRate(winRate: Double): String = String.format(Locale.ROOT, "%.0f%%", winRate * PERCENT)

private const val SECONDS_IN_MINUTE = 60
private const val SECONDS_IN_HOUR = 3600
private const val PERCENT = 100
