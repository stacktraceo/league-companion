package com.stacktraceo.leaguecompanion.data.remote

import com.stacktraceo.leaguecompanion.core.AppError
import com.stacktraceo.leaguecompanion.data.httpException
import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Test
import java.io.IOException
import java.net.SocketTimeoutException

class ApiErrorMapperTest {
    private val mapper = ApiErrorMapper(Json { ignoreUnknownKeys = true })

    @Test
    fun `401 значит неверный ключ самого приложения`() {
        val error = mapper.map(httpException(401, """{"error":"unauthorized","message":"требуется валидный заголовок X-API-Key"}"""))

        assertEquals(AppError.Unauthorized, error)
    }

    @Test
    fun `502 riot_unauthorized не путается с 401`() {
        // Протухший 24-часовой ключ Riot - проблема бэкенда, а не настройки клиента
        // (backend/internal/httpapi/errors.go). Пользователю нужен другой текст.
        val error = mapper.map(httpException(502, """{"error":"riot_unauthorized","message":"Riot API отклонил ключ бэкенда"}"""))

        assertEquals(AppError.RiotUnavailable, error)
    }

    @Test
    fun `504 riot_timeout тоже про недоступность Riot`() {
        val error = mapper.map(httpException(504, """{"error":"riot_timeout","message":"Riot API не ответил вовремя"}"""))

        assertEquals(AppError.RiotUnavailable, error)
    }

    @Test
    fun `429 забирает срок из Retry-After`() {
        val error =
            mapper.map(
                httpException(
                    code = 429,
                    body = """{"error":"sync_too_soon","message":"синхронизация уже была недавно"}""",
                    headers = mapOf("Retry-After" to "42"),
                ),
            )

        assertEquals(AppError.RateLimited(42), error)
    }

    @Test
    fun `429 без Retry-After откатывается на окно второго лимита Riot`() {
        val error = mapper.map(httpException(429, """{"error":"rate_limited","message":"превышен лимит"}"""))

        assertEquals(AppError.RateLimited(120), error)
    }

    @Test
    fun `404 отличается от прочих ошибок`() {
        val error = mapper.map(httpException(404, """{"error":"summoner_not_found","message":"саммонер не найден"}"""))

        assertEquals(AppError.NotFound, error)
    }

    @Test
    fun `404 матча не путается с 404 саммонера`() {
        // На экране деталей общий текст «Summoner not found» отправлял бы искать
        // несуществующую проблему: матч есть, его просто ещё не синхронизировали.
        val error = mapper.map(httpException(404, """{"error":"match_not_found","message":"матч не найден"}"""))

        assertEquals(AppError.MatchNotFound, error)
    }

    @Test
    fun `400 доносит сообщение бэкенда`() {
        val error = mapper.map(httpException(400, """{"error":"invalid_region","message":"неизвестный регион"}"""))

        assertEquals(AppError.BadRequest("неизвестный регион"), error)
    }

    @Test
    fun `500 остаётся ошибкой сервера`() {
        val error = mapper.map(httpException(500, """{"error":"internal_error","message":"внутренняя ошибка сервера"}"""))

        assertEquals(AppError.Server(500), error)
    }

    @Test
    fun `недоступный бэкенд это отсутствие связи`() {
        assertEquals(AppError.NoNetwork, mapper.map(IOException("Failed to connect")))
        assertEquals(AppError.NoNetwork, mapper.map(SocketTimeoutException("timeout")))
    }

    @Test
    fun `нераспознанное тело не роняет разбор`() {
        // Постороннее тело ответа (например, HTML от прокси) не должно превращать
        // понятную ошибку в исключение внутри маппера.
        val error = mapper.map(httpException(404, "<html>404 Not Found</html>"))

        assertEquals(AppError.NotFound, error)
    }
}
