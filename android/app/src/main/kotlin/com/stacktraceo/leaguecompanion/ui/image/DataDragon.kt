package com.stacktraceo.leaguecompanion.ui.image

/**
 * Ссылки на картинки Data Dragon — единственное место в приложении, которое ходит
 * не к своему бэкенду.
 *
 * Версия зафиксирована константой, а не выводится из `gameVersion` матча. Вывести
 * можно: в базе лежит `16.15.799.6036`, из чего получается `16.15.1`. Но у иконки
 * профиля матча нет вовсе, а квадратики чемпионов от патча к патчу не меняются —
 * разные версии на разных строках ленты дали бы разные URL, разные записи дискового
 * кэша и лишний движущийся кусок ради одинаковых картинок. Старые версии CDN не
 * удаляются, так что константа не протухнет: она просто не будет знать чемпионов,
 * вышедших после неё, а промах по картинке экран не ломает.
 */
object DataDragon {
    const val VERSION = "16.15.1"

    private const val BASE = "https://ddragon.leagueoflegends.com/cdn"

    fun profileIconUrl(profileIconId: Int): String = "$BASE/$VERSION/img/profileicon/$profileIconId.png"

    fun championIconUrl(championName: String): String = "$BASE/$VERSION/img/champion/${championImageId(championName)}.png"

    /**
     * Имя чемпиона из Match-V5 → id картинки в CDN.
     *
     * Совпадает почти всегда, но не всегда, и промах даёт `403`, то есть дыру на
     * месте иконки. Проверено на живом CDN: `FiddleSticks` (как отдаёт Riot) — 403,
     * `Fiddlesticks` — 200; `Wukong` — 403, `MonkeyKing` — 200. У текущего саммонера
     * все двенадцать чемпионов совпадают один в один, поэтому расхождение ловится
     * только тестом, а не глазами.
     */
    fun championImageId(championName: String): String = EXCEPTIONS[championName] ?: championName

    private val EXCEPTIONS =
        mapOf(
            "FiddleSticks" to "Fiddlesticks",
            "Wukong" to "MonkeyKing",
        )
}
