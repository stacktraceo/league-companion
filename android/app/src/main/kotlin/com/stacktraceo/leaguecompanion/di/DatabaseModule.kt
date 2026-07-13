package com.stacktraceo.leaguecompanion.di

import android.content.Context
import androidx.room.Room
import com.stacktraceo.leaguecompanion.data.local.LeagueDatabase
import com.stacktraceo.leaguecompanion.data.local.MatchDao
import com.stacktraceo.leaguecompanion.data.local.MatchDetailDao
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
            .addMigrations(LeagueDatabase.MIGRATION_1_2)
            // Фолбэк на случай версии, для которой миграции нет: содержимое базы
            // целиком перекачивается с бэкенда. Но списки отслеживаемых саммонеров
            // пользователь заводит руками, поэтому там, где потеря состояния заметна,
            // пишется настоящая миграция — см. MIGRATION_1_2.
            .fallbackToDestructiveMigration(dropAllTables = true)
            .build()

    @Provides
    fun provideSummonerDao(database: LeagueDatabase): SummonerDao = database.summonerDao()

    @Provides
    fun provideMatchDao(database: LeagueDatabase): MatchDao = database.matchDao()

    @Provides
    fun provideMatchDetailDao(database: LeagueDatabase): MatchDetailDao = database.matchDetailDao()
}
