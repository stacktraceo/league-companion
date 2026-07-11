package com.stacktraceo.leaguecompanion.ui.format

import org.junit.Assert.assertEquals
import org.junit.Test
import java.time.Instant

class RelativeTimeTest {
    private val now = Instant.parse("2026-07-11T12:00:00Z")

    @Test
    fun `меньше минуты — только что`() {
        assertEquals(Ago.JustNow, ago(now.minusSeconds(59), now))
    }

    @Test
    fun `ровно минута уже считается минутой`() {
        assertEquals(Ago.Minutes(1), ago(now.minusSeconds(60), now))
    }

    @Test
    fun `последняя минута перед часом остаётся минутами`() {
        assertEquals(Ago.Minutes(59), ago(now.minusSeconds(59 * 60), now))
    }

    @Test
    fun `час переключает единицу`() {
        assertEquals(Ago.Hours(1), ago(now.minusSeconds(60 * 60), now))
        assertEquals(Ago.Hours(23), ago(now.minusSeconds(23 * 60 * 60), now))
    }

    @Test
    fun `сутки переключают на дни`() {
        assertEquals(Ago.Days(1), ago(now.minusSeconds(24 * 60 * 60), now))
        assertEquals(Ago.Days(30), ago(now.minusSeconds(30L * 24 * 60 * 60), now))
    }

    @Test
    fun `матч из будущего читается как только что`() {
        // Часы устройства могут отставать от бэкенда. «-3 минуты назад» на карточке
        // выглядело бы поломкой, хотя данные в порядке.
        assertEquals(Ago.JustNow, ago(now.plusSeconds(5 * 60), now))
    }

    @Test
    fun `неполная единица округляется вниз, а не вверх`() {
        // 90 минут — это «1 час назад», а не «2 часа»: завышать давность нельзя,
        // иначе только что сыгранный матч уезжает в прошлое.
        assertEquals(Ago.Hours(1), ago(now.minusSeconds(90 * 60), now))
    }
}
