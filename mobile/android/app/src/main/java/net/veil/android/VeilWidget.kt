package net.veil.android

import android.app.PendingIntent
import android.appwidget.AppWidgetManager
import android.appwidget.AppWidgetProvider
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.res.Configuration
import android.net.VpnService
import android.os.Bundle
import android.os.SystemClock
import android.view.View
import android.widget.RemoteViews
import org.json.JSONObject
import veilcore.Veilcore

/**
 * Виджет на рабочем столе (дизайн W1–W2). Один виджет тянется: 1×1 — кольцо, 2×1 — кольцо и сервер,
 * 3–4×1 — строка с кнопкой, 4×2 — панель с тремя серверами. Подключение — без открытия приложения;
 * если Android ещё не дал разрешение на VPN, открывается приложение с системным запросом.
 * Обновляется сервисом VPN: сразу при смене состояния и раз в минуту, пока туннель поднят.
 */
class VeilWidget : AppWidgetProvider() {

    override fun onUpdate(ctx: Context, mgr: AppWidgetManager, ids: IntArray) = update(ctx)

    override fun onAppWidgetOptionsChanged(ctx: Context, mgr: AppWidgetManager, id: Int, options: Bundle) = update(ctx)

    override fun onReceive(ctx: Context, intent: Intent) {
        when (intent.action) {
            ACTION_TOGGLE -> {
                if (state(ctx) == State.ON || state(ctx) == State.BUSY) {
                    pendingSince = 0L
                    VeilVpnService.disconnect(ctx)
                } else connect(ctx, null)
                update(ctx)
            }
            ACTION_SELECT -> {
                val id = intent.getStringExtra(EXTRA_PROFILE) ?: return
                Settings.setActive(ctx, id)
                connect(ctx, id)
                update(ctx)
            }
            else -> super.onReceive(ctx, intent)
        }
    }

    enum class State { OFF, BUSY, ON, ERR }

    companion object {
        const val ACTION_TOGGLE = "net.veil.android.WIDGET_TOGGLE"
        const val ACTION_SELECT = "net.veil.android.WIDGET_SELECT"
        /** Открыть приложение и подключиться (нужен системный запрос разрешения VPN). */
        const val ACTION_OPEN_CONNECT = "net.veil.android.WIDGET_CONNECT"
        const val EXTRA_PROFILE = "profile"

        /** Нажатие «подключить» в виджете: до ответа сервиса виджет показывает «Подключение…». */
        @Volatile private var pendingSince = 0L
        @Volatile private var lastKey = ""

        private fun connect(ctx: Context, profileId: String?) {
            if (Profiles.all(ctx).isEmpty()) { open(ctx, null); return }
            Store.lastError = null
            if (VpnService.prepare(ctx) != null) { open(ctx, profileId ?: Profiles.active(ctx)?.id); return }
            pendingSince = SystemClock.elapsedRealtime()
            VeilVpnService.connect(ctx, profileId)
        }

        private fun open(ctx: Context, profileId: String?) {
            val i = Intent(ctx, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP)
            if (profileId != null) i.setAction(ACTION_OPEN_CONNECT).putExtra(EXTRA_PROFILE, profileId)
            ctx.startActivity(i)
        }

        private fun status(): JSONObject = try { JSONObject(Veilcore.status()) } catch (_: Exception) { JSONObject().put("state", "stopped") }

        fun state(ctx: Context, st: JSONObject = status()): State {
            val stopped = st.optString("state", "stopped") == "stopped"
            return when {
                !stopped && st.optBoolean("up") -> State.ON
                !stopped -> State.BUSY
                Store.lastError != null -> { pendingSince = 0L; State.ERR }
                pendingSince != 0L && SystemClock.elapsedRealtime() - pendingSince < 20_000 -> State.BUSY
                else -> State.OFF
            }
        }

        /** Вызывается сервисом каждые 2 с: перерисовать, если сменилось состояние или минута таймера. */
        fun tick(ctx: Context, st: JSONObject) {
            val s = state(ctx, st)
            if (s == State.ON || s == State.ERR) pendingSince = 0L
            val minutes = if (Store.since > 0) (SystemClock.elapsedRealtime() - Store.since) / 60_000 else -1
            val key = "$s:$minutes:${Settings.activeId(ctx)}"
            if (key != lastKey) update(ctx, st)
        }

        fun update(ctx: Context, st: JSONObject = status()) {
            val mgr = AppWidgetManager.getInstance(ctx)
            val ids = mgr.getAppWidgetIds(ComponentName(ctx, VeilWidget::class.java))
            if (ids.isEmpty()) return
            val s = state(ctx, st)
            val minutes = if (Store.since > 0) (SystemClock.elapsedRealtime() - Store.since) / 60_000 else -1
            lastKey = "$s:$minutes:${Settings.activeId(ctx)}"
            for (id in ids) {
                try { mgr.updateAppWidget(id, views(ctx, mgr.getAppWidgetOptions(id), s, st)) } catch (_: Exception) { }
            }
        }

        private fun isDark(ctx: Context): Boolean = when (Settings.theme(ctx)) {
            "dark" -> true
            "light" -> false
            else -> (ctx.resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES
        }

        private fun protoTitle(p: String) = when (p) { "wireguard" -> "WireGuard"; "amneziawg" -> "AmneziaWG"; else -> "Veil" }

        private fun views(ctx: Context, opts: Bundle, s: State, st: JSONObject): RemoteViews {
            // размер: в портретной ориентации ширина — минимальная, высота — максимальная из опций лаунчера
            val w = opts.getInt(AppWidgetManager.OPTION_APPWIDGET_MIN_WIDTH, 250)
            val h = opts.getInt(AppWidgetManager.OPTION_APPWIDGET_MAX_HEIGHT, 70)
            val layout = when {
                w < 100 -> R.layout.widget_1x1
                h >= 130 && w >= 200 -> R.layout.widget_4x2
                w < 200 -> R.layout.widget_2x1
                else -> R.layout.widget_4x1
            }
            val v = RemoteViews(ctx.packageName, layout)
            val dark = isDark(ctx)
            val ink = if (dark) 0xFFFFFFFF.toInt() else 0xFF0B0B0C.toInt()
            val muted = if (dark) 0xFFA8AAB2.toInt() else 0xFF5E626B.toInt()
            val accent = 0xFFFF393D.toInt()
            val errInk = if (dark) 0xFFFF6B6E.toInt() else 0xFFC4161C.toInt()
            val dot = when (s) { State.ON -> 0xFF7DE0A0.toInt(); State.BUSY -> 0xFFFFB547.toInt(); State.ERR -> accent; else -> muted }
            v.setInt(R.id.root, "setBackgroundResource", if (dark) R.drawable.wg_bg_dark else R.drawable.wg_bg_light)

            val active = Profiles.active(ctx)
            val name = active?.name ?: ctx.getString(R.string.wg_no_server)
            val proto = protoTitle(active?.protocol ?: "veil")
            val time = if (Store.since > 0) {
                val m = (SystemClock.elapsedRealtime() - Store.since) / 60_000
                "%02d:%02d".format(m / 60, m % 60)
            } else "00:00"
            val traffic = "↓ " + fmtBytes(st.optLong("rx")) + " ↑ " + fmtBytes(st.optLong("tx"))
            val stateText = ctx.getString(when (s) { State.ON -> R.string.wg_on; State.BUSY -> R.string.wg_busy; State.ERR -> R.string.wg_err; else -> R.string.wg_off })
            val subInk = if (s == State.ERR) errInk else muted

            // кольцо (есть во всех размерах, кроме строки)
            if (layout != R.layout.widget_4x1) {
                v.setInt(R.id.ring, "setBackgroundResource", when (s) {
                    State.ON -> R.drawable.wg_ring_on
                    State.ERR -> R.drawable.wg_ring_err
                    State.BUSY -> if (dark) R.drawable.wg_ring_busy_dark else R.drawable.wg_ring_busy_light
                    else -> if (dark) R.drawable.wg_ring_off_dark else R.drawable.wg_ring_off_light
                })
                v.setImageViewResource(R.id.icon, when (s) { State.ON -> R.drawable.wg_shield; State.ERR -> R.drawable.wg_alert; else -> R.drawable.wg_power })
                v.setInt(R.id.icon, "setColorFilter", when (s) { State.ON -> 0xFF0B0B0C.toInt(); State.ERR -> accent; else -> ink })
                v.setViewVisibility(R.id.spin, if (s == State.BUSY) View.VISIBLE else View.GONE)
                v.setContentDescription(R.id.ring, ctx.getString(if (s == State.ON || s == State.BUSY) R.string.wg_disconnect else R.string.wg_connect))
                v.setOnClickPendingIntent(R.id.ring, toggleIntent(ctx))
            }
            when (layout) {
                R.layout.widget_1x1 -> v.setOnClickPendingIntent(R.id.root, toggleIntent(ctx))
                R.layout.widget_2x1 -> {
                    text(v, R.id.name, name, ink)
                    text(v, R.id.sub, when (s) { State.ON -> "$time · ↓ " + fmtBytes(st.optLong("rx")); else -> stateText }, subInk)
                    v.setOnClickPendingIntent(R.id.root, toggleIntent(ctx))
                }
                R.layout.widget_4x1 -> {
                    v.setInt(R.id.dot, "setColorFilter", dot)
                    text(v, R.id.name, name, ink)
                    text(v, R.id.sub, when (s) {
                        State.ON -> "$time · $traffic"
                        State.ERR -> stateText + " · " + ctx.getString(R.string.wg_tap_retry)
                        State.BUSY -> stateText
                        else -> "$stateText · $proto"
                    }, subInk)
                    val (label, bg, fg) = when (s) {
                        State.ON -> Triple(R.string.wg_disconnect, if (dark) R.drawable.wg_chip_dark else R.drawable.wg_chip_light, ink)
                        State.BUSY -> Triple(R.string.wg_disconnect, if (dark) R.drawable.wg_chip_dark else R.drawable.wg_chip_light, ink)
                        State.ERR -> Triple(R.string.wg_retry, R.drawable.wg_btn_accent, 0xFF0B0B0C.toInt())
                        else -> Triple(R.string.wg_connect, R.drawable.wg_btn_accent, 0xFF0B0B0C.toInt())
                    }
                    text(v, R.id.btn, if (s == State.BUSY) "…" else ctx.getString(label), fg)
                    v.setInt(R.id.btn, "setBackgroundResource", bg)
                    v.setOnClickPendingIntent(R.id.btn, toggleIntent(ctx))
                    v.setOnClickPendingIntent(R.id.root, openIntent(ctx))
                }
                R.layout.widget_4x2 -> {
                    v.setInt(R.id.dot, "setColorFilter", dot)
                    text(v, R.id.state, stateText, subInk)
                    text(v, R.id.name, name, ink)
                    text(v, R.id.sub, when (s) {
                        State.ON -> "$proto · $time · $traffic"
                        State.ERR -> proto + " · " + ctx.getString(R.string.wg_tap_retry)
                        State.BUSY -> proto
                        else -> if (active == null) ctx.getString(R.string.wg_add_server) else proto + " · " + ctx.getString(R.string.wg_tap_connect)
                    }, subInk)
                    v.setOnClickPendingIntent(R.id.root, openIntent(ctx))
                    chips(ctx, v, active?.id, dark, ink, muted)
                }
            }
            return v
        }

        /** Три сервера в панели 4×2: активный всегда среди них. */
        private fun chips(ctx: Context, v: RemoteViews, activeId: String?, dark: Boolean, ink: Int, muted: Int) {
            val all = Profiles.all(ctx)
            val list = all.take(3).toMutableList()
            val act = all.firstOrNull { it.id == activeId }
            if (act != null && list.none { it.id == act.id } && list.isNotEmpty()) list[list.size - 1] = act
            val ids = listOf(Triple(R.id.chip0, R.id.chip0_name, R.id.chip0_proto), Triple(R.id.chip1, R.id.chip1_name, R.id.chip1_proto), Triple(R.id.chip2, R.id.chip2_name, R.id.chip2_proto))
            ids.forEachIndexed { i, (box, n, p) ->
                val prof = list.getOrNull(i)
                if (prof == null) { v.setViewVisibility(box, View.INVISIBLE); return@forEachIndexed }
                v.setViewVisibility(box, View.VISIBLE)
                val sel = prof.id == activeId
                v.setInt(box, "setBackgroundResource", when {
                    sel && dark -> R.drawable.wg_chip_sel_dark
                    sel -> R.drawable.wg_chip_sel_light
                    dark -> R.drawable.wg_chip_dark
                    else -> R.drawable.wg_chip_light
                })
                text(v, n, prof.name, ink)
                text(v, p, protoTitle(prof.protocol), muted)
                v.setOnClickPendingIntent(box, PendingIntent.getBroadcast(ctx, 100 + i,
                    Intent(ctx, VeilWidget::class.java).setAction(ACTION_SELECT).putExtra(EXTRA_PROFILE, prof.id),
                    PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT))
            }
        }

        private fun text(v: RemoteViews, id: Int, s: String, color: Int) { v.setTextViewText(id, s); v.setTextColor(id, color) }

        private fun toggleIntent(ctx: Context) = PendingIntent.getBroadcast(ctx, 1,
            Intent(ctx, VeilWidget::class.java).setAction(ACTION_TOGGLE), PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)

        private fun openIntent(ctx: Context) = PendingIntent.getActivity(ctx, 2,
            Intent(ctx, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
    }
}
