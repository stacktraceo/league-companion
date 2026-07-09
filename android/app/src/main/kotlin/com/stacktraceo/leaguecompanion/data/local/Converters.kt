package com.stacktraceo.leaguecompanion.data.local

import androidx.room.TypeConverter
import java.time.Instant

/**
 * Время в базе — эпоха в миллисекундах, а не ISO-строка.
 *
 * Лента сортируется по `game_creation`, и сравнение INTEGER в SQLite не зависит от
 * формата: строки RFC3339 из Go сортируются лексикографически верно только пока у
 * всех одинаковое смещение, а `time.Time` отдаёт то `Z`, то `+03:00`.
 * Точность миллисекунд достаточна — Riot и отдаёт `gameCreation` в них.
 */
class Converters {
    @TypeConverter
    fun instantToEpochMilli(value: Instant?): Long? = value?.toEpochMilli()

    @TypeConverter
    fun epochMilliToInstant(value: Long?): Instant? = value?.let(Instant::ofEpochMilli)
}
