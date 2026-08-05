package com.stacktraceo.leaguecompanion.data.remote

import mockwebserver3.MockResponse
import mockwebserver3.MockWebServer
import okhttp3.OkHttpClient
import okhttp3.Request
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Before
import org.junit.Test

class ApiKeyInterceptorTest {
    private lateinit var server: MockWebServer

    @Before
    fun setUp() {
        server = MockWebServer()
        server.start()
    }

    @After
    fun tearDown() {
        server.close()
    }

    @Test
    fun `подставляет заголовок X-API-Key`() {
        server.enqueue(
            MockResponse
                .Builder()
                .code(200)
                .body("{}")
                .build(),
        )

        call(apiKey = "secret-from-local-properties")

        val recorded = server.takeRequest()
        assertEquals("secret-from-local-properties", recorded.headers["X-API-Key"])
    }

    @Test
    fun `пустой ключ не превращается в пустой заголовок`() {
        // Иначе бэкенд увидел бы X-API-Key: "" и в логах отметил «ключ_передан=true»,
        // уводя от настоящей причины - ключ просто не настроен.
        server.enqueue(
            MockResponse
                .Builder()
                .code(401)
                .body("{}")
                .build(),
        )

        call(apiKey = "")

        val recorded = server.takeRequest()
        assertNull(recorded.headers["X-API-Key"])
    }

    private fun call(apiKey: String) {
        val client =
            OkHttpClient
                .Builder()
                .addInterceptor(ApiKeyInterceptor(apiKey))
                .build()

        client
            .newCall(Request.Builder().url(server.url("/api/v1/summoners/puuid")).build())
            .execute()
            .close()
    }
}
