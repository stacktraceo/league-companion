package com.stacktraceo.leaguecompanion.data.repository

import com.stacktraceo.leaguecompanion.core.AppResult
import com.stacktraceo.leaguecompanion.data.mapper.toDomain
import com.stacktraceo.leaguecompanion.data.remote.ApiErrorMapper
import com.stacktraceo.leaguecompanion.data.remote.LeagueApi
import com.stacktraceo.leaguecompanion.data.remote.runCatchingApi
import com.stacktraceo.leaguecompanion.domain.model.Stats
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class StatsRepository
    @Inject
    constructor(
        private val api: LeagueApi,
        private val errors: ApiErrorMapper,
    ) {
        suspend fun stats(
            puuid: String,
            period: String = DEFAULT_PERIOD,
        ): AppResult<Stats> = errors.runCatchingApi { api.stats(puuid, period).toDomain() }

        companion object {
            const val DEFAULT_PERIOD = "30d"
        }
    }
