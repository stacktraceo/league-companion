package com.stacktraceo.leaguecompanion.ui.format

import org.junit.Assert.assertEquals
import org.junit.Test
import java.time.Duration

class FormatsTest {
    @Test
    fun `длительность выводится как на табло`() {
        assertEquals("42:03", formatDuration(Duration.ofSeconds(2523)))
        assertEquals("0:07", formatDuration(Duration.ofSeconds(7)))
    }

    @Test
    fun `часовой матч не превращается в 65 минут`() {
        assertEquals("1:05:20", formatDuration(Duration.ofSeconds(3920)))
    }

    @Test
    fun `нулевая длительность не ломает формат`() {
        // Ремейк на первых минутах приходит с нулём - карточка обязана отрисоваться.
        assertEquals("0:00", formatDuration(Duration.ZERO))
    }

    @Test
    fun `KDA - два знака`() {
        assertEquals("1.40", formatKda(1.4))
        assertEquals("10.00", formatKda(10.0))
        // Перфект-KDA бэкенд отдаёт суммой K+A, без деления - округлять её нечем.
        assertEquals("14.00", formatKda(14.0))
    }

    @Test
    fun `винрейт из доли превращается в проценты`() {
        assertEquals("54%", formatWinRate(0.5432))
        assertEquals("0%", formatWinRate(0.0))
        assertEquals("100%", formatWinRate(1.0))
    }

    @Test
    fun `счёт разделён пробелами`() {
        assertEquals("5 / 10 / 9", formatScore(5, 10, 9))
    }
}
