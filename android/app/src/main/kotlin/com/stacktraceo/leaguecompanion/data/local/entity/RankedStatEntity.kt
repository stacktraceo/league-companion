package com.stacktraceo.leaguecompanion.data.local.entity

import androidx.room.ColumnInfo
import androidx.room.Entity
import androidx.room.ForeignKey
import java.time.Instant

@Entity(
    tableName = "ranked_stats",
    primaryKeys = ["puuid", "queue_type"],
    foreignKeys = [
        ForeignKey(
            entity = SummonerEntity::class,
            parentColumns = ["puuid"],
            childColumns = ["puuid"],
            onDelete = ForeignKey.CASCADE,
        ),
    ],
)
data class RankedStatEntity(
    @ColumnInfo(name = "puuid")
    val puuid: String,
    // RANKED_SOLO_5x5, RANKED_FLEX_SR, ...
    @ColumnInfo(name = "queue_type")
    val queueType: String,
    @ColumnInfo(name = "tier")
    val tier: String,
    @ColumnInfo(name = "rank")
    val rank: String,
    @ColumnInfo(name = "league_points")
    val leaguePoints: Int,
    @ColumnInfo(name = "wins")
    val wins: Int,
    @ColumnInfo(name = "losses")
    val losses: Int,
    @ColumnInfo(name = "updated_at")
    val updatedAt: Instant,
)
