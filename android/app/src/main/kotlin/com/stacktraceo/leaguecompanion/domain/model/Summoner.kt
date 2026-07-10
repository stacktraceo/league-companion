package com.stacktraceo.leaguecompanion.domain.model

import java.time.Instant

/**
 * Профиль отслеживаемого саммонера — то, что видит UI.
 *
 * `region` остаётся строкой, а не [Region]: платформа приходит с бэкенда, и если
 * Riot заведёт новую (а такое случается), enum превратил бы её в исключение при
 * чтении собственного кэша. [Region] нужен там, где выбор ограничен нами самими —
 * в списке на экране добавления.
 */
data class Summoner(
    val puuid: String,
    val riotId: String,
    val tagLine: String,
    val region: String,
    val level: Int,
    val profileIconId: Int,
    /** null — синхронизация ещё ни разу не доходила до конца; лента при этом пуста. */
    val lastSyncedAt: Instant?,
    val ranked: List<RankedStat>,
) {
    val displayName: String get() = "$riotId#$tagLine"
}
