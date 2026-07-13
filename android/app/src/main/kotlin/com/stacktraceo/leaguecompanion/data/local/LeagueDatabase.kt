package com.stacktraceo.leaguecompanion.data.local

import androidx.room.Database
import androidx.room.RoomDatabase
import androidx.room.TypeConverters
import androidx.room.migration.Migration
import androidx.sqlite.SQLiteConnection
import androidx.sqlite.execSQL
import com.stacktraceo.leaguecompanion.data.local.entity.MatchDetailEntity
import com.stacktraceo.leaguecompanion.data.local.entity.MatchEntity
import com.stacktraceo.leaguecompanion.data.local.entity.MatchParticipantEntity
import com.stacktraceo.leaguecompanion.data.local.entity.RankedStatEntity
import com.stacktraceo.leaguecompanion.data.local.entity.SummonerEntity

/**
 * Локальный кэш. Источник истины для UI (SPEC.md 3.5): экраны читают только отсюда,
 * сеть в базу пишет — поэтому офлайн получается сам собой, а не отдельной веткой.
 *
 * При этом источник истины для *данных* — бэкенд: здесь нет ничего, что нельзя
 * перекачать заново. Отсюда destructive-фолбэк в `DatabaseModule` — но именно
 * фолбэк: см. [MIGRATION_1_2].
 */
@Database(
    entities = [
        SummonerEntity::class,
        RankedStatEntity::class,
        MatchEntity::class,
        MatchParticipantEntity::class,
        MatchDetailEntity::class,
    ],
    version = 2,
    exportSchema = true,
)
@TypeConverters(Converters::class)
abstract class LeagueDatabase : RoomDatabase() {
    abstract fun summonerDao(): SummonerDao

    abstract fun matchDao(): MatchDao

    abstract fun matchDetailDao(): MatchDetailDao

    companion object {
        const val NAME = "league-companion.db"

        /**
         * Добавление `match_details` — настоящей миграцией, а не через
         * destructive-фолбэк.
         *
         * Формально кэш перекачиваем целиком, и уронить базу было бы «бесплатно».
         * Но список отслеживаемых саммонеров пользователь заводит руками, и после
         * обновления приложения он оказался бы пустым — потеря состояния, которого
         * на бэкенде нет. Пять строк дешевле.
         */
        val MIGRATION_1_2 =
            object : Migration(1, 2) {
                override fun migrate(connection: SQLiteConnection) {
                    connection.execSQL(
                        """
                        CREATE TABLE IF NOT EXISTS `match_details` (
                            `match_id` TEXT NOT NULL,
                            `raw_data` TEXT NOT NULL,
                            `fetched_at` INTEGER NOT NULL,
                            PRIMARY KEY(`match_id`)
                        )
                        """.trimIndent(),
                    )
                }
            }
    }
}
