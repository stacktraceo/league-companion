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

/**
 * Профили саммонеров: чтение — из Room, запись — из сети.
 *
 * Наружу торчат две разные вещи: [observe] — подписка, которая живёт всегда и не
 * умеет падать, и `suspend`-методы, которые ходят в сеть и возвращают [AppResult].
 * Экран из-за этого не выбирает между «кэш или сеть»: он подписан на кэш, а
 * обновление — отдельное действие, чей провал не стирает уже показанное.
 */
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

        /**
         * Список отслеживаемых — без ранга: на экране выбора он не показывается,
         * а тянуть его значило бы держать подписку на вторую таблицу ради ничего.
         */
        fun observeTracked(): Flow<List<Summoner>> = dao.observeAll().map { rows -> rows.map { it.toDomain(emptyList()) } }

        /**
         * Добавить саммонера в отслеживаемые. Возвращает puuid — по нему экран
         * переходит к профилю.
         *
         * Запись в базу — внутри [runCatchingApi] вместе с запросом: если вставка
         * упадёт, это должно стать ошибкой операции, а не молча оставить экран
         * подписанным на пустой кэш.
         */
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

        /** Перечитать профиль и ранг. Матчи обновляет [MatchRepository]. */
        suspend fun refresh(puuid: String): AppResult<Unit> =
            errors.runCatchingApi {
                val dto = api.summoner(puuid)
                dao.upsertWithRanked(dto.toEntity(), dto.toRankedEntities())
            }

        /**
         * Попросить бэкенд синхронизироваться с Riot.
         *
         * Отвечает `202` сразу, до того как матчи появятся в базе бэкенда, поэтому
         * возвращается только отметка прошлой синхронизации: новые матчи приедут
         * следующим [MatchRepository.refresh], а не этим вызовом.
         */
        suspend fun requestSync(puuid: String): AppResult<Instant?> = errors.runCatchingApi { api.sync(puuid).lastSyncedAt }

        suspend fun untrack(puuid: String) = dao.delete(puuid)
    }
