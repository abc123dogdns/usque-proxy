package com.nhubaotruong.usqueproxy.data

import android.content.Context
import android.content.pm.ApplicationInfo
import android.content.pm.PackageManager
import android.graphics.drawable.Drawable

data class AppInfo(
    val packageName: String,
    val label: String,
    val icon: Drawable,
    val isSystemApp: Boolean,
)

class AppRepository(private val context: Context) {

    fun getInstalledApps(): List<AppInfo> {
        val pm = context.packageManager
        return pm.getInstalledApplications(PackageManager.GET_META_DATA)
            .filter { it.packageName != context.packageName }
            .map { info ->
                AppInfo(
                    packageName = info.packageName,
                    label = info.loadLabel(pm).toString(),
                    icon = info.loadIcon(pm),
                    isSystemApp = info.flags and ApplicationInfo.FLAG_SYSTEM != 0,
                )
            }
            .sortedBy { it.label.lowercase() }
    }
}
