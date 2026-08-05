package com.stacktraceo.leaguecompanion.domain.model

data class RiotId(
    val gameName: String,
    val tagLine: String,
) {
    companion object {
        private const val SEPARATOR = '#'

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
