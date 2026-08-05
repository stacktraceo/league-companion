package com.stacktraceo.leaguecompanion.data.remote

import okhttp3.Interceptor
import okhttp3.Response
import javax.inject.Inject
import javax.inject.Named

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
