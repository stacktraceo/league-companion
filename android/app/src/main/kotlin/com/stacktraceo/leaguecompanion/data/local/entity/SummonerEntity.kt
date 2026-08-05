package com.stacktraceo.leaguecompanion.data.local.entity

import androidx.room.ColumnInfo
import androidx.room.Entity
import androidx.room.PrimaryKey
import java.time.Instant

@Entity(tableName = "summoners")
data class SummonerEntity(
    @PrimaryKey
    @ColumnInfo(name = "puuid")
    val puuid: String,
    // GameName из Riot ID; тег хранится отдельно, потому что бэкенд принимает их
    // раздельно в POST /summoners.
    @ColumnInfo(name = "riot_id")
    val riotId: String,
    @ColumnInfo(name = "tag_line")
    val tagLine: String,
    @ColumnInfo(name = "region")
    val region: String,
    @ColumnInfo(name = "summoner_level")
    val summonerLevel: Int,
    @ColumnInfo(name = "profile_icon_id")
    val profileIconId: Int,
    // null до первой успешной фоновой синхронизации: POST /summoners отвечает 201
    // сразу, а матчи догружаются позже (журнал решений, «Дни 5–6»).
    @ColumnInfo(name = "last_synced_at")
    val lastSyncedAt: Instant?,
    @ColumnInfo(name = "created_at")
    val createdAt: Instant,
)
