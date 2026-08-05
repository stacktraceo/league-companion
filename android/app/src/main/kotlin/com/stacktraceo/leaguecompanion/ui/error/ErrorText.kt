package com.stacktraceo.leaguecompanion.ui.error

import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource
import com.stacktraceo.leaguecompanion.R
import com.stacktraceo.leaguecompanion.core.AppError

@Composable
fun AppError.asText(): String =
    when (this) {
        AppError.NoNetwork -> stringResource(R.string.error_no_network)
        AppError.Unauthorized -> stringResource(R.string.error_unauthorized)
        AppError.NotFound -> stringResource(R.string.error_not_found)
        AppError.MatchNotFound -> stringResource(R.string.error_match_not_found)
        AppError.RiotUnavailable -> stringResource(R.string.error_riot_unavailable)
        is AppError.RateLimited -> stringResource(R.string.error_rate_limited, retryAfterSeconds)
        is AppError.Server -> stringResource(R.string.error_server, statusCode)
        // Сообщение бэкенда здесь осмысленное: `400` он отдаёт с текстом вида
        // «неизвестный регион», и подменять его своим значило бы терять причину.
        is AppError.BadRequest -> message
        is AppError.Unknown -> stringResource(R.string.error_unknown)
    }
