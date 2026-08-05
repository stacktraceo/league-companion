package com.stacktraceo.leaguecompanion.data.repository

import com.stacktraceo.leaguecompanion.core.AppResult
import com.stacktraceo.leaguecompanion.data.local.SummonerDao
import com.stacktraceo.leaguecompanion.data.mapper.toDomain
import com.stacktraceo.leaguecompanion.data.mapper.toEntity
import com.stacktraceo.leaguecompanion.data.mapper.toRankedEntities
import com.stacktraceo.leaguecompanion.data.remote.ApiErrorMapper
import com.stacktraceo.leaguecompanion.data.remote.LeagueApi
import com.stacktraceo.leaguecompanion.data.remote.dto.CreateSummonerRequest
import com.stacktraceo.leaguecompanion.data.remote.runCatchingApi
import com.stacktraceo.leaguecompanion.domain.model.Summoner
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.map
import java.time.Instant
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class SummonerRepository
    @Inject
    constructor(
        private val api: LeagueApi,
        private val dao: SummonerDao,
        private val errors: ApiErrorMapper,
    ) {
        fun observe(puuid: String): Flow<Summoner?> =
            combine(dao.observe(puuid), dao.observeRanked(puuid)) { summoner, ranked ->
                summoner?.toDomain(ranked.map { it.toDomain() })
            }

        fun observeTracked(): Flow<List<Summoner>> = dao.observeAll().map { rows -> rows.map { it.toDomain(emptyList()) } }

        suspend fun track(
            riotId: String,
            tagLine: String,
            region: String,
        ): AppResult<String> =
            errors.runCatchingApi {
                val dto = api.trackSummoner(CreateSummonerRequest(riotId = riotId, tagLine = tagLine, region = region))
                dao.upsertWithRanked(dto.toEntity(), dto.toRankedEntities())
                dto.puuid
            }

        suspend fun refresh(puuid: String): AppResult<Unit> =
            errors.runCatchingApi {
                val dto = api.summoner(puuid)
                dao.upsertWithRanked(dto.toEntity(), dto.toRankedEntities())
            }

        suspend fun requestSync(puuid: String): AppResult<Instant?> = errors.runCatchingApi { api.sync(puuid).lastSyncedAt }

        suspend fun untrack(puuid: String) = dao.delete(puuid)
    }
