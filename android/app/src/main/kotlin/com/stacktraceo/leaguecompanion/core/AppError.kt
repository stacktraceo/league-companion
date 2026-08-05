package com.stacktraceo.leaguecompanion.core

sealed interface AppError {
    data object NoNetwork : AppError

    data object Unauthorized : AppError

    data object NotFound : AppError

    data object MatchNotFound : AppError

    data class RateLimited(
        val retryAfterSeconds: Int,
    ) : AppError

    data object RiotUnavailable : AppError

    data class Server(
        val statusCode: Int,
    ) : AppError

    data class BadRequest(
        val message: String,
    ) : AppError

    data class Unknown(
        val description: String,
    ) : AppError
}
