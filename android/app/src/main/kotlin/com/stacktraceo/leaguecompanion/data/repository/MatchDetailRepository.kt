package com.stacktraceo.leaguecompanion.data.repository

import com.stacktraceo.leaguecompanion.core.AppResult
import com.stacktraceo.leaguecompanion.data.local.MatchDetailDao
import com.stacktraceo.leaguecompanion.data.local.entity.MatchDetailEntity
import com.stacktraceo.leaguecompanion.data.mapper.toDomain
import com.stacktraceo.leaguecompanion.data.remote.ApiErrorMapper
import com.stacktraceo.leaguecompanion.data.remote.LeagueApi
import com.stacktraceo.leaguecompanion.data.remote.dto.MatchDetailDto
import com.stacktraceo.leaguecompanion.data.remote.runCatchingApi
import com.stacktraceo.leaguecompanion.domain.model.MatchDetail
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map
import kotlinx.serialization.json.Json
import java.time.Instant
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class MatchDetailRepository
    @Inject
    constructor(
        private val api: LeagueApi,
        private val dao: MatchDetailDao,
        private val json: Json,
        private val errors: ApiErrorMapper,
    ) {
        /**
         * Разбор происходит на чтении, а не на записи: в кэше лежит тот же текст,
         * что прислал бэкенд, и расширение набора полей не требует ни миграции, ни
         * повторного запроса — ровно то же рассуждение, что у `raw_data` на бэкенде.
         */
        fun observe(
            matchId: String,
            trackedPuuid: String,
        ): Flow<MatchDetail?> = dao.observe(matchId).map { entity -> entity?.let { parse(it.rawData, trackedPuuid) } }

        suspend fun refresh(matchId: String): AppResult<Unit> =
            errors.runCatchingApi {
                val raw = api.matchDetail(matchId).use { it.string() }
                dao.upsert(MatchDetailEntity(matchId = matchId, rawData = raw, fetchedAt = Instant.now()))
            }

        /**
         * Битую строку кэша отдаём как «ничего нет», а не роняем поток: экран тогда
         * просто уходит в загрузку, а следующий [refresh] перезаписывает строку.
         * Исключение здесь убивало бы подписку, и починиться сам экран уже не смог бы.
         */
        private fun parse(
            raw: String,
            trackedPuuid: String,
        ): MatchDetail? = runCatching { json.decodeFromString<MatchDetailDto>(raw).toDomain(trackedPuuid) }.getOrNull()
    }
