package store.dz.dz_inventory_app.ui.sales

import android.Manifest
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material.icons.outlined.Warning
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.ui.unit.dp
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import kotlinx.coroutines.launch
import store.dz.dz_inventory_app.R
import store.dz.dz_inventory_app.data.InventoryApi
import store.dz.dz_inventory_app.data.Product
import store.dz.dz_inventory_app.data.Sale

private const val STOCK_CHANNEL_ID = "stock_zero"

@Composable
fun SalesScreen(
    api: InventoryApi,
    contentPadding: androidx.compose.foundation.layout.PaddingValues
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val snackbar = remember { SnackbarHostState() }
    var products by remember { mutableStateOf<List<Product>>(emptyList()) }
    var sales by remember { mutableStateOf<List<Sale>>(emptyList()) }
    var loading by remember { mutableStateOf(true) }
    var error by remember { mutableStateOf<String?>(null) }
    var refreshToken by remember { mutableLongStateOf(0L) }
    var saleDialogOpen by remember { mutableStateOf(false) }
    var pendingNotification by remember { mutableStateOf<String?>(null) }

    fun refresh() {
        refreshToken++
    }

    fun showError(message: String) {
        error = message
        scope.launch { snackbar.showSnackbar(message) }
    }

    LaunchedEffect(refreshToken) {
        loading = true
        error = null
        runCatching {
            products = api.listProducts()
            sales = api.listSales()
        }.onFailure { showError(it.message ?: "No se pudieron cargar las ventas") }
        loading = false
    }

    ensureStockChannel(context)
    val permissionLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        if (granted) {
            pendingNotification?.let { postStockNotification(context, it) }
        }
        pendingNotification = null
    }

    fun notifyStockZero(product: Product, size: String) {
        val message = "${product.name}: talla $size sin stock"
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ContextCompat.checkSelfPermission(
                context,
                Manifest.permission.POST_NOTIFICATIONS
            ) != PackageManager.PERMISSION_GRANTED
        ) {
            pendingNotification = message
            permissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
        } else {
            postStockNotification(context, message)
        }
    }

    Scaffold(
        modifier = Modifier.padding(contentPadding),
        snackbarHost = { SnackbarHost(snackbar) }
    ) { innerPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding)
                .padding(horizontal = 16.dp)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text("Ventas", style = MaterialTheme.typography.headlineSmall)
                Spacer(Modifier.weight(1f))
                IconButton(onClick = { refresh() }) {
                    Icon(Icons.Outlined.Refresh, contentDescription = "Actualizar ventas")
                }
            }
            Button(
                onClick = { saleDialogOpen = true },
                enabled = products.any { it.units > 0 },
                modifier = Modifier.fillMaxWidth()
            ) {
                Text("Registrar venta")
            }
            if (loading) {
                CircularProgressIndicator(modifier = Modifier.align(Alignment.CenterHorizontally))
            } else if (sales.isEmpty()) {
                Text(
                    "Todavía no hay ventas registradas.",
                    modifier = Modifier.padding(vertical = 24.dp)
                )
            } else {
                LazyColumn(
                    verticalArrangement = Arrangement.spacedBy(10.dp),
                    modifier = Modifier.fillMaxSize()
                ) {
                    items(sales, key = { it.id }) { sale ->
                        SaleCard(
                            sale = sale,
                            onAnnul = {
                                scope.launch {
                                    runCatching { api.annulSale(sale.id) }
                                        .onSuccess { refresh() }
                                        .onFailure {
                                            showError(it.message ?: "No se pudo anular la venta")
                                        }
                                }
                            }
                        )
                    }
                }
            }
        }
    }

    if (saleDialogOpen) {
        SaleDialog(
            products = products,
            onDismiss = { saleDialogOpen = false },
            onSave = { product, size, quantity, price ->
                scope.launch {
                    runCatching {
                        api.createSale(product.id, size, quantity, price)
                        val updatedProducts = api.listProducts()
                        products = updatedProducts
                        sales = api.listSales()
                        updatedProducts.firstOrNull { it.id == product.id }
                            ?.stock?.firstOrNull { it.size == size }
                            ?.takeIf { it.quantity == 0 }
                            ?.let { notifyStockZero(product, size) }
                    }.onSuccess {
                        saleDialogOpen = false
                    }.onFailure {
                        showError(it.message ?: "No se pudo registrar la venta")
                    }
                }
            }
        )
    }
}

@Composable
private fun SaleDialog(
    products: List<Product>,
    onDismiss: () -> Unit,
    onSave: (Product, String, Int, Double) -> Unit
) {
    var selectedProduct by remember { mutableStateOf(products.firstOrNull()) }
    var selectedSize by remember { mutableStateOf("S") }
    var quantity by remember { mutableStateOf("1") }
    var price by remember { mutableStateOf("") }
    var productMenuOpen by remember { mutableStateOf(false) }
    LaunchedEffect(selectedProduct?.id) {
        selectedSize = selectedProduct?.stock
            ?.firstOrNull { it.quantity > 0 }
            ?.size
            ?: "S"
    }
    val stockForSize = selectedProduct?.stock?.firstOrNull { it.size == selectedSize }?.quantity ?: 0
    val parsedQuantity = quantity.toIntOrNull() ?: 0
    val parsedPrice = price.toDoubleOrNull()
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Registrar venta") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedButton(onClick = { productMenuOpen = true }) {
                    Text(selectedProduct?.name ?: "Seleccionar prenda")
                }
                DropdownMenu(
                    expanded = productMenuOpen,
                    onDismissRequest = { productMenuOpen = false }
                ) {
                    products.filter { it.units > 0 }.forEach { product ->
                        DropdownMenuItem(
                            text = { Text("${product.name} (${product.units})") },
                            onClick = {
                                selectedProduct = product
                                productMenuOpen = false
                            }
                        )
                    }
                }
                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    listOf("S", "M", "L", "XL").forEach { size ->
                        FilterChip(
                            selected = selectedSize == size,
                            onClick = { selectedSize = size },
                            enabled = (selectedProduct?.stock
                                ?.firstOrNull { it.size == size }
                                ?.quantity ?: 0) > 0,
                            label = { Text(size) }
                        )
                    }
                }
                Text("Disponible: $stockForSize")
                OutlinedTextField(
                    quantity,
                    { quantity = it },
                    label = { Text("Unidades") },
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                    singleLine = true
                )
                OutlinedTextField(
                    price,
                    { price = it },
                    label = { Text("Precio de venta por unidad") },
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                    singleLine = true
                )
            }
        },
        confirmButton = {
            Button(
                onClick = {
                    selectedProduct?.let { onSave(it, selectedSize, parsedQuantity, parsedPrice ?: 0.0) }
                },
                enabled = selectedProduct != null &&
                    parsedQuantity > 0 &&
                    parsedQuantity <= stockForSize &&
                    parsedPrice != null &&
                    parsedPrice >= 0
            ) { Text("Registrar") }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancelar") } }
    )
}

@Composable
private fun SaleCard(sale: Sale, onAnnul: () -> Unit) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text("Venta #${sale.id}", style = MaterialTheme.typography.titleLarge)
                Spacer(Modifier.weight(1f))
                Text(sale.status)
            }
            Text(sale.date)
            sale.lines.forEach { line ->
                Text("${line.product} · ${line.size} · ${line.quantity} x ${line.unitPrice}")
            }
            Text("Total: ${sale.total}", modifier = Modifier.padding(top = 8.dp))
            if (sale.status == "activa") {
                TextButton(onClick = onAnnul) { Text("Anular venta") }
            }
        }
    }
}

private fun ensureStockChannel(context: Context) {
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
        val manager = context.getSystemService(NotificationManager::class.java)
        manager.createNotificationChannel(
            NotificationChannel(
                STOCK_CHANNEL_ID,
                "Stock en cero",
                NotificationManager.IMPORTANCE_DEFAULT
            )
        )
    }
}

private fun postStockNotification(context: Context, message: String) {
    val notification = NotificationCompat.Builder(context, STOCK_CHANNEL_ID)
        .setSmallIcon(R.drawable.ic_launcher_foreground)
        .setContentTitle("Stock agotado")
        .setContentText(message)
        .setPriority(NotificationCompat.PRIORITY_DEFAULT)
        .setAutoCancel(true)
        .build()
    NotificationManagerCompat.from(context).notify(message.hashCode(), notification)
}
