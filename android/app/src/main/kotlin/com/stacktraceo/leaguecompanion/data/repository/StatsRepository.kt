package com.stacktraceo.leaguecompanion.data.repository

import com.stacktraceo.leaguecompanion.core.AppResult
import com.stacktraceo.leaguecompanion.data.mapper.toDomain
import com.stacktraceo.leaguecompanion.data.remote.ApiErrorMapper
import com.stacktraceo.leaguecompanion.data.remote.LeagueApi
import com.stacktraceo.leaguecompanion.data.remote.runCatchingApi
import com.stacktraceo.leaguecompanion.domain.model.Stats
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Единственный репозиторий без кэша — и это осознанно.
 *
 * Матчи за период уже лежат в Room, так что статистику можно было бы считать
 * локально. Но тогда правило «при нуле смертей KDA = K+A» и порядок топ-5 жили бы
 * в двух местах, и первое же расхождение показало бы на одном экране одни цифры,
 * а на другом другие. Бэкенд считает это в `domain.Aggregate`, под десятью тестами.
 * Цена решения — офлайн экран статистики не работает и честно говорит об этом.
 */
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
            /** Период по умолчанию из SPEC.md 3.4. */
            const val DEFAULT_PERIOD = "30d"
        }
    }
