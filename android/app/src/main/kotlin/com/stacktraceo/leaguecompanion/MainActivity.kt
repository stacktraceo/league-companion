package com.stacktraceo.leaguecompanion

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import com.stacktraceo.leaguecompanion.debug.DebugScreen
import com.stacktraceo.leaguecompanion.ui.theme.LeagueCompanionTheme
import dagger.hilt.android.AndroidEntryPoint

@AndroidEntryPoint
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            LeagueCompanionTheme {
                AppScaffold()
            }
        }
    }
}

// Пока единственный экран — отладочный (пакет debug). Навигация появится вместе с
// настоящими экранами в вехе «Дни 11–12», тогда же уедет и он.
@Composable
private fun AppScaffold() {
    Scaffold(modifier = Modifier.fillMaxSize()) { innerPadding ->
        DebugScreen(modifier = Modifier.padding(innerPadding))
    }
}
