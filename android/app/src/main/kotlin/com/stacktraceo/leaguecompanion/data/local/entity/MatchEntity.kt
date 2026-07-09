package com.stacktraceo.leaguecompanion.data.local.entity

import androidx.room.ColumnInfo
import androidx.room.Entity
import androidx.room.Index
import androidx.room.PrimaryKey
import java.time.Instant

/**
 * Матч как таковой — общий для всех участников. Зеркало таблицы `matches`,
 * но без колонки `raw_data`: полный JSON Match-V5 держит бэкенд (CLAUDE.md,
 * отклонение 1), клиенту он не нужен и весит десятки килобайт на матч.
 *
 * Разделение «матч» / «участие» повторяет бэкенд не ради симметрии: один матч
 * может принадлежать нескольким отслеживаемым саммонерам, и плоская таблица под
 * ленту дублировала бы его строку на каждого.
 */
@Entity(
    tableName = "matches",
    indices = [Index(value = ["game_creation"])],
)
data class MatchEntity(
    @PrimaryKey
    @ColumnInfo(name = "match_id")
    val matchId: String,
    @ColumnInfo(name = "game_creation")
    val gameCreation: Instant,
    @ColumnInfo(name = "game_duration")
    val gameDurationSeconds: Int,
    @ColumnInfo(name = "queue_id")
    val queueId: Int,
    @ColumnInfo(name = "game_version")
    val gameVersion: String,
)
