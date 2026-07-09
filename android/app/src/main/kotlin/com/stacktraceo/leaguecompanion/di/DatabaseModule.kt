package com.stacktraceo.leaguecompanion.di

import android.content.Context
import androidx.room.Room
import com.stacktraceo.leaguecompanion.data.local.LeagueDatabase
import com.stacktraceo.leaguecompanion.data.local.MatchDao
import com.stacktraceo.leaguecompanion.data.local.SummonerDao
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object DatabaseModule {
    @Provides
    @Singleton
    fun provideDatabase(
        @ApplicationContext context: Context,
    ): LeagueDatabase =
        Room
            .databaseBuilder(context, LeagueDatabase::class.java, LeagueDatabase.NAME)
            // База — кэш поверх бэкенда, своих данных в ней нет. Писать миграции ради
            // того, чтобы сохранить перекачиваемое, значит поддерживать вторую копию
            // схемы; при смене версии таблицы просто пересоздаются, и следующий
            // refresh наполняет их заново.
            .fallbackToDestructiveMigration(dropAllTables = true)
            .build()

    @Provides
    fun provideSummonerDao(database: LeagueDatabase): SummonerDao = database.summonerDao()

    @Provides
    fun provideMatchDao(database: LeagueDatabase): MatchDao = database.matchDao()
}
