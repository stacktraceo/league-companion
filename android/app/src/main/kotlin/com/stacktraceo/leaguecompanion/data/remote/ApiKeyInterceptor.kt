package com.stacktraceo.leaguecompanion.data.remote

import okhttp3.Interceptor
import okhttp3.Response
import javax.inject.Inject
import javax.inject.Named

/**
 * Проставляет общий с бэкендом секрет (CLAUDE.md, «Отклонения», п. 3).
 *
 * Значение приходит из BuildConfig, а туда — из local.properties, поэтому в коде
 * и в git его нет. Пустой ключ не подставляем вовсе: пусть бэкенд ответит `401`,
 * который мы покажем как «проверь настройку», а не как невнятную ошибку.
 */
class ApiKeyInterceptor
    @Inject
    constructor(
        @Named("apiKey") private val apiKey: String,
    ) : Interceptor {
        override fun intercept(chain: Interceptor.Chain): Response {
            if (apiKey.isEmpty()) {
                return chain.proceed(chain.request())
            }

            val request =
                chain
                    .request()
                    .newBuilder()
                    .header(HEADER, apiKey)
                    .build()

            return chain.proceed(request)
        }

        companion object {
            const val HEADER = "X-API-Key"
        }
    }
