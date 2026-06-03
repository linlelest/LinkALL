package com.linkall.app.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import androidx.compose.material3.Typography

val Primary = Color(0xFF2F78FA)
val PrimaryDark = Color(0xFF0F2A66)
val Bg = Color(0xFF0B0F14)
val Panel = Color(0xFF121821)
val Border = Color(0xFF1F2733)
val TextColor = Color(0xFFE6EDF6)
val Muted = Color(0xFF8A96A8)

private val DarkColors = darkColorScheme(
    primary = Primary,
    onPrimary = Color.White,
    secondary = Primary,
    background = Bg,
    onBackground = TextColor,
    surface = Panel,
    onSurface = TextColor,
    surfaceVariant = Panel,
    onSurfaceVariant = Muted,
    outline = Border
)

private val LightColors = lightColorScheme(
    primary = Primary,
    secondary = Primary
)

private val Typo = Typography(
    titleLarge = TextStyle(fontSize = 22.sp, fontWeight = FontWeight.SemiBold),
    titleMedium = TextStyle(fontSize = 16.sp, fontWeight = FontWeight.SemiBold),
    bodyLarge = TextStyle(fontSize = 14.sp),
    bodyMedium = TextStyle(fontSize = 13.sp),
    bodySmall = TextStyle(fontSize = 12.sp),
    labelMedium = TextStyle(fontSize = 12.sp, color = Muted)
)

@Composable
fun LinkALLTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = if (isSystemInDarkTheme()) DarkColors else LightColors,
        typography = Typo,
        content = content
    )
}
