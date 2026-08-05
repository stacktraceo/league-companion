package com.stacktraceo.leaguecompanion.ui.navigation

import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.toRoute
import com.stacktraceo.leaguecompanion.ui.match.MatchDetailScreen
import com.stacktraceo.leaguecompanion.ui.search.SearchScreen
import com.stacktraceo.leaguecompanion.ui.stats.StatsScreen
import com.stacktraceo.leaguecompanion.ui.summoner.SummonerScreen

@Composable
fun LeagueNavHost(
    modifier: Modifier = Modifier,
    navController: NavHostController = rememberNavController(),
) {
    NavHost(
        navController = navController,
        startDestination = SearchRoute,
        modifier = modifier,
    ) {
        composable<SearchRoute> {
            SearchScreen(onOpenSummoner = { puuid -> navController.navigate(SummonerRoute(puuid)) })
        }

        composable<SummonerRoute> { entry ->
            // Здесь toRoute() уместен: это композиция, а не ViewModel, и Bundle
            // на устройстве есть. В ViewModel'ях аргументы читаются по имени
            // свойства - иначе их нельзя было бы проверить JVM-тестом.
            val puuid = entry.toRoute<SummonerRoute>().puuid

            SummonerScreen(
                onBack = { navController.popBackStack() },
                onOpenMatch = { matchId -> navController.navigate(MatchRoute(matchId = matchId, puuid = puuid)) },
                onOpenStats = { navController.navigate(StatsRoute(puuid)) },
            )
        }

        composable<MatchRoute> {
            MatchDetailScreen(onBack = { navController.popBackStack() })
        }

        composable<StatsRoute> {
            StatsScreen(onBack = { navController.popBackStack() })
        }
    }
}
