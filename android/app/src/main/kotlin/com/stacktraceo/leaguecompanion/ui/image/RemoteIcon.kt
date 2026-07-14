package com.stacktraceo.leaguecompanion.ui.image

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage

/**
 * Картинка с CDN поверх залитого прямоугольника.
 *
 * Заливка и есть заглушка: она видна, пока картинка грузится, и остаётся навсегда,
 * если CDN ответил `403`/`404` или сети нет. Отдельного состояния ошибки у иконки
 * нет намеренно — текст рядом самодостаточен (исход матча продублирован словом,
 * чемпион назван по имени), и дыра в картинке не повод показывать экран ошибки.
 */
@Composable
fun RemoteIcon(
    url: String,
    contentDescription: String?,
    modifier: Modifier = Modifier,
    shape: Shape = RoundedCornerShape(6.dp),
) {
    Box(
        modifier =
            modifier
                .clip(shape)
                .background(MaterialTheme.colorScheme.surfaceVariant),
    ) {
        AsyncImage(
            model = url,
            contentDescription = contentDescription,
            modifier = Modifier.matchParentSize(),
        )
    }
}
