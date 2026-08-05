package com.stacktraceo.leaguecompanion.data.local

import androidx.room.Dao
import androidx.room.Query
import androidx.room.Transaction
import androidx.room.Upsert
import com.stacktraceo.leaguecompanion.data.local.entity.RankedStatEntity
import com.stacktraceo.leaguecompanion.data.local.entity.SummonerEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface SummonerDao {
    @Query("SELECT * FROM summoners WHERE puuid = :puuid")
    fun observe(puuid: String): Flow<SummonerEntity?>

    @Query("SELECT * FROM summoners ORDER BY riot_id COLLATE NOCASE, tag_line COLLATE NOCASE")
    fun observeAll(): Flow<List<SummonerEntity>>

    @Query("SELECT * FROM ranked_stats WHERE puuid = :puuid ORDER BY queue_type")
    fun observeRanked(puuid: String): Flow<List<RankedStatEntity>>

    @Upsert
    suspend fun upsert(summoner: SummonerEntity)

    @Upsert
    suspend fun upsertRanked(rows: List<RankedStatEntity>)

    @Query("DELETE FROM ranked_stats WHERE puuid = :puuid")
    suspend fun deleteRanked(puuid: String)

    @Transaction
    suspend fun upsertWithRanked(
        summoner: SummonerEntity,
        ranked: List<RankedStatEntity>,
    ) {
        upsert(summoner)
        deleteRanked(summoner.puuid)
        upsertRanked(ranked)
    }

    @Query("DELETE FROM summoners WHERE puuid = :puuid")
    suspend fun delete(puuid: String)
}
