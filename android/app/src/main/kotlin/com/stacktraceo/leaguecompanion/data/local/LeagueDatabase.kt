package com.stacktraceo.leaguecompanion.data.local

import androidx.room.Database
import androidx.room.RoomDatabase
import androidx.room.TypeConverters
import com.stacktraceo.leaguecompanion.data.local.entity.MatchEntity
import com.stacktraceo.leaguecompanion.data.local.entity.MatchParticipantEntity
import com.stacktraceo.leaguecompanion.data.local.entity.RankedStatEntity
import com.stacktraceo.leaguecompanion.data.local.entity.SummonerEntity

/**
 * Локальный кэш. Источник истины для UI (SPEC.md 3.5): экраны читают только отсюда,
 * сеть в базу пишет — поэтому офлайн получается сам собой, а не отдельной веткой.
 *
 * При этом источник истины для *данных* — бэкенд: здесь нет ничего, что нельзя
 * перекачать заново. Отсюда и destructive-миграции в `DatabaseModule`.
 */
@Database(
    entities = [
        SummonerEntity::class,
        RankedStatEntity::class,
        MatchEntity::class,
        MatchParticipantEntity::class,
    ],
    version = 1,
    exportSchema = true,
)
@TypeConverters(Converters::class)
abstract class LeagueDatabase : RoomDatabase() {
    abstract fun summonerDao(): SummonerDao

    abstract fun matchDao(): MatchDao

    companion object {
        const val NAME = "league-companion.db"
    }
}
