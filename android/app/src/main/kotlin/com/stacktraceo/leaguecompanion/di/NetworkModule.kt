package com.stacktraceo.leaguecompanion.di

import com.stacktraceo.leaguecompanion.BuildConfig
import com.stacktraceo.leaguecompanion.data.remote.ApiKeyInterceptor
import com.stacktraceo.leaguecompanion.data.remote.LeagueApi
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.logging.HttpLoggingInterceptor
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory
import java.util.concurrent.TimeUnit
import javax.inject.Named
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object NetworkModule {
    @Provides
    @Singleton
    @Named("apiKey")
    fun provideApiKey(): String = BuildConfig.API_KEY

    @Provides
    @Singleton
    fun provideJson(): Json =
        Json {
            // Бэкенд может добавить поля раньше, чем клиент про них узнает,
            // и это не повод падать при разборе ответа.
            ignoreUnknownKeys = true
        }

    @Provides
    @Singleton
    fun provideOkHttpClient(apiKeyInterceptor: ApiKeyInterceptor): OkHttpClient {
        val builder =
            OkHttpClient
                .Builder()
                .addInterceptor(apiKeyInterceptor)
                .connectTimeout(CONNECT_TIMEOUT_SECONDS, TimeUnit.SECONDS)
                .readTimeout(READ_TIMEOUT_SECONDS, TimeUnit.SECONDS)

        if (BuildConfig.DEBUG) {
            val logging =
                HttpLoggingInterceptor().apply {
                    level = HttpLoggingInterceptor.Level.BODY
                    // Иначе общий секрет уехал бы в logcat при каждом запросе
                    // (DECISIONS.md, «Конвенции»: секреты не печатаем в логи).
                    redactHeader(ApiKeyInterceptor.HEADER)
                }
            builder.addInterceptor(logging)
        }

        return builder.build()
    }

    @Provides
    @Singleton
    fun provideRetrofit(
        client: OkHttpClient,
        json: Json,
    ): Retrofit =
        Retrofit
            .Builder()
            .baseUrl(BuildConfig.BASE_URL)
            .client(client)
            .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
            .build()

    @Provides
    @Singleton
    fun provideLeagueApi(retrofit: Retrofit): LeagueApi = retrofit.create(LeagueApi::class.java)

    // POST /summoners делает три запроса к Riot и отвечает примерно за полторы
    // секунды на холодном кэше - read-таймаут должен это переживать с запасом.
    private const val CONNECT_TIMEOUT_SECONDS = 10L
    private const val READ_TIMEOUT_SECONDS = 30L
}
