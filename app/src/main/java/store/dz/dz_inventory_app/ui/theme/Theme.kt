package store.dz.dz_inventory_app.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable

private val DarkColorScheme = darkColorScheme(
    primary = DzNaranja,
    secondary = DzAmbar,
    tertiary = DzVerde,
    background = DzNegro,
    surface = DzNegro,
    onPrimary = DzNegro,
    onSecondary = DzNegro,
    onTertiary = DzNegro,
    onBackground = DzBlanco,
    onSurface = DzBlanco
)

private val LightColorScheme = lightColorScheme(
    primary = DzNaranja,
    secondary = DzAmbar,
    tertiary = DzVerde,
    background = DzBlanco,
    surface = DzBlanco,
    onPrimary = DzBlanco,
    onSecondary = DzNegro,
    onTertiary = DzNegro,
    onBackground = DzNegro,
    onSurface = DzNegro
)

@Composable
fun DzinventoryappTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit
) {
    val colorScheme = when {
        darkTheme -> DarkColorScheme
        else -> LightColorScheme
    }

    MaterialTheme(
        colorScheme = colorScheme,
        typography = Typography,
        content = content
    )
}