package com.stacktraceo.leaguecompanion.data.local

import androidx.room.Dao
import androidx.room.Query
import androidx.room.Transaction
import androidx.room.Upsert
import com.stacktraceo.leaguecompanion.data.local.entity.MatchEntity
import com.stacktraceo.leaguecompanion.data.local.entity.MatchParticipantEntity
import com.stacktraceo.leaguecompanion.data.local.view.MatchListItemView
import kotlinx.coroutines.flow.Flow

@Dao
interface MatchDao {
    @Query(
        """
        SELECT m.match_id, m.game_creation, m.game_duration, m.queue_id, m.game_version,
               p.champion_name, p.kills, p.deaths, p.assists, p.kda, p.win, p.cs, p.gold_earned
        FROM match_participants AS p
        INNER JOIN matches AS m ON m.match_id = p.match_id
        WHERE p.puuid = :puuid
        ORDER BY m.game_creation DESC, m.match_id DESC
        LIMIT :limit
        """,
    )
    fun observeFeed(
        puuid: String,
        limit: Int,
    ): Flow<List<MatchListItemView>>

    @Query("SELECT COUNT(*) FROM match_participants WHERE puuid = :puuid")
    suspend fun cachedCount(puuid: String): Int

    @Upsert
    suspend fun upsertMatches(matches: List<MatchEntity>)

    @Upsert
    suspend fun upsertParticipants(participants: List<MatchParticipantEntity>)

    @Transaction
    suspend fun upsertPage(
        matches: List<MatchEntity>,
        participants: List<MatchParticipantEntity>,
    ) {
        upsertMatches(matches)
        upsertParticipants(participants)
    }
}
