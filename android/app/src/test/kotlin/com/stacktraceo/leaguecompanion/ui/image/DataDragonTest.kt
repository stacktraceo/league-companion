package com.stacktraceo.leaguecompanion.ui.image

import org.junit.Assert.assertEquals
import org.junit.Test

class DataDragonTest {
    @Test
    fun `имя чемпиона обычно совпадает с id картинки`() {
        assertEquals("Yone", DataDragon.championImageId("Yone"))
        assertEquals("TwistedFate", DataDragon.championImageId("TwistedFate"))
    }

    @Test
    fun `FiddleSticks от Riot и Fiddlesticks в CDN различаются регистром`() {
        // Проверено на живом CDN: FiddleSticks.png - 403, Fiddlesticks.png - 200.
        assertEquals("Fiddlesticks", DataDragon.championImageId("FiddleSticks"))
    }

    @Test
    fun `Wukong в CDN зовётся MonkeyKing`() {
        assertEquals("MonkeyKing", DataDragon.championImageId("Wukong"))
    }

    @Test
    fun `незнакомое имя отдаётся как есть`() {
        // Riot выпускает чемпионов чаще, чем обновляется этот список. Промах даст
        // заглушку вместо иконки - это лучше, чем пустая строка в URL.
        assertEquals("НовыйЧемпион", DataDragon.championImageId("НовыйЧемпион"))
    }

    @Test
    fun `URL иконки чемпиона собирается через id, а не через имя`() {
        assertEquals(
            "https://ddragon.leagueoflegends.com/cdn/${DataDragon.VERSION}/img/champion/MonkeyKing.png",
            DataDragon.championIconUrl("Wukong"),
        )
    }

    @Test
    fun `URL иконки профиля собирается по её номеру`() {
        assertEquals(
            "https://ddragon.leagueoflegends.com/cdn/${DataDragon.VERSION}/img/profileicon/6.png",
            DataDragon.profileIconUrl(6),
        )
    }
}
