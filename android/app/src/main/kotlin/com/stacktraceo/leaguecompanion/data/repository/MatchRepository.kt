package com.stacktraceo.leaguecompanion.data.repository

import com.stacktraceo.leaguecompanion.core.AppResult
import com.stacktraceo.leaguecompanion.data.local.MatchDao
import com.stacktraceo.leaguecompanion.data.mapper.toDomain
import com.stacktraceo.leaguecompanion.data.mapper.toMatchEntity
import com.stacktraceo.leaguecompanion.data.mapper.toParticipantEntity
import com.stacktraceo.leaguecompanion.data.remote.ApiErrorMapper
import com.stacktraceo.leaguecompanion.data.remote.LeagueApi
import com.stacktraceo.leaguecompanion.data.remote.runCatchingApi
import com.stacktraceo.leaguecompanion.domain.model.MatchListItem
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map
import javax.inject.Inject
import javax.inject.Singleton

data class MatchPage(
    val loaded: Int,
    val total: Int,
)

@Singleton
class MatchRepository
    @Inject
    constructor(
        private val api: LeagueApi,
        private val dao: MatchDao,
        private val errors: ApiErrorMapper,
    ) {
        fun observeFeed(
            puuid: String,
            limit: Int = DEFAULT_PAGE_SIZE,
        ): Flow<List<MatchListItem>> = dao.observeFeed(puuid, limit).map { rows -> rows.map { it.toDomain() } }

        suspend fun refresh(
            puuid: String,
            limit: Int = DEFAULT_PAGE_SIZE,
            offset: Int = 0,
        ): AppResult<MatchPage> =
            errors.runCatchingApi {
                val page = api.matches(puuid = puuid, limit = limit, offset = offset)

                dao.upsertPage(
                    matches = page.items.map { it.toMatchEntity() },
                    participants = page.items.map { it.toParticipantEntity(puuid) },
                )

                MatchPage(loaded = page.items.size, total = page.total)
            }

        suspend fun cachedCount(puuid: String): Int = dao.cachedCount(puuid)

        companion object {
            const val DEFAULT_PAGE_SIZE = 20
        }
    }
