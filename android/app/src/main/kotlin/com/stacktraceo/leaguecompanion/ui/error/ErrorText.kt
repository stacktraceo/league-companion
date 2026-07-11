package com.stacktraceo.leaguecompanion.ui.error

import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource
import com.stacktraceo.leaguecompanion.R
import com.stacktraceo.leaguecompanion.core.AppError

/**
 * Единственное место, где [AppError] превращается в текст для пользователя.
 *
 * Формулировки различаются не для разнообразия: `401` значит, что неверно настроено
 * само приложение, а `502` — что бэкенд не смог достучаться до Riot (чаще всего
 * протух его 24-часовой ключ). Это чинят разные люди в разных местах, и один общий
 * текст на оба случая отправлял бы искать не туда. Ровно этот случай поймался на
 * живой проверке вехи «Дни 9–10».
 */
@Composable
fun AppError.asText(): String =
    when (this) {
        AppError.NoNetwork -> stringResource(R.string.error_no_network)
        AppError.Unauthorized -> stringResource(R.string.error_unauthorized)
        AppError.NotFound -> stringResource(R.string.error_not_found)
        AppError.RiotUnavailable -> stringResource(R.string.error_riot_unavailable)
        is AppError.RateLimited -> stringResource(R.string.error_rate_limited, retryAfterSeconds)
        is AppError.Server -> stringResource(R.string.error_server, statusCode)
        // Сообщение бэкенда здесь осмысленное: `400` он отдаёт с текстом вида
        // «неизвестный регион», и подменять его своим значило бы терять причину.
        is AppError.BadRequest -> message
        is AppError.Unknown -> stringResource(R.string.error_unknown)
    }
