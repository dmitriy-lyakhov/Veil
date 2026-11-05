package net.veil.android

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.net.ConnectivityManager
import android.net.IpPrefix
import android.net.VpnService
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.ParcelFileDescriptor
import android.util.Log
import org.json.JSONObject
import veilcore.Protector
import veilcore.Veilcore
import java.net.InetAddress

/**
 * VPN-сервис: Builder из Veilcore.parse() + раздельное туннелирование, TUN-дескриптор отдаётся Go-ядру,
 * сокеты ядра защищаются protect(). Содержимое ссылок и ключи в журнал не пишутся.
 */
class VeilVpnService : VpnService() {

    private var tun: ParcelFileDescriptor? = null
    private val main = Handler(Looper.getMainLooper())
    @Volatile private var running = false
    private var lastRx = 0L
    private var lastTx = 0L

    private val ticker = object : Runnable {
        override fun run() {
            if (!running) return
            val st = try { JSONObject(Veilcore.status()) } catch (_: Exception) { null }
            if (st != null) {
                val rx = st.optLong("rx"); val tx = st.optLong("tx")
                Stats.add(this@VeilVpnService, rx - lastRx, tx - lastTx)
                lastRx = rx; lastTx = tx
                updateNotification(st)
            }
            main.postDelayed(this, 2000)
        }
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val action = intent?.action
        when (action) {
            ACTION_DISCONNECT -> {
                Settings.setWantOn(this, false)
                disconnect()
                return START_NOT_STICKY
            }
            ACTION_CONNECT -> {
                Settings.setWantOn(this, true)
                startForegroundCompat(buildNotification(getString(R.string.notif_connecting)))
                val id = intent.getStringExtra(EXTRA_PROFILE)
                Thread { connect(id) }.start()
            }
            else -> {
                // перезапуск системой после убийства процесса (null) или запуск системным Always-on VPN
                if (Settings.wantOn(this) || action == SERVICE_INTERFACE) {
                    Settings.setWantOn(this, true)
                    startForegroundCompat(buildNotification(getString(R.string.notif_connecting)))
                    Thread { connect(null) }.start()
                } else {
                    stopSelf()
                }
            }
        }
        return START_STICKY
    }

    private fun connect(profileId: String?) {
        synchronized(this) {
            closeAll()
            try {
                val prof = (if (profileId != null) Profiles.all(this).firstOrNull { it.id == profileId } else null)
                    ?: Profiles.active(this) ?: throw IllegalStateException("E_EMPTY")
                if (profileId != null) Settings.setActive(this, prof.id)
                Veilcore.setDNS(networkDns())
                val p = JSONObject(Veilcore.parse(prof.link))
                val b = Builder()
                    .setSession(p.optString("name", "Veil").ifBlank { "Veil" })
                    .setMtu(p.optInt("mtu", 1280))
                    .setBlocking(false)
                if (Build.VERSION.SDK_INT >= 29) b.setMetered(false)
                val addrs = p.getJSONArray("addresses")
                var hasV6 = false
                for (i in 0 until addrs.length()) {
                    val a = addrs.getJSONObject(i)
                    b.addAddress(a.getString("addr"), a.getInt("prefix"))
                    if (a.optBoolean("v6")) hasV6 = true
                }
                // DNS
                val dnsList = mutableListOf<String>()
                val custom = Settings.dns(this).split(",", " ", ";").map { it.trim() }.filter { it.isNotEmpty() }
                if (custom.isNotEmpty()) dnsList += custom else {
                    val dns = p.getJSONArray("dns")
                    for (i in 0 until dns.length()) dnsList += dns.getString(i)
                }
                for (d in dnsList) try { b.addDnsServer(d) } catch (_: Exception) { }

                applyApps(b, p)
                val routeCount = applyRoutes(b, p, dnsList)
                val pfd = b.establish() ?: throw IllegalStateException("E_VPN")
                tun = pfd
                val fd = pfd.detachFd()
                Veilcore.start(prof.link, fd, object : Protector {
                    override fun protect(fd: Int): Boolean = this@VeilVpnService.protect(fd)
                })
                running = true
                Store.since = android.os.SystemClock.elapsedRealtime()
                lastRx = 0; lastTx = 0
                Store.lastError = null
                main.post(ticker)
                Log.i(TAG, "tunnel started protocol=${p.optString("protocol")} routes=$routeCount")
            } catch (e: Exception) {
                Log.w(TAG, "connect failed: ${e.javaClass.simpleName}")
                Store.lastError = e.message ?: e.javaClass.simpleName
                closeAll()
                main.post { stopForegroundCompat(); stopSelf() }
            }
        }
    }

    /**
     * Раздельное туннелирование по приложениям. Для WireGuard/AmneziaWG свой пакет всегда вне VPN (UDP ядра не защищается protect).
     * Для Veil своё приложение остаётся в туннеле: сокеты ядра защищены protect(), а админ-API вкладки «Доступ» ходит по туннелю обычным сокетом.
     */
    private fun applyApps(b: Builder, p: JSONObject) {
        val veil = p.optString("protocol") == "veil"
        val apps = Settings.apps(this).filter { it != packageName }
        if (Settings.appsOn(this) && apps.isNotEmpty() && Settings.appsMode(this) == "via") {
            for (a in apps) try { b.addAllowedApplication(a) } catch (_: Exception) { }
            if (veil) try { b.addAllowedApplication(packageName) } catch (_: Exception) { }
            return
        }
        if (!veil) try { b.addDisallowedApplication(packageName) } catch (_: Exception) { }
        if (Settings.appsOn(this) && Settings.appsMode(this) == "direct") {
            for (a in apps) try { b.addDisallowedApplication(a) } catch (_: Exception) { }
        }
    }

    /** Маршруты: весь трафик / только выбранные адреса / всё, кроме выбранных / AllowedIPs WireGuard. */
    private fun applyRoutes(b: Builder, p: JSONObject, dns: List<String>): Int {
        var n = 0
        fun route(s: String) {
            val parts = s.split("/")
            val l = if (parts.size > 1) parts[1].toInt() else if (parts[0].contains(':')) 128 else 32
            b.addRoute(parts[0], l); n++
        }
        if (Settings.sitesOn(this) && Settings.sites(this).isNotEmpty()) {
            val mode = Settings.sitesMode(this)
            val plan = JSONObject(Veilcore.plan(JSONObject().put("mode", mode).put("text", Settings.sites(this).joinToString("\n")).toString()))
            if (mode == "via") {
                val r = plan.getJSONArray("routes")
                for (i in 0 until r.length()) route(r.getString(i))
                for (d in dns) try { route(d) } catch (_: Exception) { } // DNS туннеля всегда через туннель
                // адрес админки сервера в туннеле (вкладка «Доступ») — тоже через туннель
                val adminHost = p.optString("admin_url").substringAfter("://").substringBefore("/").substringBefore(":")
                if (adminHost.isNotEmpty()) try { route(adminHost) } catch (_: Exception) { }
                return n
            }
            if (Build.VERSION.SDK_INT >= 33) {
                b.addRoute("0.0.0.0", 0); b.addRoute("::", 0); n += 2
                val ex = plan.getJSONArray("exclude")
                for (i in 0 until ex.length()) {
                    val (a, l) = ex.getString(i).split("/")
                    b.excludeRoute(IpPrefix(InetAddress.getByName(a), l.toInt())); n++
                }
            } else {
                val r = plan.getJSONArray("routes")
                for (i in 0 until r.length()) route(r.getString(i))
            }
            return n
        }
        val wr = p.optJSONArray("routes")
        if (wr != null && wr.length() > 0) {
            for (i in 0 until wr.length()) route(wr.getString(i))
            return n
        }
        b.addRoute("0.0.0.0", 0); b.addRoute("::", 0)
        return 2
    }

    private fun networkDns(): String = try {
        val cm = getSystemService(ConnectivityManager::class.java)
        cm.getLinkProperties(cm.activeNetwork)?.dnsServers?.joinToString(",") { it.hostAddress ?: "" } ?: ""
    } catch (_: Exception) { "" }

    private fun closeAll() {
        Store.since = 0L
        running = false
        try { Veilcore.stop() } catch (_: Exception) { }
        try { tun?.close() } catch (_: Exception) { }
        tun = null
    }

    private fun disconnect() {
        Thread {
            synchronized(this) { closeAll() }
            main.post { stopForegroundCompat(); stopSelf() }
        }.start()
    }

    override fun onRevoke() {
        Settings.setWantOn(this, false)
        disconnect()
    }

    override fun onDestroy() {
        main.removeCallbacksAndMessages(null)
        synchronized(this) { closeAll() }
        super.onDestroy()
    }

    // ---------- уведомление ----------

    private fun buildNotification(text: String): Notification {
        val nm = getSystemService(NotificationManager::class.java)
        if (nm.getNotificationChannel(CHANNEL) == null) {
            nm.createNotificationChannel(NotificationChannel(CHANNEL, "Veil VPN", NotificationManager.IMPORTANCE_LOW))
        }
        val open = PendingIntent.getActivity(this, 0, Intent(this, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
        val stop = PendingIntent.getService(this, 1, Intent(this, VeilVpnService::class.java).setAction(ACTION_DISCONNECT), PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
        return Notification.Builder(this, CHANNEL)
            .setSmallIcon(R.drawable.ic_notify)
            .setContentTitle("Veil")
            .setContentText(text)
            .setContentIntent(open)
            .setOngoing(true)
            .addAction(Notification.Action.Builder(null, getString(R.string.notif_disconnect), stop).build())
            .build()
    }

    private fun updateNotification(st: JSONObject) {
        val text = when {
            st.optBoolean("up") -> getString(R.string.notif_connected, fmtBytes(st.optLong("rx")), fmtBytes(st.optLong("tx")))
            st.optString("state") == "reconnecting" -> getString(R.string.notif_reconnecting)
            else -> getString(R.string.notif_connecting)
        }
        getSystemService(NotificationManager::class.java).notify(NOTIF_ID, buildNotification(text))
    }

    private fun startForegroundCompat(n: Notification) {
        if (Build.VERSION.SDK_INT >= 34) startForeground(NOTIF_ID, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE)
        else startForeground(NOTIF_ID, n)
    }

    private fun stopForegroundCompat() = stopForeground(STOP_FOREGROUND_REMOVE)

    companion object {
        const val TAG = "VeilVpn"
        const val ACTION_CONNECT = "net.veil.android.CONNECT"
        const val ACTION_DISCONNECT = "net.veil.android.DISCONNECT"
        const val EXTRA_PROFILE = "profile"
        private const val CHANNEL = "veil_vpn"
        private const val NOTIF_ID = 1

        fun connect(c: Context, profileId: String? = null) {
            val i = Intent(c, VeilVpnService::class.java).setAction(ACTION_CONNECT)
            if (profileId != null) i.putExtra(EXTRA_PROFILE, profileId)
            c.startForegroundService(i)
        }

        fun disconnect(c: Context) {
            Settings.setWantOn(c, false)
            c.startService(Intent(c, VeilVpnService::class.java).setAction(ACTION_DISCONNECT))
        }
    }
}
