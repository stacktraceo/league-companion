package com.stacktraceo.leaguecompanion.ui.navigation

import kotlinx.serialization.Serializable

/**
 * Маршруты — типы, а не строки.
 *
 * `puuid` от Riot — 78 символов base64url. В строковом маршруте вида
 * `"summoner/$puuid"` его пришлось бы кодировать руками, и первый же `/` или `-`
 * ломал бы навигацию молча, уже в рантайме. Типобезопасные маршруты
 * navigation-compose 2.9 делают кодирование сами, а опечатка в имени аргумента
 * становится ошибкой компиляции.
 */
@Serializable
data object SearchRoute

@Serializable
data class SummonerRoute(
    val puuid: String,
)
