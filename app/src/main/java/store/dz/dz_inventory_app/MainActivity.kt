package store.dz.dz_inventory_app

import android.content.Context
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.Row
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Assessment
import androidx.compose.material.icons.outlined.Inventory2
import androidx.compose.material.icons.outlined.PointOfSale
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material.icons.outlined.WbSunny
import androidx.compose.material.icons.outlined.DarkMode
import androidx.compose.material3.CenterAlignedTopAppBar
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.platform.LocalContext
import store.dz.dz_inventory_app.ui.theme.DzinventoryappTheme
import store.dz.dz_inventory_app.data.InventoryApi
import store.dz.dz_inventory_app.ui.inventory.InventoryScreen
import store.dz.dz_inventory_app.ui.sales.SalesScreen

private const val PREFERENCES_NAME = "dz_inventory_preferences"
private const val THEME_MODE_KEY = "theme_mode"

private enum class ThemeMode {
    SYSTEM,
    LIGHT,
    DARK
}

private enum class AppDestination(
    val label: String,
    val icon: ImageVector
) {
    INVENTARIO("Inventario", Icons.Outlined.Inventory2),
    VENTAS("Ventas", Icons.Outlined.PointOfSale),
    REPORTES("Reportes", Icons.Outlined.Assessment)
}

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        val preferences = getSharedPreferences(PREFERENCES_NAME, Context.MODE_PRIVATE)
        setContent {
            var themeMode by rememberSaveable {
                mutableStateOf(
                    preferences.getString(THEME_MODE_KEY, ThemeMode.SYSTEM.name)
                        ?.let { runCatching { ThemeMode.valueOf(it) }.getOrDefault(ThemeMode.SYSTEM) }
                        ?: ThemeMode.SYSTEM
                )
            }
            val darkTheme = when (themeMode) {
                ThemeMode.SYSTEM -> androidx.compose.foundation.isSystemInDarkTheme()
                ThemeMode.LIGHT -> false
                ThemeMode.DARK -> true
            }

            DzinventoryappTheme(darkTheme = darkTheme) {
                InventoryApp(
                    api = remember { InventoryApi() },
                    themeMode = themeMode,
                    onThemeModeChange = { selectedMode ->
                        themeMode = selectedMode
                        preferences.edit().putString(THEME_MODE_KEY, selectedMode.name).apply()
                    }
                )
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun InventoryApp(
    api: InventoryApi,
    themeMode: ThemeMode,
    onThemeModeChange: (ThemeMode) -> Unit
) {
    var selectedDestination by rememberSaveable { mutableStateOf(AppDestination.INVENTARIO) }
    var settingsOpen by rememberSaveable { mutableStateOf(false) }

    Scaffold(
        topBar = {
            CenterAlignedTopAppBar(
                title = {
                    Text(
                        text = if (settingsOpen) "Ajustes" else "DZ Urban Style",
                        style = MaterialTheme.typography.titleLarge
                    )
                },
                navigationIcon = {
                    if (settingsOpen) {
                        IconButton(onClick = { settingsOpen = false }) {
                            Text("‹", style = MaterialTheme.typography.headlineSmall)
                        }
                    }
                },
                actions = {
                    if (!settingsOpen) {
                        IconButton(onClick = { settingsOpen = true }) {
                            Icon(
                                imageVector = Icons.Outlined.Settings,
                                contentDescription = "Abrir ajustes"
                            )
                        }
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background
                )
            )
        },
        bottomBar = {
            if (!settingsOpen) {
                NavigationBar {
                    AppDestination.entries.forEach { destination ->
                        NavigationBarItem(
                            selected = selectedDestination == destination,
                            onClick = { selectedDestination = destination },
                            icon = {
                                Icon(
                                    imageVector = destination.icon,
                                    contentDescription = null
                                )
                            },
                            label = { Text(destination.label) }
                        )
                    }
                }
            }
        },
        containerColor = MaterialTheme.colorScheme.background
    ) { innerPadding ->
        if (settingsOpen) {
            SettingsScreen(
                themeMode = themeMode,
                onThemeModeChange = onThemeModeChange,
                contentPadding = innerPadding
            )
        } else {
            when (selectedDestination) {
                AppDestination.INVENTARIO -> InventoryScreen(
                    api = api,
                    contentResolver = LocalContext.current.contentResolver,
                    contentPadding = innerPadding
                )
                AppDestination.VENTAS -> SalesScreen(
                    api = api,
                    contentPadding = innerPadding
                )
                else -> DestinationScreen(
                    destination = selectedDestination,
                    contentPadding = innerPadding
                )
            }
        }
    }
}

@Composable
private fun DestinationScreen(
    destination: AppDestination,
    contentPadding: PaddingValues
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(contentPadding)
            .padding(horizontal = 24.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Text(
            text = destination.label,
            style = MaterialTheme.typography.headlineSmall,
            textAlign = TextAlign.Center
        )
        Text(
            text = "La estructura está lista para la siguiente fase.",
            modifier = Modifier.padding(top = 8.dp),
            style = MaterialTheme.typography.bodyLarge,
            textAlign = TextAlign.Center,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.72f)
        )
    }
}

@Composable
private fun SettingsScreen(
    themeMode: ThemeMode,
    onThemeModeChange: (ThemeMode) -> Unit,
    contentPadding: PaddingValues
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(contentPadding)
            .padding(horizontal = 24.dp, vertical = 16.dp)
    ) {
        Text(
            text = "Apariencia",
            style = MaterialTheme.typography.titleLarge
        )
        ThemeMode.entries.forEach { mode ->
            Row(
                modifier = Modifier.padding(vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                RadioButton(
                    selected = themeMode == mode,
                    onClick = { onThemeModeChange(mode) }
                )
                Icon(
                    imageVector = when (mode) {
                        ThemeMode.SYSTEM -> Icons.Outlined.Settings
                        ThemeMode.LIGHT -> Icons.Outlined.WbSunny
                        ThemeMode.DARK -> Icons.Outlined.DarkMode
                    },
                    contentDescription = null,
                    modifier = Modifier.padding(horizontal = 12.dp)
                )
                Text(
                    when (mode) {
                        ThemeMode.SYSTEM -> "Usar configuración del sistema"
                        ThemeMode.LIGHT -> "Tema claro"
                        ThemeMode.DARK -> "Tema oscuro"
                    }
                )
            }
        }
    }
}
