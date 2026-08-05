package com.stacktraceo.leaguecompanion.data.local.view

import androidx.room.ColumnInfo
import java.time.Instant

data class MatchListItemView(
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
    @ColumnInfo(name = "champion_name")
    val championName: String,
    @ColumnInfo(name = "kills")
    val kills: Int,
    @ColumnInfo(name = "deaths")
    val deaths: Int,
    @ColumnInfo(name = "assists")
    val assists: Int,
    @ColumnInfo(name = "kda")
    val kda: Double,
    @ColumnInfo(name = "win")
    val win: Boolean,
    @ColumnInfo(name = "cs")
    val cs: Int,
    @ColumnInfo(name = "gold_earned")
    val goldEarned: Int,
)
