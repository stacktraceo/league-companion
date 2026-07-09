package com.stacktraceo.leaguecompanion.data.remote

import com.stacktraceo.leaguecompanion.data.remote.dto.CreateSummonerRequest
import com.stacktraceo.leaguecompanion.data.remote.dto.MatchListDto
import com.stacktraceo.leaguecompanion.data.remote.dto.StatsDto
import com.stacktraceo.leaguecompanion.data.remote.dto.SummonerDto
import com.stacktraceo.leaguecompanion.data.remote.dto.SyncAcceptedDto
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.Path
import retrofit2.http.Query

/** REST-контракт бэкенда, SPEC.md 3.4. Пути без ведущего слэша — базовый URL уже с ним. */
interface LeagueApi {
    @POST("api/v1/summoners")
    suspend fun trackSummoner(
        @Body request: CreateSummonerRequest,
    ): SummonerDto

    @GET("api/v1/summoners/{puuid}")
    suspend fun summoner(
        @Path("puuid") puuid: String,
    ): SummonerDto

    @GET("api/v1/summoners/{puuid}/matches")
    suspend fun matches(
        @Path("puuid") puuid: String,
        @Query("limit") limit: Int,
        @Query("offset") offset: Int,
    ): MatchListDto

    @GET("api/v1/summoners/{puuid}/stats")
    suspend fun stats(
        @Path("puuid") puuid: String,
        @Query("period") period: String,
    ): StatsDto

    @POST("api/v1/summoners/{puuid}/sync")
    suspend fun sync(
        @Path("puuid") puuid: String,
    ): SyncAcceptedDto
}
