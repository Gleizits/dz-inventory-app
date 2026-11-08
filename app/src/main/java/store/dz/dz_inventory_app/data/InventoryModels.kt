package store.dz.dz_inventory_app.data

data class Category(
    val id: Long,
    val name: String,
    val description: String?,
    val active: Boolean
)

data class SizeStock(
    val size: String,
    val quantity: Int
)

data class Product(
    val id: Long,
    val categoryId: Long,
    val category: String,
    val name: String,
    val description: String?,
    val photoPath: String?,
    val active: Boolean,
    val units: Int,
    val stock: List<SizeStock>
)

data class Lot(
    val id: Long,
    val productId: Long,
    val date: String,
    val units: Int,
    val totalCost: Double,
    val shippingCost: Double,
    val unitCost: Double
)

data class ProductDetails(
    val product: Product,
    val lots: List<Lot>
)
