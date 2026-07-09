package com.stacktraceo.leaguecompanion.data.local.entity

import androidx.room.ColumnInfo
import androidx.room.Entity
import androidx.room.ForeignKey
import androidx.room.Index

/**
 * Участие отслеживаемого саммонера в матче. Зеркало таблицы `match_participants`.
 *
 * Внешние ключи с CASCADE: удалили саммонера — его участия уходят следом, а матч
 * остаётся, пока на него ссылается кто-то ещё. Room включает `PRAGMA foreign_keys`
 * по умолчанию, поэтому строки нужно писать в порядке matches → participants,
 * иначе вставка упадёт на несуществующем `match_id` (см. `MatchDao.upsertPage`).
 */
@Entity(
    tableName = "match_participants",
    primaryKeys = ["match_id", "puuid"],
    foreignKeys = [
        ForeignKey(
            entity = MatchEntity::class,
            parentColumns = ["match_id"],
            childColumns = ["match_id"],
            onDelete = ForeignKey.CASCADE,
        ),
        ForeignKey(
            entity = SummonerEntity::class,
            parentColumns = ["puuid"],
            childColumns = ["puuid"],
            onDelete = ForeignKey.CASCADE,
        ),
    ],
    // match_id — левая колонка составного ключа, её индекс есть; puuid нужен свой,
    // по нему идёт выборка ленты.
    indices = [Index(value = ["puuid"])],
)
data class MatchParticipantEntity(
    @ColumnInfo(name = "match_id")
    val matchId: String,
    @ColumnInfo(name = "puuid")
    val puuid: String,
    @ColumnInfo(name = "champion_name")
    val championName: String,
    @ColumnInfo(name = "kills")
    val kills: Int,
    @ColumnInfo(name = "deaths")
    val deaths: Int,
    @ColumnInfo(name = "assists")
    val assists: Int,
    @ColumnInfo(name = "win")
    val win: Boolean,
    @ColumnInfo(name = "cs")
    val cs: Int,
    @ColumnInfo(name = "gold_earned")
    val goldEarned: Int,
    // Колонки сверх схемы бэкенда: там KDA считается на лету
    // (domain.MatchParticipant.KDA), у нас же значение приходит готовым в DTO.
    // Пересчитывать его на клиенте — значит завести вторую копию правила
    // «при нуле смертей возвращаем K+A», и разойтись с /stats на первом же
    // безупречном матче. Дешевле сохранить то, что прислал бэкенд.
    @ColumnInfo(name = "kda")
    val kda: Double,
)
