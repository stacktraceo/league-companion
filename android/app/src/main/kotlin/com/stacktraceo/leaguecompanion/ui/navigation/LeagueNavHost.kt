package com.stacktraceo.leaguecompanion.ui.navigation

import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.stacktraceo.leaguecompanion.ui.search.SearchScreen
import com.stacktraceo.leaguecompanion.ui.summoner.SummonerScreen

/**
 * Стартовый экран — поиск, и он же список отслеживаемых.
 *
 * Иначе после перезапуска к сохранённому саммонеру нет пути: экран профиля знает
 * только свой puuid, а взять его неоткуда. Список уже отслеживаемых на экране
 * поиска решает это без отдельного «главного» экрана.
 */
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

        composable<SummonerRoute> {
            SummonerScreen(onBack = { navController.popBackStack() })
        }
    }
}
