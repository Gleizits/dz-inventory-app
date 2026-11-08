package store.dz.dz_inventory_app.ui.inventory

import android.content.ContentResolver
import android.content.Context
import android.net.Uri
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.content.FileProvider
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.CameraAlt
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Edit
import androidx.compose.material.icons.outlined.PhotoLibrary
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FloatingActionButton
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
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import java.io.File
import store.dz.dz_inventory_app.data.Category
import store.dz.dz_inventory_app.data.InventoryApi
import store.dz.dz_inventory_app.data.LotDraft
import store.dz.dz_inventory_app.data.Product

@Composable
fun InventoryScreen(
    api: InventoryApi,
    contentResolver: ContentResolver,
    contentPadding: androidx.compose.foundation.layout.PaddingValues
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val snackbar = remember { SnackbarHostState() }
    var categories by remember { mutableStateOf<List<Category>>(emptyList()) }
    var products by remember { mutableStateOf<List<Product>>(emptyList()) }
    var selectedCategoryId by remember { mutableStateOf<Long?>(null) }
    var loading by remember { mutableStateOf(true) }
    var error by remember { mutableStateOf<String?>(null) }
    var refreshToken by remember { mutableLongStateOf(0L) }
    var categoryDialog by remember { mutableStateOf<Category?>(null) }
    var newCategoryDialog by remember { mutableStateOf(false) }
    var productDialog by remember { mutableStateOf<Product?>(null) }
    var newProductDialog by remember { mutableStateOf(false) }
    var lotDialog by remember { mutableStateOf<Product?>(null) }
    var photoProduct by remember { mutableStateOf<Product?>(null) }
    var cameraUri by remember { mutableStateOf<Uri?>(null) }

    fun refresh() {
        refreshToken++
    }

    fun showError(message: String) {
        error = message
        scope.launch { snackbar.showSnackbar(message) }
    }

    LaunchedEffect(selectedCategoryId, refreshToken) {
        loading = true
        error = null
        runCatching {
            categories = api.listCategories()
            products = api.listProducts(selectedCategoryId)
        }.onFailure { showError(it.message ?: "No se pudo cargar el inventario") }
        loading = false
    }

    val galleryLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.GetContent()
    ) { uri ->
        val product = photoProduct
        if (uri != null && product != null) {
            scope.launch {
                runCatching { api.uploadPhoto(product.id, uri, contentResolver) }
                    .onSuccess { refresh() }
                    .onFailure { showError(it.message ?: "No se pudo subir la foto") }
            }
        }
    }

    val cameraLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.TakePicture()
    ) { captured ->
        val product = photoProduct
        val uri = cameraUri
        if (captured && product != null && uri != null) {
            scope.launch {
                    runCatching { api.uploadPhoto(product.id, uri, contentResolver) }
                        .onSuccess { refresh() }
                        .onFailure { showError(it.message ?: "No se pudo subir la foto") }
            }
        }
    }

    fun launchCamera(product: Product) {
        photoProduct = product
        val file = File.createTempFile("dz_foto_", ".jpg", context.cacheDir)
        val uri = FileProvider.getUriForFile(
            context,
            "${context.packageName}.fileprovider",
            file
        )
        cameraUri = uri
        cameraLauncher.launch(uri)
    }

    Scaffold(
        modifier = Modifier.padding(contentPadding),
        snackbarHost = { SnackbarHost(snackbar) },
        floatingActionButton = {
            FloatingActionButton(onClick = { newProductDialog = true }) {
                Icon(Icons.Outlined.Add, contentDescription = "Nueva prenda")
            }
        }
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
                Text("Inventario", style = MaterialTheme.typography.headlineSmall)
                Spacer(Modifier.weight(1f))
                IconButton(onClick = { refresh() }) {
                    Icon(Icons.Outlined.Refresh, contentDescription = "Actualizar")
                }
            }
            LazyRow(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                modifier = Modifier.padding(vertical = 8.dp)
            ) {
                item {
                    FilterChip(
                        selected = selectedCategoryId == null,
                        onClick = { selectedCategoryId = null },
                        label = { Text("Todas") }
                    )
                }
                items(categories) { category ->
                    FilterChip(
                        selected = selectedCategoryId == category.id,
                        onClick = { selectedCategoryId = category.id },
                        label = { Text(category.name) },
                        trailingIcon = {
                            IconButton(onClick = { categoryDialog = category }) {
                                Icon(Icons.Outlined.Edit, contentDescription = "Editar categoría")
                            }
                        }
                    )
                }
                item {
                    OutlinedButton(onClick = { newCategoryDialog = true }) {
                        Text("Categoría")
                    }
                }
            }
            if (loading) {
                CircularProgressIndicator(modifier = Modifier.align(Alignment.CenterHorizontally))
            } else if (products.isEmpty()) {
                EmptyInventory(onRetry = { refresh() })
            } else {
                LazyColumn(
                    verticalArrangement = Arrangement.spacedBy(10.dp),
                    modifier = Modifier.fillMaxSize()
                ) {
                    items(products, key = { it.id }) { product ->
                        ProductCard(
                            product = product,
                            onEdit = { productDialog = product },
                            onLot = { lotDialog = product },
                            onPhoto = { photoProduct = product; galleryLauncher.launch("image/*") },
                            onCamera = { launchCamera(product) },
                            onDelete = {
                                scope.launch {
                                    runCatching { api.deleteProduct(product.id) }
                                        .onSuccess { refresh() }
                                        .onFailure { showError(it.message ?: "No se pudo eliminar la prenda") }
                                }
                            }
                        )
                    }
                }
            }
        }
    }

    if (newCategoryDialog || categoryDialog != null) {
        CategoryDialog(
            category = categoryDialog,
            onDismiss = { newCategoryDialog = false; categoryDialog = null },
            onSave = { name, description ->
                scope.launch {
                    runCatching {
                        if (categoryDialog == null) api.createCategory(name, description)
                        else api.updateCategory(categoryDialog!!.id, name, description)
                    }.onSuccess {
                        newCategoryDialog = false
                        categoryDialog = null
                        refresh()
                    }.onFailure { showError(it.message ?: "No se pudo guardar la categoría") }
                }
            },
            onDelete = categoryDialog?.let { category ->
                {
                    scope.launch {
                        runCatching { api.deleteCategory(category.id) }
                            .onSuccess {
                                categoryDialog = null
                                selectedCategoryId = null
                                refresh()
                            }
                            .onFailure { showError(it.message ?: "No se pudo desactivar la categoría") }
                    }
                }
            }
        )
    }

    if (newProductDialog || productDialog != null) {
        ProductDialog(
            product = productDialog,
            categories = categories,
            onDismiss = { newProductDialog = false; productDialog = null },
            onSave = { categoryId, name, description, stock, lot ->
                scope.launch {
                    runCatching {
                        if (productDialog == null) {
                            api.createProduct(categoryId, name, description, stock, lot)
                        } else {
                            api.updateProduct(productDialog!!.id, categoryId, name, description)
                        }
                    }.onSuccess {
                        newProductDialog = false
                        productDialog = null
                        refresh()
                    }.onFailure { showError(it.message ?: "No se pudo guardar la prenda") }
                }
            }
        )
    }

    if (lotDialog != null) {
        LotDialog(
            product = lotDialog!!,
            onDismiss = { lotDialog = null },
            onSave = { draft ->
                scope.launch {
                    runCatching { api.createLot(lotDialog!!.id, draft) }
                        .onSuccess { lotDialog = null; refresh() }
                        .onFailure { showError(it.message ?: "No se pudo registrar el lote") }
                }
            }
        )
    }
}

@Composable
private fun EmptyInventory(onRetry: () -> Unit) {
    Column(
        modifier = Modifier.fillMaxSize(),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Text("No hay prendas para mostrar", style = MaterialTheme.typography.titleLarge)
        Text("Crea una prenda o revisa la conexión con la API.")
        TextButton(onClick = onRetry) { Text("Reintentar") }
    }
}

@Composable
private fun ProductCard(
    product: Product,
    onEdit: () -> Unit,
    onLot: () -> Unit,
    onPhoto: () -> Unit,
    onCamera: () -> Unit,
    onDelete: () -> Unit
) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(product.name, style = MaterialTheme.typography.titleLarge)
                    Text(product.category, style = MaterialTheme.typography.labelSmall)
                }
                IconButton(onClick = onEdit) {
                    Icon(Icons.Outlined.Edit, contentDescription = "Editar prenda")
                }
                IconButton(onClick = onDelete) {
                    Icon(Icons.Outlined.Delete, contentDescription = "Desactivar prenda")
                }
            }
            Text(product.description ?: "Sin descripción")
            Text("Total: ${product.units} unidades", modifier = Modifier.padding(top = 8.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                product.stock.forEach { stock ->
                    Text("${stock.size}: ${stock.quantity}")
                }
            }
            Row(modifier = Modifier.padding(top = 8.dp)) {
                OutlinedButton(onClick = onLot) { Text("Agregar lote") }
                Spacer(Modifier.width(8.dp))
                OutlinedButton(onClick = onPhoto) {
                    Icon(Icons.Outlined.PhotoLibrary, contentDescription = null)
                    Spacer(Modifier.width(4.dp))
                    Text("Foto")
                }
                Spacer(Modifier.width(8.dp))
                OutlinedButton(onClick = onCamera) {
                    Icon(Icons.Outlined.CameraAlt, contentDescription = null)
                    Spacer(Modifier.width(4.dp))
                    Text("Cámara")
                }
            }
        }
    }
}

@Composable
private fun CategoryDialog(
    category: Category?,
    onDismiss: () -> Unit,
    onSave: (String, String?) -> Unit,
    onDelete: (() -> Unit)?
) {
    var name by remember(category) { mutableStateOf(category?.name ?: "") }
    var description by remember(category) { mutableStateOf(category?.description ?: "") }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(if (category == null) "Nueva categoría" else "Editar categoría") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(name, { name = it }, label = { Text("Nombre") }, singleLine = true)
                OutlinedTextField(description, { description = it }, label = { Text("Descripción") })
            }
        },
        confirmButton = {
            Button(onClick = { onSave(name.trim(), description.trim().ifBlank { null }) }, enabled = name.isNotBlank()) {
                Text("Guardar")
            }
        },
        dismissButton = {
            Row {
                if (onDelete != null) {
                    TextButton(onClick = onDelete) { Text("Desactivar") }
                }
                TextButton(onClick = onDismiss) { Text("Cancelar") }
            }
        }
    )
}

@Composable
private fun ProductDialog(
    product: Product?,
    categories: List<Category>,
    onDismiss: () -> Unit,
    onSave: (Long, String, String?, Map<String, Int>, LotDraft?) -> Unit
) {
    var categoryId by remember(product) { mutableStateOf(product?.categoryId ?: categories.firstOrNull()?.id) }
    var name by remember(product) { mutableStateOf(product?.name ?: "") }
    var description by remember(product) { mutableStateOf(product?.description ?: "") }
    var sizes by remember(product) {
        mutableStateOf(
            listOf("S", "M", "L", "XL").associateWith { size ->
                product?.stock?.firstOrNull { it.size == size }?.quantity ?: 0
            }
        )
    }
    var totalCost by remember(product) { mutableStateOf("") }
    var shippingCost by remember(product) { mutableStateOf("") }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(if (product == null) "Nueva prenda" else "Editar prenda") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(name, { name = it }, label = { Text("Nombre") }, singleLine = true)
                OutlinedTextField(description, { description = it }, label = { Text("Descripción") })
                Text("Stock por talla", style = MaterialTheme.typography.titleMedium)
                LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    items(categories) { category ->
                        FilterChip(
                            selected = category.id == categoryId,
                            onClick = { categoryId = category.id },
                            label = { Text(category.name) }
                        )
                    }
                }
                if (product == null) {
                    listOf("S", "M", "L", "XL").forEach { size ->
                        OutlinedTextField(
                            value = sizes[size].toString(),
                            onValueChange = { value -> sizes = sizes + (size to (value.toIntOrNull() ?: 0)) },
                            label = { Text(size) },
                            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                            singleLine = true
                        )
                    }
                    Text("Lote inicial", style = MaterialTheme.typography.titleMedium)
                    OutlinedTextField(totalCost, { totalCost = it }, label = { Text("Costo total") }, singleLine = true)
                    OutlinedTextField(shippingCost, { shippingCost = it }, label = { Text("Costo de envío") }, singleLine = true)
                } else {
                    Text("El stock se modifica agregando un lote.")
                }
            }
        },
        confirmButton = {
            Button(
                onClick = {
                    val lot = if (product == null) LotDraft(
                        date = "",
                        units = sizes.values.sum(),
                        totalCost = totalCost.toDoubleOrNull() ?: 0.0,
                        shippingCost = shippingCost.toDoubleOrNull() ?: 0.0,
                        stock = sizes
                    ) else null
                    onSave(categoryId ?: 0L, name.trim(), description.trim().ifBlank { null }, sizes, lot)
                },
                enabled = categoryId != null && name.isNotBlank()
            ) { Text("Guardar") }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancelar") } }
    )
}

@Composable
private fun LotDialog(
    product: Product,
    onDismiss: () -> Unit,
    onSave: (LotDraft) -> Unit
) {
    var sizes by remember(product) { mutableStateOf(listOf("S", "M", "L", "XL").associateWith { 0 }) }
    var totalCost by remember(product) { mutableStateOf("") }
    var shippingCost by remember(product) { mutableStateOf("") }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Agregar lote: ${product.name}") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                listOf("S", "M", "L", "XL").forEach { size ->
                    OutlinedTextField(
                        value = sizes[size].toString(),
                        onValueChange = { value -> sizes = sizes + (size to (value.toIntOrNull() ?: 0)) },
                        label = { Text(size) },
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                        singleLine = true
                    )
                }
                OutlinedTextField(totalCost, { totalCost = it }, label = { Text("Costo total") }, singleLine = true)
                OutlinedTextField(shippingCost, { shippingCost = it }, label = { Text("Costo de envío") }, singleLine = true)
            }
        },
        confirmButton = {
            Button(
                onClick = {
                    onSave(
                        LotDraft(
                            date = "",
                            units = sizes.values.sum(),
                            totalCost = totalCost.toDoubleOrNull() ?: 0.0,
                            shippingCost = shippingCost.toDoubleOrNull() ?: 0.0,
                            stock = sizes
                        )
                    )
                },
                enabled = sizes.values.sum() > 0
            ) { Text("Guardar lote") }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancelar") } }
    )
}
