package com.stacktraceo.leaguecompanion.data.local

import androidx.room.Dao
import androidx.room.Query
import androidx.room.Upsert
import com.stacktraceo.leaguecompanion.data.local.entity.MatchDetailEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface MatchDetailDao {
    @Query("SELECT * FROM match_details WHERE match_id = :matchId")
    fun observe(matchId: String): Flow<MatchDetailEntity?>

    @Upsert
    suspend fun upsert(detail: MatchDetailEntity)
}
