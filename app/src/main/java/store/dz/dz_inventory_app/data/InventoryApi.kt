package store.dz.dz_inventory_app.data

import android.content.ContentResolver
import android.net.Uri
import org.json.JSONArray
import org.json.JSONObject
import java.io.BufferedInputStream
import java.io.BufferedOutputStream
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder

class InventoryApi(
    private val baseUrl: String = "http://127.0.0.1:8080"
) {
    suspend fun listCategories(): List<Category> = get("/api/categorias").let { json ->
        (json as JSONArray).toList { it.toCategory() }
    }

    suspend fun createCategory(name: String, description: String?): Category {
        return post("/api/categorias", JSONObject().apply {
            put("nombre", name)
            putNullable("descripcion", description)
        }).toCategory()
    }

    suspend fun updateCategory(id: Long, name: String, description: String?): Category {
        return put("/api/categorias/$id", JSONObject().apply {
            put("nombre", name)
            putNullable("descripcion", description)
        }).toCategory()
    }

    suspend fun deleteCategory(id: Long) {
        delete("/api/categorias/$id")
    }

    suspend fun listProducts(categoryId: Long? = null, search: String = ""): List<Product> {
        val query = buildString {
            append("?incluir_inactivos=false")
            if (categoryId != null) append("&categoria_id=$categoryId")
            if (search.isNotBlank()) {
                append("&q=")
                append(URLEncoder.encode(search, Charsets.UTF_8.name()))
            }
        }
        return get("/api/productos$query").let { json ->
            (json as JSONArray).toList { it.toProduct() }
        }
    }

    suspend fun createProduct(
        categoryId: Long,
        name: String,
        description: String?,
        stock: Map<String, Int>,
        lot: LotDraft?
    ): ProductDetails {
        val body = JSONObject().apply {
            put("categoria_id", categoryId)
            put("nombre", name)
            putNullable("descripcion", description)
            put("stock", stock.toJson())
            if (lot != null) put("lote", lot.toJson())
        }
        return post("/api/productos", body).toProductDetails()
    }

    suspend fun updateProduct(
        id: Long,
        categoryId: Long,
        name: String,
        description: String?
    ): Product {
        return put("/api/productos/$id", JSONObject().apply {
            put("categoria_id", categoryId)
            put("nombre", name)
            putNullable("descripcion", description)
            put("stock", JSONObject())
        }).toProduct()
    }

    suspend fun deleteProduct(id: Long) {
        delete("/api/productos/$id")
    }

    suspend fun createLot(productId: Long, draft: LotDraft) {
        post("/api/lotes", draft.toJson().apply { put("producto_id", productId) })
    }

    suspend fun uploadPhoto(
        productId: Long,
        uri: Uri,
        contentResolver: ContentResolver
    ) {
        val type = contentResolver.getType(uri) ?: "image/jpeg"
        val fileName = "foto_${System.currentTimeMillis()}.jpg"
        val boundary = "----DZInventory${System.currentTimeMillis()}"
        val connection = openConnection("/api/productos/$productId/foto", "POST")
        connection.doOutput = true
        connection.setRequestProperty("Content-Type", "multipart/form-data; boundary=$boundary")
        try {
            BufferedOutputStream(connection.outputStream).use { output ->
                output.write("--$boundary\r\n".toByteArray())
                output.write(
                    "Content-Disposition: form-data; name=\"foto\"; filename=\"$fileName\"\r\n".toByteArray()
                )
                output.write("Content-Type: $type\r\n\r\n".toByteArray())
                contentResolver.openInputStream(uri)?.use { input ->
                    input.copyTo(output)
                } ?: throw IOException("No se pudo leer la imagen seleccionada")
                output.write("\r\n--$boundary--\r\n".toByteArray())
            }
            readResponse(connection)
        } finally {
            connection.disconnect()
        }
    }

    suspend fun listSales(): List<Sale> {
        return (get("/api/ventas?limite=100&offset=0") as JSONArray)
            .toList { it.toSale() }
    }

    suspend fun createSale(
        productId: Long,
        size: String,
        quantity: Int,
        unitPrice: Double,
        notes: String? = null
    ): Sale {
        return post("/api/ventas", JSONObject().apply {
            putNullable("notas", notes)
            put("lineas", JSONArray().put(JSONObject().apply {
                put("producto_id", productId)
                put("talla", size)
                put("cantidad", quantity)
                put("precio_unitario", unitPrice)
            }))
        }).toSale()
    }

    suspend fun annulSale(id: Long): Sale {
        return post("/api/ventas/$id/anular", JSONObject()).toSale()
    }

    private fun get(path: String): Any = request(path, "GET")

    private fun post(path: String, body: JSONObject): JSONObject =
        request(path, "POST", body.toString()) as JSONObject

    private fun put(path: String, body: JSONObject): JSONObject =
        request(path, "PUT", body.toString()) as JSONObject

    private fun delete(path: String) {
        request(path, "DELETE")
    }

    private fun request(path: String, method: String, body: String? = null): Any {
        val connection = openConnection(path, method)
        try {
            if (body != null) {
                connection.doOutput = true
                connection.setRequestProperty("Content-Type", "application/json")
                connection.outputStream.use { it.write(body.toByteArray()) }
            }
            return readResponse(connection)
        } finally {
            connection.disconnect()
        }
    }

    private fun openConnection(path: String, method: String): HttpURLConnection {
        return (URL(baseUrl + path).openConnection() as HttpURLConnection).apply {
            requestMethod = method
            connectTimeout = 8_000
            readTimeout = 15_000
            setRequestProperty("Accept", "application/json")
        }
    }

    private fun readResponse(connection: HttpURLConnection): Any {
        val status = connection.responseCode
        val stream = if (status in 200..299) {
            connection.inputStream
        } else {
            connection.errorStream ?: throw IOException("La API devolvió HTTP $status sin detalles")
        }
        val text = BufferedInputStream(stream).bufferedReader().use { it.readText() }
        if (status !in 200..299) {
            val error = runCatching { JSONObject(text).optString("error") }.getOrNull()
            throw InventoryApiException(error ?: "No se pudo completar la solicitud")
        }
        if (text.isBlank()) return JSONObject()
        return if (text.trimStart().startsWith("[")) JSONArray(text) else JSONObject(text)
    }
}

data class LotDraft(
    val date: String,
    val units: Int,
    val totalCost: Double,
    val shippingCost: Double,
    val stock: Map<String, Int>
) {
    fun toJson(): JSONObject = JSONObject().apply {
        put("fecha", date)
        put("unidades", units)
        put("costo_total", totalCost)
        put("costo_envio", shippingCost)
        put("stock", stock.toJson())
    }
}

class InventoryApiException(message: String) : IOException(message)

private fun JSONObject.toCategory() = Category(
    id = getLong("id"),
    name = getString("nombre"),
    description = optNullableString("descripcion"),
    active = getBoolean("activo")
)

private fun JSONObject.toProduct() = Product(
    id = getLong("id"),
    categoryId = getLong("categoria_id"),
    category = getString("categoria"),
    name = getString("nombre"),
    description = optNullableString("descripcion"),
    photoPath = optNullableString("foto_path"),
    active = getBoolean("activo"),
    units = getInt("unidades"),
    stock = getJSONArray("stock").toList { it.toStock() }
)

private fun JSONObject.toProductDetails() = ProductDetails(
    product = getJSONObject("producto").toProduct(),
    lots = getJSONArray("lotes").toList { it.toLot() }
)

private fun JSONObject.toStock() = SizeStock(
    size = getString("talla"),
    quantity = getInt("cantidad")
)

private fun JSONObject.toLot() = Lot(
    id = getLong("id"),
    productId = getLong("producto_id"),
    date = getString("fecha"),
    units = getInt("unidades"),
    totalCost = getDouble("costo_total"),
    shippingCost = getDouble("costo_envio"),
    unitCost = getDouble("costo_unitario")
)

private fun JSONObject.toSale() = Sale(
    id = getLong("id"),
    date = getString("fecha"),
    status = getString("estado"),
    notes = optNullableString("notas"),
    units = getInt("unidades"),
    total = getDouble("total"),
    lines = getJSONArray("lineas").toList { it.toSaleLine() }
)

private fun JSONObject.toSaleLine() = SaleLine(
    detailId = getLong("detalle_id"),
    productId = getLong("producto_id"),
    product = getString("producto"),
    size = getString("talla"),
    quantity = getInt("cantidad"),
    unitPrice = getDouble("precio_unitario"),
    subtotal = getDouble("subtotal")
)

private fun JSONObject.putNullable(key: String, value: String?) {
    put(key, value ?: JSONObject.NULL)
}

private fun Map<String, Int>.toJson() = JSONObject().apply {
    forEach { (size, quantity) -> put(size, quantity) }
}

private inline fun <T> JSONArray.toList(mapper: (JSONObject) -> T): List<T> =
    List(length()) { index -> mapper(getJSONObject(index)) }

private fun JSONObject.optNullableString(key: String): String? =
    if (isNull(key)) null else optString(key).takeIf { it.isNotBlank() }
