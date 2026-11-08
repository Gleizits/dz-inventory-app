package store.dz.dz_inventory_app.ui.reports

import android.app.Activity
import android.content.Context
import android.print.PrintManager
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.PictureAsPdf
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
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
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import store.dz.dz_inventory_app.data.InventoryApi
import store.dz.dz_inventory_app.data.ReportLine
import store.dz.dz_inventory_app.data.SalesReport
import java.text.SimpleDateFormat
import java.util.Calendar
import java.util.Locale

@Composable
fun ReportsScreen(
    api: InventoryApi,
    contentPadding: androidx.compose.foundation.layout.PaddingValues
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val snackbar = remember { SnackbarHostState() }
    var from by remember { mutableStateOf("") }
    var to by remember { mutableStateOf("") }
    var report by remember { mutableStateOf<SalesReport?>(null) }
    var loading by remember { mutableStateOf(true) }
    var refreshToken by remember { mutableLongStateOf(0L) }

    fun refresh() {
        refreshToken++
    }

    LaunchedEffect(from, to, refreshToken) {
        loading = true
        runCatching { api.getSalesReport(from, to) }
            .onSuccess { report = it }
            .onFailure {
                scope.launch {
                    snackbar.showSnackbar(it.message ?: "No se pudo cargar el reporte")
                }
            }
        loading = false
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
                Text("Reportes", style = MaterialTheme.typography.headlineSmall)
                Spacer(Modifier.weight(1f))
                IconButton(onClick = { refresh() }) {
                    Icon(Icons.Outlined.Refresh, contentDescription = "Actualizar reporte")
                }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(
                    value = from,
                    onValueChange = { from = it },
                    label = { Text("Desde") },
                    placeholder = { Text("AAAA-MM-DD") },
                    modifier = Modifier.weight(1f),
                    singleLine = true
                )
                OutlinedTextField(
                    value = to,
                    onValueChange = { to = it },
                    label = { Text("Hasta") },
                    placeholder = { Text("AAAA-MM-DD") },
                    modifier = Modifier.weight(1f),
                    singleLine = true
                )
            }
            Row(
                horizontalArrangement = Arrangement.spacedBy(6.dp),
                modifier = Modifier.padding(vertical = 8.dp)
            ) {
                DateShortcut("Hoy") {
                    val today = dateOffset(0)
                    from = today
                    to = today
                }
                DateShortcut("Ayer") {
                    val yesterday = dateOffset(-1)
                    from = yesterday
                    to = yesterday
                }
                DateShortcut("Semana") {
                    from = dateOffset(-6)
                    to = dateOffset(0)
                }
                DateShortcut("Mes") {
                    from = monthStart()
                    to = dateOffset(0)
                }
                DateShortcut("Año") {
                    from = yearStart()
                    to = dateOffset(0)
                }
            }
            if (loading) {
                CircularProgressIndicator(modifier = Modifier.align(Alignment.CenterHorizontally))
            } else {
                report?.let { current ->
                    Button(
                        onClick = {
                            printReport(context, current)
                        },
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Icon(Icons.Outlined.PictureAsPdf, contentDescription = null)
                        Text(" Generar PDF")
                    }
                    ReportSummary(current)
                    LazyColumn(
                        verticalArrangement = Arrangement.spacedBy(8.dp),
                        modifier = Modifier.fillMaxSize()
                    ) {
                        items(current.byProduct, key = { it.productId }) { line ->
                            ReportLineCard(line)
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun DateShortcut(label: String, onClick: () -> Unit) {
    FilterChip(
        selected = false,
        onClick = onClick,
        label = { Text(label) }
    )
}

@Composable
private fun ReportSummary(report: SalesReport) {
    val total = report.total
    Card(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 8.dp)
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text("${report.from} al ${report.to}", style = MaterialTheme.typography.titleLarge)
            Text("Ventas: ${total.sales} · Unidades: ${total.soldUnits}")
            Text("Venta: ${money(total.saleTotal)}")
            Text("Costo de compra: ${money(total.purchaseCost)}")
            Text("Ganancia: ${money(total.profit)} (${total.profitPercentage}%)")
        }
    }
}

@Composable
private fun ReportLineCard(line: ReportLine) {
    Card(modifier = Modifier.fillMaxWidth()) {
        Column(modifier = Modifier.padding(12.dp)) {
            Text(line.product, style = MaterialTheme.typography.titleLarge)
            Text(line.category)
            Text("${line.units} unidades · Venta ${money(line.saleTotal)}")
            Text("Costo ${money(line.purchaseCost)} · Ganancia ${money(line.profit)} (${line.profitPercentage}%)")
        }
    }
}

private fun printReport(context: Context, report: SalesReport) {
    val activity = context as? Activity ?: return
    val webView = WebView(activity)
    webView.settings.javaScriptEnabled = false
    webView.webViewClient = object : WebViewClient() {
        override fun onPageFinished(view: WebView, url: String) {
            val printManager = activity.getSystemService(Context.PRINT_SERVICE) as PrintManager
            printManager.print(
                "DZ Urban Style ${report.from} - ${report.to}",
                webView.createPrintDocumentAdapter("reporte-ventas"),
                null
            )
        }
    }
    webView.loadDataWithBaseURL(
        "file:///android_asset/",
        reportHtml(report),
        "text/html",
        "UTF-8",
        null
    )
}

private fun reportHtml(report: SalesReport): String {
    val rows = report.byProduct.joinToString("") { line ->
        "<tr><td>${html(line.product)}</td><td>${html(line.category)}</td>" +
            "<td>${line.units}</td><td>${money(line.saleTotal)}</td>" +
            "<td>${money(line.purchaseCost)}</td><td>${money(line.profit)}</td>" +
            "<td>${line.profitPercentage}%</td></tr>"
    }
    return """
        <!doctype html>
        <html lang="es">
        <head>
          <meta charset="utf-8">
          <meta name="viewport" content="width=device-width, initial-scale=1">
          <link rel="stylesheet" href="bootstrap.min.css">
          <style>
            :root { --dz-orange: #F4592F; --dz-amber: #FEC554; }
            body { color: #000000; }
            h1 { color: var(--dz-orange); }
            .table thead { background: var(--dz-orange); color: #FFFFFF; }
            .summary { border-left: 6px solid var(--dz-amber); }
          </style>
        </head>
        <body class="p-4">
          <h1>DZ Urban Style</h1>
          <h2>Reporte de ventas</h2>
          <p>Rango: ${html(report.from)} al ${html(report.to)}</p>
          <div class="summary p-3 mb-4">
            <strong>Resumen</strong><br>
            Ventas: ${report.total.sales} · Unidades: ${report.total.soldUnits}<br>
            Venta: ${money(report.total.saleTotal)} · Costo: ${money(report.total.purchaseCost)}<br>
            Ganancia: ${money(report.total.profit)} (${report.total.profitPercentage}%)
          </div>
          <table class="table table-striped">
            <thead><tr><th>Producto</th><th>Categoría</th><th>Unidades</th><th>Venta</th><th>Costo</th><th>Ganancia</th><th>%</th></tr></thead>
            <tbody>$rows</tbody>
          </table>
        </body>
        </html>
    """.trimIndent()
}

private fun money(value: Double): String =
    String.format(Locale.US, "%.2f", value)

private fun html(value: String): String =
    value.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")

private fun dateOffset(days: Int): String {
    val calendar = Calendar.getInstance().apply { add(Calendar.DAY_OF_YEAR, days) }
    return SimpleDateFormat("yyyy-MM-dd", Locale.US).format(calendar.time)
}

private fun monthStart(): String {
    val calendar = Calendar.getInstance().apply {
        set(Calendar.DAY_OF_MONTH, 1)
    }
    return SimpleDateFormat("yyyy-MM-dd", Locale.US).format(calendar.time)
}

private fun yearStart(): String {
    val calendar = Calendar.getInstance().apply {
        set(Calendar.DAY_OF_YEAR, 1)
    }
    return SimpleDateFormat("yyyy-MM-dd", Locale.US).format(calendar.time)
}
