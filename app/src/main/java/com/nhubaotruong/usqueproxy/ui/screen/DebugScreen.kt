package com.nhubaotruong.usqueproxy.ui.screen

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.repeatOnLifecycle
import com.nhubaotruong.usqueproxy.ui.viewmodel.VpnViewModel
import kotlinx.coroutines.delay
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

@Composable
fun DebugScreen(viewModel: VpnViewModel) {
    val stats by viewModel.stats.collectAsStateWithLifecycle()
    val lifecycleOwner = LocalLifecycleOwner.current

    // Poll stats at 2s — faster than MainScreen's 10s for live diagnostic feedback.
    LaunchedEffect(lifecycleOwner) {
        lifecycleOwner.repeatOnLifecycle(Lifecycle.State.STARTED) {
            while (true) {
                runCatching { viewModel.refreshStats() }
                delay(2_000L)
            }
        }
    }

    LazyColumn(
        modifier = Modifier
            .fillMaxSize()
            .padding(horizontal = 16.dp),
        verticalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        item { Spacer(Modifier.height(8.dp)) }

        item {
            DebugSectionHeader("Connection")
            DebugRow("State", if (stats.connected) "Connected" else "Disconnected")
            DebugRow("Has network", if (stats.hasNetwork) "yes" else "no")
            DebugRow("Uptime", formatDebugUptime(stats.uptimeSec))
            DebugRow(
                "Connected since",
                if (stats.connectedSinceMs > 0L)
                    SimpleDateFormat("HH:mm:ss", Locale.getDefault()).format(Date(stats.connectedSinceMs))
                else "—"
            )
            Spacer(Modifier.height(4.dp))
            HorizontalDivider()
        }

        item {
            DebugSectionHeader("Traffic")
            DebugRow("TX", formatBytes(stats.txBytes))
            DebugRow("RX", formatBytes(stats.rxBytes))
            DebugRow("TX packets", stats.txPackets.toString())
            DebugRow("RX packets", stats.rxPackets.toString())
            DebugRow(
                "Delivery",
                if (stats.deliveryRatio >= 0) "${stats.deliveryRatio}%" else "—",
                highlight = stats.deliveryRatio in 0..49,
            )
            DebugRow(
                "RX stall",
                if (stats.connected && stats.rxStallSec > 0) "${stats.rxStallSec}s" else "—",
                highlight = stats.connected && stats.rxStallSec >= 20,
            )
            Spacer(Modifier.height(4.dp))
            HorizontalDivider()
        }

        item {
            DebugSectionHeader("Health")
            DebugRow("Reconnections", stats.connectCount.toString())
            DebugRow("Lifetime rotations", stats.lifetimeRotations.toString())
            DebugRow(
                "Last error",
                stats.lastError.ifEmpty { "none" },
                highlight = stats.lastError.isNotEmpty(),
            )
            Spacer(Modifier.height(16.dp))
        }
    }
}

@Composable
private fun DebugSectionHeader(title: String) {
    Text(
        title,
        style = MaterialTheme.typography.titleSmall,
        color = MaterialTheme.colorScheme.primary,
        modifier = Modifier.padding(vertical = 12.dp),
    )
}

@Composable
private fun DebugRow(label: String, value: String, highlight: Boolean = false) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 4.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        Text(
            label,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Text(
            value,
            style = MaterialTheme.typography.bodyMedium.copy(
                fontFamily = FontFamily.Monospace,
            ),
            color = if (highlight) MaterialTheme.colorScheme.error
                    else MaterialTheme.colorScheme.onSurface,
        )
    }
}

private fun formatDebugUptime(seconds: Int): String {
    if (seconds <= 0) return "—"
    val h = seconds / 3600
    val m = (seconds % 3600) / 60
    val s = seconds % 60
    return "%02d:%02d:%02d".format(h, m, s)
}

private fun formatBytes(bytes: Long): String = when {
    bytes < 1024 -> "$bytes B"
    bytes < 1024 * 1024 -> "%.1f KB".format(bytes / 1024.0)
    bytes < 1024 * 1024 * 1024 -> "%.1f MB".format(bytes / (1024.0 * 1024))
    else -> "%.2f GB".format(bytes / (1024.0 * 1024 * 1024))
}
