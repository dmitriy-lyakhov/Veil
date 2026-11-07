package net.veil.android

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.net.VpnService

/** Подключение при включении устройства (если включено в настройках и разрешение VPN уже выдано). */
class BootReceiver : BroadcastReceiver() {
    override fun onReceive(c: Context, i: Intent) {
        if (i.action != Intent.ACTION_BOOT_COMPLETED && i.action != Intent.ACTION_MY_PACKAGE_REPLACED) return
        val boot = Settings.autoBoot(c) && i.action == Intent.ACTION_BOOT_COMPLETED
        val restore = Settings.wantOn(c) && i.action == Intent.ACTION_MY_PACKAGE_REPLACED
        if (!(boot || restore)) return
        if (Profiles.active(c) == null || VpnService.prepare(c) != null) return
        VeilVpnService.connect(c)
    }
}
