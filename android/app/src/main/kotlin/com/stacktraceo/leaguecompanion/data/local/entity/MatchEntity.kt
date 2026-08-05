package com.stacktraceo.leaguecompanion.data.local.entity

import androidx.room.ColumnInfo
import androidx.room.Entity
import androidx.room.Index
import androidx.room.PrimaryKey
import java.time.Instant

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
