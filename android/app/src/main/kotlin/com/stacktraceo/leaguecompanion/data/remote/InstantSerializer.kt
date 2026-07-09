package com.stacktraceo.leaguecompanion.data.remote

import kotlinx.serialization.KSerializer
import kotlinx.serialization.descriptors.PrimitiveKind
import kotlinx.serialization.descriptors.PrimitiveSerialDescriptor
import kotlinx.serialization.descriptors.SerialDescriptor
import kotlinx.serialization.encoding.Decoder
import kotlinx.serialization.encoding.Encoder
import java.time.Instant
import java.time.OffsetDateTime
import java.time.format.DateTimeFormatter

/**
 * Go отдаёт `time.Time` в RFC3339 — либо с `Z`, либо со смещением вида `+03:00`
 * (зависит от зоны Postgres, а она у контейнера своя).
 *
 * `Instant.parse` рассчитан на форму с `Z`, поэтому разбираем через
 * [OffsetDateTime]: он принимает обе и сам приводит к UTC.
 */
object InstantSerializer : KSerializer<Instant> {
    override val descriptor: SerialDescriptor =
        PrimitiveSerialDescriptor("java.time.Instant", PrimitiveKind.STRING)

    override fun deserialize(decoder: Decoder): Instant = OffsetDateTime.parse(decoder.decodeString()).toInstant()

    override fun serialize(
        encoder: Encoder,
        value: Instant,
    ) {
        encoder.encodeString(DateTimeFormatter.ISO_INSTANT.format(value))
    }
}
