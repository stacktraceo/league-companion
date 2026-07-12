package com.stacktraceo.leaguecompanion.domain.model

/**
 * Riot ID в том виде, в каком его показывает игра: `GameName#TAG`.
 *
 * Бэкенд принимает две отдельные части (`riotId` и `tagLine`), но вводить их в два
 * поля неудобно: ник копируют целиком — из клиента, из чата, из профиля на сайте.
 * Разбор живёт здесь, а не в экране, потому что это правило, а не оформление, и
 * проверяется обычным тестом.
 */
data class RiotId(
    val gameName: String,
    val tagLine: String,
) {
    companion object {
        private const val SEPARATOR = '#'

        /**
         * `null` — строка не похожа на Riot ID. Пробелы внутри имени сохраняются:
         * «Hide on bush» — валидный GameName, обрезаются только края.
         */
        fun parse(input: String): RiotId? {
            val parts = input.trim().split(SEPARATOR)
            if (parts.size != 2) return null

            val gameName = parts[0].trim()
            val tagLine = parts[1].trim()
            if (gameName.isEmpty() || tagLine.isEmpty()) return null

            return RiotId(gameName = gameName, tagLine = tagLine)
        }
    }
}
