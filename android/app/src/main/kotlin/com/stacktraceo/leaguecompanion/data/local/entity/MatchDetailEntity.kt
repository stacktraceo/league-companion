package com.stacktraceo.leaguecompanion.data.local.entity

import androidx.room.ColumnInfo
import androidx.room.Entity
import androidx.room.PrimaryKey
import java.time.Instant

/**
 * Детали просмотренного матча — сырым JSON, а не разложенные по колонкам.
 *
 * Та же логика, что у `matches.raw_data` на бэкенде (CLAUDE.md, отклонение 1):
 * в ответе 155 полей на участника, и набор показываемых ещё будет меняться —
 * предметы, руны, спеллы уже приезжают. Раскладка по колонкам означала бы миграцию
 * на каждое такое изменение, а так добавляется только поле в DTO.
 *
 * Таблица намеренно **не** ссылается на `matches`: детали кладутся по факту
 * открытия экрана, и FK заставил бы держать порядок «сначала лента, потом деталь»
 * там, где он не нужен.
 */
@Entity(tableName = "match_details")
data class MatchDetailEntity(
    @PrimaryKey
    @ColumnInfo(name = "match_id")
    val matchId: String,
    @ColumnInfo(name = "raw_data")
    val rawData: String,
    /** Когда положили — по нему чистить кэш, когда он начнёт мешать. */
    @ColumnInfo(name = "fetched_at")
    val fetchedAt: Instant,
)
