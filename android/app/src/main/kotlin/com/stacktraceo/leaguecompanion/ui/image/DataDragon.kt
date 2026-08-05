package com.stacktraceo.leaguecompanion.ui.image

object DataDragon {
    const val VERSION = "16.15.1"

    private const val BASE = "https://ddragon.leagueoflegends.com/cdn"

    fun profileIconUrl(profileIconId: Int): String = "$BASE/$VERSION/img/profileicon/$profileIconId.png"

    fun championIconUrl(championName: String): String = "$BASE/$VERSION/img/champion/${championImageId(championName)}.png"

    fun championImageId(championName: String): String = EXCEPTIONS[championName] ?: championName

    private val EXCEPTIONS =
        mapOf(
            "FiddleSticks" to "Fiddlesticks",
            "Wukong" to "MonkeyKing",
        )
}
