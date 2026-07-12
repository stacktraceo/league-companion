package com.stacktraceo.leaguecompanion

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.ui.Modifier
import com.stacktraceo.leaguecompanion.ui.navigation.LeagueNavHost
import com.stacktraceo.leaguecompanion.ui.theme.LeagueCompanionTheme
import dagger.hilt.android.AndroidEntryPoint

@AndroidEntryPoint
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            LeagueCompanionTheme {
                // Scaffold'ы живут в самих экранах: у поиска и у профиля разные
                // topBar'ы, и общий на всё приложение пришлось бы настраивать
                // снаружи по текущему маршруту.
                LeagueNavHost(modifier = Modifier.fillMaxSize())
            }
        }
    }
}
