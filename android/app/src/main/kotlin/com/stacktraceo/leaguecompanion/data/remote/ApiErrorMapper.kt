package com.stacktraceo.leaguecompanion.data.remote

import com.stacktraceo.leaguecompanion.core.AppError
import com.stacktraceo.leaguecompanion.core.AppResult
import com.stacktraceo.leaguecompanion.data.remote.dto.ApiErrorDto
import kotlinx.serialization.json.Json
import retrofit2.HttpException
import java.io.IOException
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ApiErrorMapper
    @Inject
    constructor(
        private val json: Json,
    ) {
        fun map(throwable: Throwable): AppError =
            when (throwable) {
                // IOException здесь - это «до бэкенда не дошли»: он не поднят,
                // нет сети, неверный адрес в LC_BASE_URL.
                is IOException -> AppError.NoNetwork
                is HttpException -> mapHttp(throwable)
                else -> AppError.Unknown(throwable.message ?: throwable::class.java.simpleName)
            }

        private fun mapHttp(exception: HttpException): AppError {
            val response = exception.response()
            val body = response?.errorBody()?.string().orEmpty()
            val apiError = parseBody(body)

            return when (exception.code()) {
                HTTP_UNAUTHORIZED -> AppError.Unauthorized
                // Под 404 у бэкенда два разных случая: ненайденный саммонер и матч,
                // до которого ещё не дошла синхронизация (httpapi/matches.go).
                // Второй чинится ожиданием, и общий текст врал бы про первый.
                HTTP_NOT_FOUND -> if (apiError.error == MATCH_NOT_FOUND) AppError.MatchNotFound else AppError.NotFound
                HTTP_BAD_REQUEST -> AppError.BadRequest(apiError.message)
                HTTP_TOO_MANY_REQUESTS ->
                    AppError.RateLimited(
                        response?.headers()?.get("Retry-After")?.toIntOrNull() ?: DEFAULT_RETRY_AFTER_SECONDS,
                    )
                HTTP_BAD_GATEWAY, HTTP_GATEWAY_TIMEOUT -> AppError.RiotUnavailable
                else -> AppError.Server(exception.code())
            }
        }

        private fun parseBody(body: String): ApiErrorDto =
            if (body.isBlank()) {
                ApiErrorDto()
            } else {
                runCatching { json.decodeFromString<ApiErrorDto>(body) }.getOrElse { ApiErrorDto() }
            }

        companion object {
            private const val HTTP_BAD_REQUEST = 400
            private const val HTTP_UNAUTHORIZED = 401
            private const val HTTP_NOT_FOUND = 404
            private const val HTTP_TOO_MANY_REQUESTS = 429
            private const val HTTP_BAD_GATEWAY = 502
            private const val HTTP_GATEWAY_TIMEOUT = 504

            private const val DEFAULT_RETRY_AFTER_SECONDS = 120

            private const val MATCH_NOT_FOUND = "match_not_found"
        }
    }

suspend fun <T> ApiErrorMapper.runCatchingApi(block: suspend () -> T): AppResult<T> =
    try {
        AppResult.Success(block())
    } catch (cancellation: kotlinx.coroutines.CancellationException) {
        throw cancellation
    } catch (throwable: Throwable) {
        AppResult.Failure(map(throwable))
    }
