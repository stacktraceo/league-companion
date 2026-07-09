package com.stacktraceo.leaguecompanion.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

// Цвета клиента League: золото и бирюза на тёмно-синем.
private val Gold = Color(0xFFC8AA6E)
private val Teal = Color(0xFF0AC8B9)
private val Navy = Color(0xFF101820)

// Победа и поражение — единственная пара, которую экран матчей обязан различать
// с одного взгляда (SPEC.md 4.2, пункт 3).
val WinColor = Color(0xFF2E7D5B)
val LossColor = Color(0xFF9B3B3B)

private val DarkColors =
    darkColorScheme(
        primary = Gold,
        secondary = Teal,
        background = Navy,
        surface = Color(0xFF1A2430),
    )

private val LightColors =
    lightColorScheme(
        primary = Color(0xFF7A5C1E),
        secondary = Color(0xFF00695C),
    )

@Composable
fun LeagueCompanionTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit,
) {
    MaterialTheme(
        colorScheme = if (darkTheme) DarkColors else LightColors,
        content = content,
    )
}
