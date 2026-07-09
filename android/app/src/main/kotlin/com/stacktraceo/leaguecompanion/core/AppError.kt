package com.stacktraceo.leaguecompanion.core

/**
 * Ошибки в терминах пользователя, а не HTTP-кодов.
 *
 * Главное разделение — [Unauthorized] против [RiotUnavailable]. Бэкенд отдаёт `401`
 * только когда неверен *наш* `X-API-Key`, а протухший 24-часовой ключ Riot приходит
 * как `502 riot_unauthorized` (backend/internal/httpapi/errors.go). Первое чинит
 * владелец приложения в local.properties, второе — владелец бэкенда, и путать их
 * значит отправлять человека чинить не то.
 */
sealed interface AppError {
    /** До бэкенда не достучались вовсе: он не поднят, нет сети, неверный адрес. */
    data object NoNetwork : AppError

    /** Бэкенд не принял X-API-Key приложения. */
    data object Unauthorized : AppError

    data object NotFound : AppError

    /** Бэкенд или Riot попросили подождать; секунды — из заголовка Retry-After. */
    data class RateLimited(
        val retryAfterSeconds: Int,
    ) : AppError

    /** Бэкенд жив, но не может работать с Riot API (протухший ключ, таймаут, недоступность). */
    data object RiotUnavailable : AppError

    /** Ошибка на стороне бэкенда: 5xx, кроме перечисленных выше. */
    data class Server(
        val statusCode: Int,
    ) : AppError

    /** Запрос отвергнут как некорректный — например, неизвестный регион. */
    data class BadRequest(
        val message: String,
    ) : AppError

    data class Unknown(
        val description: String,
    ) : AppError
}
