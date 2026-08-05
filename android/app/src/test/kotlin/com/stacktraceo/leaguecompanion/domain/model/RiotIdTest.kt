package com.stacktraceo.leaguecompanion.domain.model

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class RiotIdTest {
    @Test
    fun `разбирает обычный Riot ID`() {
        assertEquals(RiotId("Faker", "KR1"), RiotId.parse("Faker#KR1"))
    }

    @Test
    fun `пробелы внутри имени сохраняются`() {
        // «Hide on bush» - валидный GameName. Схлопни мы пробелы, бэкенд вернул бы 404
        // на существующего саммонера.
        assertEquals(RiotId("Hide on bush", "KR1"), RiotId.parse("  Hide on bush # KR1 "))
    }

    @Test
    fun `без решётки - не Riot ID`() {
        assertNull(RiotId.parse("Faker"))
    }

    @Test
    fun `пустая половина - не Riot ID`() {
        assertNull(RiotId.parse("Faker#"))
        assertNull(RiotId.parse("#KR1"))
        assertNull(RiotId.parse("#"))
    }

    @Test
    fun `две решётки - не Riot ID`() {
        // Тег не может содержать решётку, поэтому «Faker#KR1#2» - опечатка, а не
        // имя с тегом «KR1#2».
        assertNull(RiotId.parse("Faker#KR1#2"))
    }

    @Test
    fun `пустой ввод не разбирается`() {
        assertNull(RiotId.parse(""))
        assertNull(RiotId.parse("   "))
    }
}
