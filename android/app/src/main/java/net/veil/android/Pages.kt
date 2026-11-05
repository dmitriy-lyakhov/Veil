package net.veil.android

import android.content.Intent
import android.graphics.Canvas
import android.graphics.Paint
import android.graphics.Path
import android.provider.Settings as AndroidSettings
import android.view.Gravity
import android.view.View
import android.widget.EditText
import android.widget.FrameLayout
import android.widget.LinearLayout
import android.widget.TextView
import veilcore.Veilcore

private fun protoName(a: MainActivity, p: String) = when (p) {
    "wireguard" -> "WireGuard"
    "amneziawg" -> "AmneziaWG"
    else -> "Veil"
}

// =============================== Главная ===============================

class HomePage(a: MainActivity) : Page(a) {
    override val scroll = false
    private lateinit var ring: RingView
    private lateinit var pillDot: View
    private lateinit var pillText: TextView
    private lateinit var titleView: TextView
    private lateinit var hint: TextView
    private lateinit var skipped: TextView
    private lateinit var splitText: TextView
    private lateinit var serverName: TextView
    private lateinit var serverSub: TextView

    override fun build(): View {
        val col = a.vbox()
        // верхняя строка: логотип, название, плашка статуса
        val top = a.hbox().apply { setPadding(dp(20), dp(10), dp(20), dp(10)) }
        top.addView(a.icon(R.drawable.ic_mark, 28f, T.p.ink).also { it.clearColorFilter() })
        top.addView(a.text("Veil", 20f, 600).also { it.setPadding(dp(10), 0, 0, 0) }, lp(0, WRAP, 1f))
        val pill = a.hbox().apply { setPadding(dp(14), dp(10), dp(14), dp(10)); background = rounded(T.p.raised, 14f) }
        pillDot = View(a); pill.addView(pillDot, lp(dp(10), dp(10)).apply { rightMargin = dp(8) })
        pillText = a.text("", 13f, 600); pill.addView(pillText)
        top.addView(pill)
        col.addView(top, lp(MATCH, WRAP))

        val mid = a.vbox().apply { gravity = Gravity.CENTER_HORIZONTAL }
        mid.addView(a.space(8f))
        ring = RingView(a)
        ring.setOnClickListener { a.toggleConnect() }
        mid.addView(ring, lp(WRAP, WRAP))
        titleView = a.text("", 16f, 600); titleView.gravity = Gravity.CENTER
        hint = a.text("", 13f, 500, T.p.muted); hint.gravity = Gravity.CENTER
        skipped = a.text("", 13f, 500, T.p.accentInk); skipped.gravity = Gravity.CENTER
        mid.addView(titleView, lp(MATCH, WRAP).apply { leftMargin = dp(48); rightMargin = dp(48) })
        mid.addView(hint, lp(MATCH, WRAP).apply { topMargin = dp(4); leftMargin = dp(48); rightMargin = dp(48) })
        mid.addView(skipped, lp(MATCH, WRAP).apply { topMargin = dp(4); leftMargin = dp(48); rightMargin = dp(48) })
        val split = a.hbox().apply {
            setPadding(dp(16), dp(12), dp(16), dp(12)); ripple(rounded(T.p.raised, 14f))
            setOnClickListener { a.push(SplitPage(a)) }
        }
        split.addView(a.icon(R.drawable.ic_split, 18f).also { (it.layoutParams as LinearLayout.LayoutParams).rightMargin = dp(10) })
        splitText = a.text("", 13f, 500); split.addView(splitText)
        split.addView(a.icon(R.drawable.ic_right, 18f, T.p.muted).also { (it.layoutParams as LinearLayout.LayoutParams).leftMargin = dp(10) })
        mid.addView(split, lp(WRAP, WRAP).apply { topMargin = dp(16) })
        col.addView(mid, lp(MATCH, 0, 1f))

        // нижняя панель с выбором сервера
        val panel = a.vbox().apply {
            background = android.graphics.drawable.GradientDrawable().apply {
                setColor(T.p.raised); val r = dp(16).toFloat(); cornerRadii = floatArrayOf(r, r, r, r, 0f, 0f, 0f, 0f)
            }
            setPadding(dp(20), dp(12), dp(20), dp(14))
            gravity = Gravity.CENTER_HORIZONTAL
            setOnClickListener { openServers() }
        }
        panel.addView(View(a).apply { background = rounded(T.p.high, 3f) }, lp(dp(44), dp(5)))
        val srv = a.hbox().apply { gravity = Gravity.CENTER; setPadding(0, dp(12), 0, 0) }
        val sc = a.vbox().apply { gravity = Gravity.CENTER_HORIZONTAL }
        val line = a.hbox().apply { gravity = Gravity.CENTER }
        serverName = a.text("", 28f, 600, lines = 1)
        line.addView(serverName, lp(WRAP, WRAP))
        line.addView(a.icon(R.drawable.ic_down, 22f, T.p.muted).also { (it.layoutParams as LinearLayout.LayoutParams).leftMargin = dp(8) })
        sc.addView(line)
        serverSub = a.text("", 13f, 500, T.p.muted, lines = 1); serverSub.setPadding(0, dp(6), 0, 0)
        serverSub.gravity = Gravity.CENTER
        sc.addView(serverSub, lp(MATCH, WRAP))
        srv.addView(sc, lp(MATCH, WRAP))
        panel.addView(srv, lp(MATCH, WRAP))
        col.addView(panel, lp(MATCH, WRAP))
        return col
    }

    private fun openServers() {
        val body = a.vbox()
        val list = Profiles.all(a)
        val act = Profiles.active(a)?.id
        if (list.isEmpty()) {
            body.addView(a.text(a.getString(R.string.no_profiles), 14f, 500, T.p.muted))
            body.addView(a.space(12f))
        }
        for (p in list) {
            val dot = View(a).apply { background = if (p.id == act) rounded(T.p.accent, 12f) else rounded(T.p.high, 12f, T.p.muted, 1f) }
            val trailing = FrameLayout(a).apply { addView(dot, FrameLayout.LayoutParams(dp(14), dp(14), Gravity.CENTER)); layoutParams = LinearLayout.LayoutParams(dp(24), dp(24)) }
            body.addView(a.row(p.name, protoName(a, p.protocol), null, trailing, bg = T.p.high) { a.switchProfile(p.id) }, lp(MATCH, WRAP).apply { bottomMargin = dp(8) })
        }
        if (a.running() && Profiles.active(a)?.protocol == "veil") {
            body.addView(a.row(a.getString(R.string.reselect), a.getString(R.string.reselect_sub), R.drawable.ic_server, bg = T.p.high) { Veilcore.reselect(); a.closeSheet() }, lp(MATCH, WRAP).apply { bottomMargin = dp(8) })
        }
        body.addView(a.row(a.getString(R.string.add_server), null, R.drawable.ic_plus, bg = T.p.high) { a.selectTab(3) }, lp(MATCH, WRAP))
        a.openSheet(a.getString(R.string.choose_server), body)
    }

    override fun refresh() {
        val st = a.status
        val state = st.optString("state", "stopped")
        val up = st.optBoolean("up")
        val err = Store.lastError
        val rs: RingState; val ring0: String; val hint0: String; val pill: String
        when {
            a.disconnecting -> { rs = RingState.DISCONNECTING; ring0 = a.getString(R.string.ring_disconnecting); hint0 = ""; pill = a.getString(R.string.st_disconnecting) }
            state == "stopped" && a.pending -> { rs = RingState.CONNECTING; ring0 = a.getString(R.string.ring_connecting); hint0 = a.getString(R.string.hint_connecting); pill = a.getString(R.string.st_connecting) }
            state == "stopped" && err != null -> { rs = RingState.ERROR; ring0 = a.getString(R.string.ring_retry); hint0 = a.errorText(err); pill = a.getString(R.string.st_error) }
            state == "stopped" -> { rs = RingState.IDLE; ring0 = a.getString(R.string.ring_connect); hint0 = a.getString(R.string.hint_idle); pill = a.getString(R.string.st_off) }
            up && state == "connected" -> { rs = RingState.CONNECTED; ring0 = a.getString(R.string.ring_protected); hint0 = listOf(st.optString("transport"), st.optString("entry")).filter { it.isNotBlank() }.joinToString(" · "); pill = a.getString(R.string.st_on) }
            state == "reconnecting" -> { rs = RingState.CONNECTING; ring0 = a.getString(R.string.ring_connecting); hint0 = st.optString("last_error"); pill = a.getString(R.string.st_reconnecting) }
            else -> { rs = RingState.CONNECTING; ring0 = a.getString(R.string.ring_connecting); hint0 = a.getString(R.string.hint_connecting); pill = a.getString(R.string.st_connecting) }
        }
        ring.setState(rs); ring.label = ring0
        titleView.text = when (rs) {
            RingState.IDLE -> a.getString(R.string.st_off)
            RingState.CONNECTED -> a.getString(R.string.st_on)
            RingState.ERROR -> a.getString(R.string.st_error)
            RingState.DISCONNECTING -> a.getString(R.string.st_disconnecting)
            else -> a.getString(R.string.st_connecting)
        }
        hint.text = hint0
        pillText.text = pill
        pillDot.background = rounded(
            when (rs) { RingState.CONNECTED -> T.p.success; RingState.ERROR -> T.p.accentInk; else -> T.p.muted }, 10f)
        val sk = st.optInt("skipped_entries")
        skipped.text = if (sk > 0 && rs == RingState.CONNECTED) a.getString(R.string.entries_skipped, sk) else ""
        val n = Settings.splitCount(a)
        splitText.text = if (Settings.splitActive(a)) a.getString(R.string.split_n, n) else a.getString(R.string.split_title)
        val prof = Profiles.active(a)
        serverName.text = prof?.name ?: a.getString(R.string.no_server)
        serverSub.text = if (prof == null) a.getString(R.string.add_first) else protoName(a, prof.protocol)
    }
}

// =============================== Трафик ===============================

class TrafficGraph(val a: MainActivity) : View(a) {
    private val p = Paint(Paint.ANTI_ALIAS_FLAG).apply { style = Paint.Style.STROKE; strokeWidth = dp(2).toFloat(); strokeJoin = Paint.Join.MITER }
    private val g = Paint().apply { color = T.p.high; strokeWidth = dp(1).toFloat() }
    override fun onMeasure(w: Int, h: Int) = setMeasuredDimension(MeasureSpec.getSize(w), dp(160))
    private fun path(h: List<Float>, max: Float): Path {
        val path = Path()
        val w = width.toFloat(); val hh = height.toFloat()
        val step = w / 59f
        val off = 60 - h.size
        h.forEachIndexed { i, v ->
            val x = (i + off) * step; val y = hh - 4 - (v / max) * (hh - 12)
            if (i == 0) path.moveTo(x, y) else path.lineTo(x, y)
        }
        return path
    }
    override fun onDraw(c: Canvas) {
        for (i in 0..3) c.drawLine(0f, height * i / 3f, width.toFloat(), height * i / 3f, g)
        val rx = a.rxHist.toList(); val tx = a.txHist.toList()
        val max = maxOf(1024f, (rx + tx).maxOrNull() ?: 0f)
        if (rx.size > 1) { p.color = T.p.accent; c.drawPath(path(rx, max), p) }
        if (tx.size > 1) { p.color = T.p.ink; c.drawPath(path(tx, max), p) }
    }
}

class TrafficPage(a: MainActivity) : Page(a) {
    override val title get() = a.getString(R.string.traffic_title)
    private lateinit var graph: TrafficGraph
    private lateinit var speed: TextView
    private val cells = HashMap<String, TextView>()

    override fun build(): View {
        val col = a.vbox().apply { setPadding(dp(20), dp(8), dp(20), dp(20)) }
        val card = a.vbox().apply { background = rounded(T.p.raised, 16f); setPadding(dp(16), dp(16), dp(16), dp(16)) }
        speed = a.text("", 24f, 700)
        card.addView(speed)
        card.addView(a.text(a.getString(R.string.traffic_last60), 13f, 500, T.p.muted).also { it.setPadding(0, dp(2), 0, dp(10)) })
        graph = TrafficGraph(a)
        card.addView(graph, lp(MATCH, WRAP))
        val lg = a.hbox().apply { setPadding(0, dp(10), 0, 0) }
        lg.addView(View(a).apply { background = rounded(T.p.accent, 2f) }, lp(dp(14), dp(4)))
        lg.addView(a.text(a.getString(R.string.rx), 13f, 500, T.p.muted).also { it.setPadding(dp(6), 0, dp(16), 0) })
        lg.addView(View(a).apply { background = rounded(T.p.ink, 2f) }, lp(dp(14), dp(4)))
        lg.addView(a.text(a.getString(R.string.tx), 13f, 500, T.p.muted).also { it.setPadding(dp(6), 0, 0, 0) })
        card.addView(lg)
        col.addView(card, lp(MATCH, WRAP))
        for ((key, label) in listOf("session" to R.string.traffic_session, "day" to R.string.traffic_day, "month" to R.string.traffic_month)) {
            col.addView(a.space(12f))
            val r = a.vbox().apply { background = rounded(T.p.raised, 14f); setPadding(dp(16), dp(14), dp(16), dp(14)) }
            r.addView(a.text(a.getString(label), 13f, 500, T.p.muted))
            val v = a.text("", 16f, 700); v.setPadding(0, dp(4), 0, 0)
            cells[key] = v; r.addView(v)
            col.addView(r, lp(MATCH, WRAP))
        }
        return col
    }

    override fun refresh() {
        val rx = a.rxHist.lastOrNull() ?: 0f; val tx = a.txHist.lastOrNull() ?: 0f
        speed.text = "↓ ${fmtBytes(rx.toLong())}/s   ↑ ${fmtBytes(tx.toLong())}/s"
        graph.invalidate()
        val st = a.status
        cells["session"]?.text = "↓ ${fmtBytes(st.optLong("rx"))}   ↑ ${fmtBytes(st.optLong("tx"))}"
        val d = Stats.today(a); val m = Stats.month(a)
        cells["day"]?.text = "↓ ${fmtBytes(d.first)}   ↑ ${fmtBytes(d.second)}"
        cells["month"]?.text = "↓ ${fmtBytes(m.first)}   ↑ ${fmtBytes(m.second)}"
    }
}

// =============================== Настройки ===============================

private fun sectionHeader(a: MainActivity, s: String) = a.text(s.uppercase(), 12f, 600, T.p.muted).also { it.setPadding(dp(4), dp(18), 0, dp(8)) }

class SettingsPage(a: MainActivity) : Page(a) {
    override val title get() = a.getString(R.string.tab_settings)
    override fun build(): View {
        val col = a.vbox().apply { setPadding(dp(20), dp(0), dp(20), dp(20)) }
        fun add(v: View) { col.addView(v, lp(MATCH, WRAP).apply { bottomMargin = dp(8) }) }
        col.addView(sectionHeader(a, a.getString(R.string.sec_servers)))
        add(a.row(a.getString(R.string.servers), a.getString(R.string.servers_count, Profiles.all(a).size), R.drawable.ic_server, chevron = true) { a.push(ProfilesPage(a)) })
        col.addView(sectionHeader(a, a.getString(R.string.sec_connection)))
        val dns = Settings.dns(a)
        add(a.row(a.getString(R.string.dns), if (dns.isBlank()) a.getString(R.string.dns_auto) else dns, R.drawable.ic_shield, chevron = true) { dnsSheet() })
        add(a.row(a.getString(R.string.split_title), if (Settings.splitActive(a)) a.getString(R.string.split_n, Settings.splitCount(a)) else a.getString(R.string.off), R.drawable.ic_split, chevron = true) { a.push(SplitPage(a)) })
        add(a.row(a.getString(R.string.killswitch), a.getString(R.string.ks_sub), R.drawable.ic_shield, chevron = true) { a.push(KillSwitchPage(a)) })
        col.addView(sectionHeader(a, a.getString(R.string.sec_app)))
        add(a.row(a.getString(R.string.auto_app), null, null, Toggle(a, Settings.autoApp(a)) { Settings.setBool(a, "auto_app", it) }) {})
        add(a.row(a.getString(R.string.auto_boot), null, null, Toggle(a, Settings.autoBoot(a)) { Settings.setBool(a, "auto_boot", it) }) {})
        add(a.row(a.getString(R.string.logs), null, R.drawable.ic_info, chevron = true) { a.push(LogsPage(a)) })
        col.addView(sectionHeader(a, a.getString(R.string.sec_look)))
        add(a.row(a.getString(R.string.theme), themeName(Settings.theme(a)), null, chevron = true) { choose(a.getString(R.string.theme), listOf("system" to R.string.theme_system, "dark" to R.string.theme_dark, "light" to R.string.theme_light), Settings.theme(a), "theme") })
        add(a.row(a.getString(R.string.language), langName(Settings.lang(a)), null, chevron = true) { choose(a.getString(R.string.language), listOf("system" to R.string.lang_system, "ru" to R.string.lang_ru, "en" to R.string.lang_en), Settings.lang(a), "lang") })
        add(a.row(a.getString(R.string.about), null, R.drawable.ic_info, chevron = true) { a.push(AboutPage(a)) })
        return col
    }

    private fun themeName(v: String) = a.getString(when (v) { "dark" -> R.string.theme_dark; "light" -> R.string.theme_light; else -> R.string.theme_system })
    private fun langName(v: String) = a.getString(when (v) { "ru" -> R.string.lang_ru; "en" -> R.string.lang_en; else -> R.string.lang_system })

    private fun choose(title: String, opts: List<Pair<String, Int>>, cur: String, key: String) {
        val body = a.vbox()
        for ((v, label) in opts) {
            val dot = View(a).apply { background = if (v == cur) rounded(T.p.accent, 12f) else rounded(T.p.high, 12f, T.p.muted, 1f) }
            val tr = FrameLayout(a).apply { addView(dot, FrameLayout.LayoutParams(dp(14), dp(14), Gravity.CENTER)); layoutParams = LinearLayout.LayoutParams(dp(24), dp(24)) }
            body.addView(a.row(a.getString(label), null, null, tr, bg = T.p.high) { Settings.set(a, key, v); a.closeSheet(); a.applyAppearance() }, lp(MATCH, WRAP).apply { bottomMargin = dp(8) })
        }
        a.openSheet(title, body)
    }

    private fun dnsSheet() {
        val body = a.vbox()
        val et = a.input("1.1.1.1, 8.8.8.8", Settings.dns(a))
        body.addView(a.text(a.getString(R.string.dns_hint), 13f, 500, T.p.muted).also { it.setPadding(0, 0, 0, dp(10)) })
        body.addView(et, lp(MATCH, WRAP))
        body.addView(a.space(12f))
        body.addView(a.button(a.getString(R.string.save)) {
            Settings.set(a, "dns_override", et.text.toString().trim()); a.closeSheet(); a.rebuild()
        })
        a.openSheet(a.getString(R.string.dns), body)
    }
}

// =============================== Профили ===============================

class ProfilesPage(a: MainActivity) : Page(a) {
    override val title get() = a.getString(R.string.servers)
    override fun build(): View {
        val col = a.vbox().apply { setPadding(dp(20), dp(0), dp(20), dp(20)) }
        val list = Profiles.all(a); val act = Profiles.active(a)?.id
        if (list.isEmpty()) col.addView(a.text(a.getString(R.string.no_profiles), 14f, 500, T.p.muted).also { it.setPadding(0, dp(8), 0, dp(12)) })
        for (p in list) {
            val dot = View(a).apply { background = if (p.id == act) rounded(T.p.accent, 12f) else rounded(T.p.high, 12f, T.p.muted, 1f) }
            val tr = FrameLayout(a).apply { addView(dot, FrameLayout.LayoutParams(dp(14), dp(14), Gravity.CENTER)); layoutParams = LinearLayout.LayoutParams(dp(24), dp(24)) }
            col.addView(a.row(p.name, protoName(a, p.protocol), null, tr) { actions(p) }, lp(MATCH, WRAP).apply { bottomMargin = dp(8) })
        }
        col.addView(a.space(8f))
        col.addView(a.button(a.getString(R.string.add_server)) { a.selectTab(3) }, lp(MATCH, WRAP))
        col.addView(sectionHeader(a, a.getString(R.string.backup)))
        col.addView(a.text(a.getString(R.string.backup_hint), 13f, 500, T.p.muted).also { it.setPadding(dp(4), 0, 0, dp(10)) })
        col.addView(a.row(a.getString(R.string.backup_export), null, R.drawable.ic_download, bg = T.p.raised) { a.exportBackup() }, lp(MATCH, WRAP).apply { bottomMargin = dp(8) })
        col.addView(a.row(a.getString(R.string.backup_import), null, R.drawable.ic_plus, bg = T.p.raised) { a.importBackupFile() }, lp(MATCH, WRAP))
        return col
    }

    private fun actions(p: Profile) {
        val body = a.vbox()
        body.addView(a.row(a.getString(R.string.make_active), null, R.drawable.ic_check, bg = T.p.high) { a.switchProfile(p.id) }, lp(MATCH, WRAP).apply { bottomMargin = dp(8) })
        body.addView(a.row(a.getString(R.string.rename), null, R.drawable.ic_info, bg = T.p.high) { renameSheet(p) }, lp(MATCH, WRAP).apply { bottomMargin = dp(8) })
        body.addView(a.row(a.getString(R.string.delete), null, R.drawable.ic_close, bg = T.p.high) { confirmDelete(p) }, lp(MATCH, WRAP))
        a.openSheet(p.name, body)
    }

    private fun renameSheet(p: Profile) {
        val body = a.vbox()
        val et = a.input(a.getString(R.string.name_hint), p.name)
        body.addView(et, lp(MATCH, WRAP)); body.addView(a.space(12f))
        body.addView(a.button(a.getString(R.string.save)) {
            try { Profiles.rename(a, p.id, et.text.toString()) } catch (_: Exception) { a.toast(a.getString(R.string.err_empty)) }
            a.closeSheet(); a.rebuild()
        })
        a.openSheet(a.getString(R.string.rename), body)
    }

    private fun confirmDelete(p: Profile) {
        val body = a.vbox()
        body.addView(a.text(a.getString(R.string.delete_q, p.name), 14f, 500, T.p.muted).also { it.setPadding(0, 0, 0, dp(14)) })
        body.addView(a.button(a.getString(R.string.delete)) {
            val wasActive = Profiles.active(a)?.id == p.id
            Profiles.delete(a, p.id)
            if (wasActive && a.running()) VeilVpnService.disconnect(a)
            a.closeSheet(); a.rebuild()
        })
        a.openSheet(a.getString(R.string.delete), body)
    }
}

// =============================== Добавить ===============================

class AddPage(a: MainActivity) : Page(a) {
    override val title get() = a.getString(R.string.add_title)
    override fun build(): View {
        val col = a.vbox().apply { setPadding(dp(20), dp(4), dp(20), dp(20)) }
        col.addView(a.text(a.getString(R.string.add_hint), 13f, 500, T.p.muted).also { it.setPadding(0, 0, 0, dp(12)) })
        val name = a.input(a.getString(R.string.name_hint))
        val link = a.input(a.getString(R.string.link_hint), "", true)
        col.addView(name, lp(MATCH, WRAP)); col.addView(a.space(10f))
        col.addView(link, lp(MATCH, WRAP)); col.addView(a.space(10f))
        col.addView(a.row(a.getString(R.string.paste), a.getString(R.string.paste_sub), R.drawable.ic_download) {
            val t = a.clipboardText()
            if (t.isEmpty()) a.toast(a.getString(R.string.clip_empty)) else link.setText(t)
        }, lp(MATCH, WRAP).apply { bottomMargin = dp(8) })
        col.addView(a.row(a.getString(R.string.scan_qr), a.getString(R.string.scan_sub), R.drawable.ic_qr) { a.scanQr() }, lp(MATCH, WRAP).apply { bottomMargin = dp(8) })
        col.addView(a.row(a.getString(R.string.pick_file), a.getString(R.string.pick_sub), R.drawable.ic_server) { a.pickFile() }, lp(MATCH, WRAP).apply { bottomMargin = dp(16) })
        col.addView(a.button(a.getString(R.string.add_btn)) {
            val t = link.text.toString().trim()
            if (t.isEmpty()) a.toast(a.getString(R.string.err_empty)) else a.importText(t, name.text.toString())
        }, lp(MATCH, WRAP))
        col.addView(a.text(a.getString(R.string.protocols_note), 12f, 500, T.p.muted).also { it.setPadding(dp(4), dp(16), 0, 0) })
        return col
    }
}

// =============================== Раздельное туннелирование ===============================

class SplitPage(a: MainActivity) : Page(a) {
    override val title get() = a.getString(R.string.split_title)
    override fun build(): View {
        val col = a.vbox().apply { setPadding(dp(20), dp(0), dp(20), dp(20)) }
        col.addView(a.text(a.getString(R.string.split_hint), 13f, 500, T.p.muted).also { it.setPadding(dp(4), 0, 0, dp(14)) })
        col.addView(a.row(a.getString(R.string.split_sites), if (Settings.sitesOn(a)) a.getString(R.string.on_n, Settings.sites(a).size) else a.getString(R.string.off), null, chevron = true) { a.push(SitesPage(a)) }, lp(MATCH, WRAP).apply { bottomMargin = dp(8) })
        col.addView(a.row(a.getString(R.string.split_apps), if (Settings.appsOn(a)) a.getString(R.string.on_n, Settings.apps(a).size) else a.getString(R.string.off), null, chevron = true) { a.push(AppsPage(a)) }, lp(MATCH, WRAP))
        col.addView(a.text(a.getString(R.string.split_note), 12f, 500, T.p.muted).also { it.setPadding(dp(4), dp(14), 0, 0) })
        return col
    }
}

class SitesPage(a: MainActivity) : Page(a) {
    override val title get() = a.getString(R.string.split_sites_title)
    override fun build(): View {
        val col = a.vbox().apply { setPadding(dp(20), dp(0), dp(20), dp(20)) }
        col.addView(a.row(a.getString(R.string.split_enable), null, null, Toggle(a, Settings.sitesOn(a)) { Settings.setBool(a, "split_sites_on", it) }) {}, lp(MATCH, WRAP).apply { bottomMargin = dp(10) })
        val mode = if (Settings.sitesMode(a) == "via") 0 else 1
        col.addView(a.segmented(listOf(a.getString(R.string.via_vpn), a.getString(R.string.direct)), mode) { Settings.set(a, "split_sites_mode", if (it == 0) "via" else "direct"); a.rebuild() }, lp(MATCH, WRAP))
        col.addView(a.text(a.getString(if (mode == 0) R.string.sites_via_hint else R.string.sites_direct_hint), 13f, 500, T.p.muted).also { it.setPadding(dp(4), dp(10), 0, dp(12)) })
        val sites = Settings.sites(a)
        for ((i, s) in sites.withIndex()) {
            val x = a.icon(R.drawable.ic_close, 20f, T.p.muted)
            x.setOnClickListener { Settings.setSites(a, sites.filterIndexed { j, _ -> j != i }); a.rebuild() }
            col.addView(a.row(s, null, null, x, bg = T.p.raised) {}, lp(MATCH, WRAP).apply { bottomMargin = dp(8) })
        }
        val add = a.hbox()
        val et = a.input(a.getString(R.string.site_hint))
        add.addView(et, lp(0, WRAP, 1f))
        val plus = a.icon(R.drawable.ic_plus, 22f, T.p.onAccent).apply {
            background = rounded(T.p.accent, 12f); setPadding(dp(12), dp(12), dp(12), dp(12))
            layoutParams = LinearLayout.LayoutParams(dp(48), dp(48)).apply { leftMargin = dp(8) }
        }
        plus.setOnClickListener {
            val t = et.text.toString().trim()
            if (t.isNotEmpty()) { Settings.setSites(a, sites + t.lines().map { it.trim() }.filter { it.isNotEmpty() }); a.rebuild() }
        }
        add.addView(plus)
        col.addView(a.space(4f)); col.addView(add, lp(MATCH, WRAP))
        col.addView(a.text(a.getString(R.string.sites_note), 12f, 500, T.p.muted).also { it.setPadding(dp(4), dp(14), 0, 0) })
        return col
    }
}

class AppsPage(a: MainActivity) : Page(a) {
    override val title get() = a.getString(R.string.split_apps_title)
    override val scroll = false
    private class AppItem(val pkg: String, val label: String)
    private var all: List<AppItem> = emptyList()
    private var shown: List<AppItem> = emptyList()
    private val sel = HashSet(Settings.apps(a))
    private lateinit var adapter: android.widget.BaseAdapter

    override fun build(): View {
        val pm = a.packageManager
        val q = Intent(Intent.ACTION_MAIN).addCategory(Intent.CATEGORY_LAUNCHER)
        all = pm.queryIntentActivities(q, 0).map { AppItem(it.activityInfo.packageName, it.loadLabel(pm).toString()) }
            .filter { it.pkg != a.packageName }.distinctBy { it.pkg }.sortedBy { it.label.lowercase() }
        shown = all
        val col = a.vbox().apply { setPadding(dp(20), dp(0), dp(20), dp(8)) }
        col.addView(a.row(a.getString(R.string.split_enable), null, null, Toggle(a, Settings.appsOn(a)) { Settings.setBool(a, "split_apps_on", it) }) {}, lp(MATCH, WRAP).apply { bottomMargin = dp(10) })
        val mode = if (Settings.appsMode(a) == "via") 0 else 1
        col.addView(a.segmented(listOf(a.getString(R.string.via_vpn), a.getString(R.string.direct)), mode) { Settings.set(a, "split_apps_mode", if (it == 0) "via" else "direct"); persist(); a.rebuild() }, lp(MATCH, WRAP))
        col.addView(a.text(a.getString(if (mode == 0) R.string.apps_via_hint else R.string.apps_direct_hint), 13f, 500, T.p.muted).also { it.setPadding(dp(4), dp(10), 0, dp(10)) })
        val search = a.input(a.getString(R.string.search_apps))
        col.addView(search, lp(MATCH, WRAP))
        col.addView(a.space(8f))
        val lv = android.widget.ListView(a).apply { divider = null; setSelector(android.R.color.transparent); isVerticalScrollBarEnabled = false }
        adapter = object : android.widget.BaseAdapter() {
            override fun getCount() = shown.size
            override fun getItem(i: Int) = shown[i]
            override fun getItemId(i: Int) = i.toLong()
            override fun getView(i: Int, cv: View?, parent: android.view.ViewGroup?): View {
                val it = shown[i]
                val row = a.hbox().apply { setPadding(dp(4), dp(8), dp(4), dp(8)) }
                val img = android.widget.ImageView(a)
                try { img.setImageDrawable(pm.getApplicationIcon(it.pkg)) } catch (_: Exception) { }
                row.addView(img, lp(dp(36), dp(36)).apply { rightMargin = dp(14) })
                val c = a.vbox()
                c.addView(a.text(it.label, 15f, 600, lines = 1))
                c.addView(a.text(it.pkg, 12f, 500, T.p.muted, lines = 1))
                row.addView(c, lp(0, WRAP, 1f))
                val on = sel.contains(it.pkg)
                row.addView(View(a).apply {
                    background = if (on) rounded(T.p.accent, 6f) else rounded(T.p.high, 6f, T.p.muted, 1f)
                }, lp(dp(22), dp(22)))
                row.setOnClickListener { if (!sel.add(it0(i))) sel.remove(it0(i)); persist(); adapter.notifyDataSetChanged() }
                return row
            }
            private fun it0(i: Int) = shown[i].pkg
        }
        lv.adapter = adapter
        search.addTextChangedListener(object : android.text.TextWatcher {
            override fun afterTextChanged(s: android.text.Editable?) {
                val q2 = s.toString().trim().lowercase()
                shown = if (q2.isEmpty()) all else all.filter { it.label.lowercase().contains(q2) || it.pkg.contains(q2) }
                adapter.notifyDataSetChanged()
            }
            override fun beforeTextChanged(s: CharSequence?, st: Int, c: Int, af: Int) {}
            override fun onTextChanged(s: CharSequence?, st: Int, b: Int, c: Int) {}
        })
        col.addView(lv, lp(MATCH, 0, 1f))
        return col
    }

    private fun persist() = Settings.setApps(a, sel)
}

// =============================== KillSwitch, журналы, о приложении ===============================

class KillSwitchPage(a: MainActivity) : Page(a) {
    override val title get() = a.getString(R.string.killswitch)
    override fun build(): View {
        val col = a.vbox().apply { setPadding(dp(20), dp(0), dp(20), dp(20)) }
        col.addView(a.text(a.getString(R.string.ks_text), 14f, 500, T.p.muted).also { it.setPadding(dp(4), 0, 0, dp(16)) })
        val always = try { AndroidSettings.Secure.getString(a.contentResolver, "always_on_vpn_app") } catch (_: Exception) { null }
        val lock = try { AndroidSettings.Secure.getInt(a.contentResolver, "always_on_vpn_lockdown", 0) == 1 } catch (_: Exception) { false }
        if (always != null) {
            col.addView(a.row(a.getString(R.string.ks_alwayson), if (always == a.packageName) a.getString(R.string.on) else a.getString(R.string.off)) {}, lp(MATCH, WRAP).apply { bottomMargin = dp(8) })
            if (always == a.packageName) col.addView(a.row(a.getString(R.string.ks_lockdown), if (lock) a.getString(R.string.on) else a.getString(R.string.off)) {}, lp(MATCH, WRAP).apply { bottomMargin = dp(8) })
        }
        col.addView(a.space(8f))
        col.addView(a.button(a.getString(R.string.ks_open)) {
            try { a.startActivity(Intent(AndroidSettings.ACTION_VPN_SETTINGS)) } catch (_: Exception) { a.startActivity(Intent(AndroidSettings.ACTION_WIRELESS_SETTINGS)) }
        }, lp(MATCH, WRAP))
        return col
    }
}

class LogsPage(a: MainActivity) : Page(a) {
    override val title get() = a.getString(R.string.logs)
    private var text = ""
    override fun build(): View {
        val col = a.vbox().apply { setPadding(dp(20), dp(0), dp(20), dp(20)) }
        text = try {
            val p = Runtime.getRuntime().exec(arrayOf("logcat", "-d", "-v", "time", "-s", "VeilVpn:*", "GoLog:*"))
            p.inputStream.bufferedReader().readLines().filter { !it.startsWith("---") }.takeLast(300).joinToString("\n")
        } catch (_: Exception) { "" }
        if (text.isBlank()) text = a.getString(R.string.logs_empty)
        val tv = a.text(text, 11f, 500, T.p.muted)
        tv.typeface = android.graphics.Typeface.MONOSPACE
        tv.setTextIsSelectable(true)
        tv.background = rounded(T.p.raised, 12f); tv.setPadding(dp(12), dp(12), dp(12), dp(12))
        col.addView(tv, lp(MATCH, WRAP))
        col.addView(a.space(12f))
        col.addView(a.button(a.getString(R.string.share)) {
            a.startActivity(Intent.createChooser(Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT, text), null))
        }, lp(MATCH, WRAP))
        return col
    }
}

class AboutPage(a: MainActivity) : Page(a) {
    override val title get() = a.getString(R.string.about)
    override fun build(): View {
        val col = a.vbox().apply { setPadding(dp(20), dp(20), dp(20), dp(20)); gravity = Gravity.CENTER_HORIZONTAL }
        col.addView(a.icon(R.drawable.ic_mark, 72f).also { it.clearColorFilter() })
        col.addView(a.text("Veil", 28f, 600).also { it.setPadding(0, dp(12), 0, dp(4)) })
        val v = try { a.packageManager.getPackageInfo(a.packageName, 0).versionName } catch (_: Exception) { "" }
        col.addView(a.text(a.getString(R.string.about_version, v ?: ""), 13f, 500, T.p.muted))
        col.addView(a.text(a.getString(R.string.about_text), 14f, 500, T.p.muted).also { it.gravity = Gravity.CENTER; it.setPadding(0, dp(20), 0, 0) })
        return col
    }
}
