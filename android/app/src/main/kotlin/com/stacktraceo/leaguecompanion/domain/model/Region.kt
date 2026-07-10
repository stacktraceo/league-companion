package com.stacktraceo.leaguecompanion.domain.model

/**
 * Платформы, которые принимает бэкенд.
 *
 * Список — зеркало `platformRouting` из backend/internal/riot/routing.go. Всё, чего
 * там нет, вернётся как `400 invalid_region`, поэтому экран добавления саммонера
 * даёт выбирать только отсюда, а не принимает произвольную строку.
 */
enum class Region(
    val code: String,
    val label: String,
) {
    BR1("br1", "Brazil"),
    LA1("la1", "LAN"),
    LA2("la2", "LAS"),
    NA1("na1", "North America"),
    EUN1("eun1", "EU Nordic & East"),
    EUW1("euw1", "EU West"),
    ME1("me1", "Middle East"),
    RU("ru", "Russia"),
    TR1("tr1", "Türkiye"),
    JP1("jp1", "Japan"),
    KR("kr", "Korea"),
    OC1("oc1", "Oceania"),
    PH2("ph2", "Philippines"),
    SG2("sg2", "Singapore"),
    TH2("th2", "Thailand"),
    TW2("tw2", "Taiwan"),
    VN2("vn2", "Vietnam"),
    ;

    companion object {
        val DEFAULT = RU

        fun fromCode(code: String): Region? = entries.firstOrNull { it.code.equals(code, ignoreCase = true) }
    }
}
