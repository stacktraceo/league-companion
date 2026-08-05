package com.stacktraceo.leaguecompanion.ui.navigation

import kotlinx.serialization.Serializable

@Serializable
data object SearchRoute

@Serializable
data class SummonerRoute(
    val puuid: String,
)

@Serializable
data class MatchRoute(
    val matchId: String,
    val puuid: String,
)

@Serializable
data class StatsRoute(
    val puuid: String,
)
