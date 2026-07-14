package com.stacktraceo.leaguecompanion.di

import android.content.Context
import coil3.ImageLoader
import coil3.network.okhttp.OkHttpNetworkFetcherFactory
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import okhttp3.OkHttpClient
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object ImageModule {
    /**
     * Загрузчик картинок собирается на **своём** OkHttp-клиенте, а не на общем.
     *
     * Общий несёт `ApiKeyInterceptor`, который вешает `X-API-Key` на каждый запрос.
     * Иконки лежат на CDN Riot, то есть переиспользование клиента отправляло бы
     * общий секрет бэкенда третьей стороне при каждой картинке — CLAUDE.md,
     * «Конвенции»: секреты не уходят наружу. Клиент указан явно, а не оставлен на
     * умолчание Coil, чтобы это решение было видно в коде.
     */
    @Provides
    @Singleton
    fun provideImageLoader(
        @ApplicationContext context: Context,
    ): ImageLoader =
        ImageLoader
            .Builder(context)
            .components {
                add(OkHttpNetworkFetcherFactory(callFactory = { OkHttpClient() }))
            }.build()
}
