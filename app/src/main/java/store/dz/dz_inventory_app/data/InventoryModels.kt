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
    val lotId: Long,
    val lotUnits: Int,
    val name: String,
    val description: String?,
    val photoPath: String?,
    val active: Boolean,
    val units: Int,
    val stock: List<SizeStock>
)

data class Lot(
    val id: Long,
    val categoryId: Long,
    val category: String,
    val productId: Long,
    val date: String,
    val units: Int,
    val totalCost: Double,
    val shippingCost: Double,
    val unitCost: Double
    ,
    val assignedUnits: Int,
    val availableUnits: Int
)

data class ProductDetails(
    val product: Product,
    val lots: List<Lot>
)

data class SaleLine(
    val detailId: Long,
    val productId: Long,
    val product: String,
    val size: String,
    val quantity: Int,
    val unitPrice: Double,
    val subtotal: Double
)

data class Sale(
    val id: Long,
    val date: String,
    val status: String,
    val notes: String?,
    val units: Int,
    val total: Double,
    val lines: List<SaleLine>
)

data class ReportLine(
    val productId: Long,
    val product: String,
    val category: String,
    val units: Int,
    val saleTotal: Double,
    val purchaseCost: Double,
    val profit: Double,
    val profitPercentage: Double
)

data class ReportTotal(
    val products: Int,
    val sales: Int,
    val soldUnits: Int,
    val saleTotal: Double,
    val purchaseCost: Double,
    val profit: Double,
    val profitPercentage: Double
)

data class ReportDay(
    val date: String,
    val units: Int,
    val saleTotal: Double,
    val purchaseCost: Double,
    val profit: Double
)

data class SalesReport(
    val from: String,
    val to: String,
    val total: ReportTotal,
    val byProduct: List<ReportLine>,
    val byDay: List<ReportDay>
)
