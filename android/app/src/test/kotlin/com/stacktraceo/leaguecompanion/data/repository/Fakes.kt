package com.stacktraceo.leaguecompanion.data.repository

import com.stacktraceo.leaguecompanion.data.local.MatchDao
import com.stacktraceo.leaguecompanion.data.local.MatchDetailDao
import com.stacktraceo.leaguecompanion.data.local.SummonerDao
import com.stacktraceo.leaguecompanion.data.local.entity.MatchDetailEntity
import com.stacktraceo.leaguecompanion.data.local.entity.MatchEntity
import com.stacktraceo.leaguecompanion.data.local.entity.MatchParticipantEntity
import com.stacktraceo.leaguecompanion.data.local.entity.RankedStatEntity
import com.stacktraceo.leaguecompanion.data.local.entity.SummonerEntity
import com.stacktraceo.leaguecompanion.data.local.view.MatchListItemView
import com.stacktraceo.leaguecompanion.data.remote.LeagueApi
import com.stacktraceo.leaguecompanion.data.remote.dto.CreateSummonerRequest
import com.stacktraceo.leaguecompanion.data.remote.dto.MatchListDto
import com.stacktraceo.leaguecompanion.data.remote.dto.StatsDto
import com.stacktraceo.leaguecompanion.data.remote.dto.SummonerDto
import com.stacktraceo.leaguecompanion.data.remote.dto.SyncAcceptedDto
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.update
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody
import okhttp3.ResponseBody.Companion.toResponseBody

class FakeSummonerDao : SummonerDao {
    private val summoners = MutableStateFlow<Map<String, SummonerEntity>>(emptyMap())
    private val ranked = MutableStateFlow<Map<Pair<String, String>, RankedStatEntity>>(emptyMap())

    override fun observe(puuid: String): Flow<SummonerEntity?> = summoners.map { it[puuid] }

    override fun observeAll(): Flow<List<SummonerEntity>> = summoners.map { rows -> rows.values.sortedBy { it.riotId.lowercase() } }

    override fun observeRanked(puuid: String): Flow<List<RankedStatEntity>> =
        ranked.map { rows -> rows.values.filter { it.puuid == puuid }.sortedBy { it.queueType } }

    override suspend fun upsert(summoner: SummonerEntity) {
        summoners.update { it + (summoner.puuid to summoner) }
    }

    override suspend fun upsertRanked(rows: List<RankedStatEntity>) {
        ranked.update { current -> current + rows.associateBy { it.puuid to it.queueType } }
    }

    override suspend fun deleteRanked(puuid: String) {
        ranked.update { rows -> rows.filterKeys { it.first != puuid } }
    }

    override suspend fun delete(puuid: String) {
        summoners.update { it - puuid }
        // ON DELETE CASCADE из схемы.
        deleteRanked(puuid)
    }
}

class FakeMatchDao : MatchDao {
    private val matches = MutableStateFlow<Map<String, MatchEntity>>(emptyMap())
    private val participants = MutableStateFlow<Map<Pair<String, String>, MatchParticipantEntity>>(emptyMap())

    override fun observeFeed(
        puuid: String,
        limit: Int,
    ): Flow<List<MatchListItemView>> =
        combine(matches, participants) { allMatches, allParticipants ->
            allParticipants.values
                .filter { it.puuid == puuid }
                .mapNotNull { participant -> allMatches[participant.matchId]?.let { participant to it } }
                .sortedWith(
                    compareByDescending<Pair<MatchParticipantEntity, MatchEntity>> { it.second.gameCreation }
                        .thenByDescending { it.first.matchId },
                ).take(limit)
                .map { (participant, match) -> view(participant, match) }
        }

    override suspend fun cachedCount(puuid: String): Int = participants.value.values.count { it.puuid == puuid }

    override suspend fun upsertMatches(matches: List<MatchEntity>) {
        this.matches.update { current -> current + matches.associateBy { it.matchId } }
    }

    override suspend fun upsertParticipants(participants: List<MatchParticipantEntity>) {
        this.participants.update { current -> current + participants.associateBy { it.matchId to it.puuid } }
    }

    private fun view(
        participant: MatchParticipantEntity,
        match: MatchEntity,
    ) = MatchListItemView(
        matchId = match.matchId,
        gameCreation = match.gameCreation,
        gameDurationSeconds = match.gameDurationSeconds,
        queueId = match.queueId,
        gameVersion = match.gameVersion,
        championName = participant.championName,
        kills = participant.kills,
        deaths = participant.deaths,
        assists = participant.assists,
        kda = participant.kda,
        win = participant.win,
        cs = participant.cs,
        goldEarned = participant.goldEarned,
    )
}

class FakeMatchDetailDao : MatchDetailDao {
    private val details = MutableStateFlow<Map<String, MatchDetailEntity>>(emptyMap())

    override fun observe(matchId: String): Flow<MatchDetailEntity?> = details.map { it[matchId] }

    override suspend fun upsert(detail: MatchDetailEntity) {
        details.update { it + (detail.matchId to detail) }
    }
}

class FakeLeagueApi : LeagueApi {
    var summonerResponse: SummonerDto? = null
    var matchesResponse: MatchListDto = MatchListDto(items = emptyList(), limit = 20, offset = 0, total = 0)
    var syncResponse: SyncAcceptedDto = SyncAcceptedDto(status = "accepted", puuid = "", lastSyncedAt = null)

    var statsResponse: StatsDto? = null
    var matchDetailJson: String? = null

    var failure: Throwable? = null

    val matchRequests = mutableListOf<Pair<Int, Int>>()
    val statsRequests = mutableListOf<String>()
    val detailRequests = mutableListOf<String>()

    override suspend fun trackSummoner(request: CreateSummonerRequest): SummonerDto = respondWithSummoner()

    override suspend fun summoner(puuid: String): SummonerDto = respondWithSummoner()

    override suspend fun matches(
        puuid: String,
        limit: Int,
        offset: Int,
    ): MatchListDto {
        failure?.let { throw it }
        matchRequests += limit to offset
        return matchesResponse
    }

    override suspend fun stats(
        puuid: String,
        period: String,
    ): StatsDto {
        failure?.let { throw it }
        statsRequests += period
        return checkNotNull(statsResponse) { "тест не задал statsResponse" }
    }

    override suspend fun matchDetail(matchId: String): ResponseBody {
        failure?.let { throw it }
        detailRequests += matchId
        return checkNotNull(matchDetailJson) { "тест не задал matchDetailJson" }
            .toResponseBody("application/json".toMediaType())
    }

    override suspend fun sync(puuid: String): SyncAcceptedDto {
        failure?.let { throw it }
        return syncResponse
    }

    private fun respondWithSummoner(): SummonerDto {
        failure?.let { throw it }
        return checkNotNull(summonerResponse) { "тест не задал summonerResponse" }
    }
}
