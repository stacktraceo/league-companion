package com.stacktraceo.leaguecompanion.ui.format

import java.time.Duration
import java.util.Locale

// Чистые форматтеры: ничего не знают ни про Compose, ни про ресурсы, поэтому
// проверяются обычными юнит-тестами. Локаль везде задана явно — на устройстве с
// арабской или деванагари локалью Locale.getDefault() дал бы другие цифры, а
// «5/10/9» и «1.40» должны читаться одинаково всюду.

/** Длительность матча как на табло: `42:03`, для часовых игр — `1:05:20`. */
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

/** `5 / 10 / 9` — с пробелами, иначе на карточке слипается в одно число. */
fun formatScore(
    kills: Int,
    deaths: Int,
    assists: Int,
): String = "$kills / $deaths / $assists"

/** KDA приходит готовым от бэкенда, здесь только два знака: `1.40`. */
fun formatKda(kda: Double): String = String.format(Locale.ROOT, "%.2f", kda)

/**
 * Доля `0..1` → проценты. И бэкенд, и доменные модели держат винрейт долей
 * (журнал решений, «День 7»), в проценты его переводит ровно это место.
 */
fun formatWinRate(winRate: Double): String = String.format(Locale.ROOT, "%.0f%%", winRate * PERCENT)

private const val SECONDS_IN_MINUTE = 60
private const val SECONDS_IN_HOUR = 3600
private const val PERCENT = 100
