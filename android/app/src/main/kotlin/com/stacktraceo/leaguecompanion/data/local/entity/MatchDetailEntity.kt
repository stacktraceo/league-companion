package com.stacktraceo.leaguecompanion.data.local.entity

import androidx.room.ColumnInfo
import androidx.room.Entity
import androidx.room.PrimaryKey
import java.time.Instant

@Entity(tableName = "match_details")
data class MatchDetailEntity(
    @PrimaryKey
    @ColumnInfo(name = "match_id")
    val matchId: String,
    @ColumnInfo(name = "raw_data")
    val rawData: String,
    @ColumnInfo(name = "fetched_at")
    val fetchedAt: Instant,
)
