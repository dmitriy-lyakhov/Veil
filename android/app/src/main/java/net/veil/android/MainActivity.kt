package net.veil.android

import android.app.Activity
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.net.VpnService
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.view.Gravity
import android.view.View
import android.view.WindowInsetsController
import android.widget.FrameLayout
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast
import com.google.zxing.integration.android.IntentIntegrator
import org.json.JSONObject
import veilcore.Veilcore

/** Страница внутри вкладки; sub — подстраница (с кнопкой «назад»). */
abstract class Page(val a: MainActivity) {
    open val title: String? = null
    open val scroll = true
    abstract fun build(): View
    open fun refresh() {}
}

class MainActivity : Activity() {

    private lateinit var content: FrameLayout
    private lateinit var navBar: LinearLayout
    private lateinit var overlay: FrameLayout
    private val handler = Handler(Looper.getMainLooper())
    private val stack = ArrayList<Page>()
    private var tab = 0
    private var navIcons = ArrayList<Pair<android.widget.ImageView, View>>()

    // состояние подключения для интерфейса
    var pending = false
    var disconnecting = false
    private var disconnectingSince = 0L
    private var autoDone = false

    // скорость: последние 60 отсчётов (байт/с)
    val rxHist = ArrayDeque<Float>()
    val txHist = ArrayDeque<Float>()
    private var lastRx = -1L
    private var lastTx = -1L
    var status: JSONObject = JSONObject().put("state", "stopped")

    override fun attachBaseContext(base: Context) = super.attachBaseContext(withLocale(base))

    private val poll = object : Runnable {
        override fun run() {
            tick()
            handler.postDelayed(this, 1000)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        T.init(this)
        val root = FrameLayout(this).apply { setBackgroundColor(T.p.surface) }
        val col = vbox()
        content = FrameLayout(this)
        col.addView(content, lp(MATCH, 0, 1f))
        navBar = buildNav()
        col.addView(navBar, lp(MATCH, WRAP))
        root.addView(col, FrameLayout.LayoutParams(MATCH, MATCH))
        overlay = FrameLayout(this)
        overlay.visibility = View.GONE
        root.addView(overlay, FrameLayout.LayoutParams(MATCH, MATCH))
        setContentView(root)
        applyBars()
        show(rootPage(0))
        handleIntent(intent)
        if (Build.VERSION.SDK_INT >= 33 && checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(arrayOf(android.Manifest.permission.POST_NOTIFICATIONS), 2)
        }
    }

    private fun applyBars() {
        window.statusBarColor = T.p.surface
        window.navigationBarColor = T.p.raised
        val flags = WindowInsetsController.APPEARANCE_LIGHT_STATUS_BARS or WindowInsetsController.APPEARANCE_LIGHT_NAVIGATION_BARS
        window.insetsController?.setSystemBarsAppearance(if (T.isDark) 0 else flags, flags)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        handleIntent(intent)
    }

    private fun handleIntent(i: Intent?) {
        val d = i?.dataString ?: return
        if (d.startsWith("veil://")) {
            i.data = null
            importText(d, null)
        }
    }

    override fun onResume() {
        super.onResume()
        handler.post(poll)
        if (!autoDone) {
            autoDone = true
            if (Settings.autoApp(this) && !Veilcore.running() && Profiles.active(this) != null && VpnService.prepare(this) == null) {
                pending = true
                VeilVpnService.connect(this)
            }
        }
    }

    override fun onPause() {
        handler.removeCallbacks(poll)
        super.onPause()
    }

    // ---------- навигация ----------

    fun rootPage(i: Int): Page = when (i) {
        0 -> HomePage(this)
        1 -> TrafficPage(this)
        2 -> SettingsPage(this)
        else -> AddPage(this)
    }

    private fun buildNav(): LinearLayout {
        val bar = hbox().apply {
            setBackgroundColor(T.p.raised)
            setPadding(dp(8), 0, dp(8), 0)
        }
        val icons = intArrayOf(R.drawable.ic_home, R.drawable.ic_share, R.drawable.ic_sliders, R.drawable.ic_plus)
        val names = intArrayOf(R.string.tab_home, R.string.tab_traffic, R.string.tab_settings, R.string.tab_add)
        navIcons.clear()
        for (i in icons.indices) {
            val cell = FrameLayout(this)
            val ic = icon(icons[i], 26f, T.p.muted)
            ic.layoutParams = FrameLayout.LayoutParams(dp(26), dp(26), Gravity.CENTER)
            cell.addView(ic)
            val bar3 = View(this).apply { background = rounded(T.p.accent, 2f) }
            cell.addView(bar3, FrameLayout.LayoutParams(dp(28), dp(3), Gravity.TOP or Gravity.CENTER_HORIZONTAL))
            cell.contentDescription = getString(names[i])
            cell.setOnClickListener { selectTab(i) }
            bar.addView(cell, lp(0, dp(56), 1f))
            navIcons.add(ic to bar3)
        }
        return bar
    }

    private fun updateNav() {
        navIcons.forEachIndexed { i, (ic, bar) ->
            ic.setColorFilter(if (i == tab) T.p.accentInk else T.p.muted)
            bar.visibility = if (i == tab) View.VISIBLE else View.INVISIBLE
        }
    }

    fun selectTab(i: Int) {
        closeSheet()
        tab = i
        stack.clear()
        show(rootPage(i))
    }

    private fun show(p: Page) {
        stack.add(p)
        render()
    }

    fun push(p: Page) { closeSheet(); show(p) }

    fun pop() {
        if (stack.size > 1) { stack.removeAt(stack.size - 1); render() }
    }

    private fun render() {
        val p = stack.last()
        updateNav()
        content.removeAllViews()
        val body = p.build()
        val v: View = if (stack.size > 1 || p.title != null) {
            val col = vbox()
            // шапка по дизайну: высота 56 dp, заголовок 17 sp по центру, «назад» 44 dp слева с отступом 8 dp
            val head = FrameLayout(this)
            val maxTitle = (resources.displayMetrics.widthPixels / T.density).toInt() - 130
            head.addView(text(p.title ?: "", 17f, 600, lines = 1).also { it.maxWidth = dp(maxTitle); it.gravity = Gravity.CENTER },
                FrameLayout.LayoutParams(WRAP, WRAP, Gravity.CENTER))
            if (stack.size > 1) {
                val back = icon(R.drawable.ic_back, 24f)
                back.setPadding(dp(10), dp(10), dp(10), dp(10))
                back.setOnClickListener { pop() }
                head.addView(back, FrameLayout.LayoutParams(dp(44), dp(44), Gravity.START or Gravity.CENTER_VERTICAL).apply { leftMargin = dp(8) })
            }
            col.addView(head, lp(MATCH, dp(56)))
            val b = if (p.scroll) ScrollView(this).apply { addView(body); isFillViewport = true } else body
            col.addView(b, lp(MATCH, 0, 1f))
            col
        } else if (p.scroll) ScrollView(this).apply { addView(body); isFillViewport = true } else body
        content.addView(v, FrameLayout.LayoutParams(MATCH, MATCH))
        p.refresh()
    }

    fun rebuild() { render() }

    override fun onBackPressed() {
        when {
            overlay.visibility == View.VISIBLE -> closeSheet()
            stack.size > 1 -> pop()
            tab != 0 -> selectTab(0)
            else -> super.onBackPressed()
        }
    }

    // ---------- нижняя шторка ----------

    private var sheetGen = 0          // номер показа: отсекает конец анимации закрытия, если шторку уже открыли заново
    private var sheetPanel: View? = null
    private var sheetScrim: View? = null
    private var sheetClosing = false
    private val sheetEase = android.view.animation.PathInterpolator(0.2f, 0f, 0.2f, 1f)

    fun openSheet(title: String, body: View) {
        sheetGen++
        sheetClosing = false
        overlay.removeAllViews()
        overlay.visibility = View.VISIBLE
        val scrim = View(this).apply { setBackgroundColor(T.p.scrim); alpha = 0f; setOnClickListener { closeSheet() } }
        overlay.addView(scrim, FrameLayout.LayoutParams(MATCH, MATCH))
        val sheet = vbox().apply {
            background = android.graphics.drawable.GradientDrawable().apply {
                setColor(T.p.raised)
                val r = dp(20).toFloat()
                cornerRadii = floatArrayOf(r, r, r, r, 0f, 0f, 0f, 0f)
            }
            setPadding(dp(20), dp(12), dp(20), dp(20))
            isClickable = true
        }
        val handle = View(this).apply { background = rounded(T.p.high, 3f) }
        sheet.addView(handle, LinearLayout.LayoutParams(dp(44), dp(5)).apply { gravity = Gravity.CENTER_HORIZONTAL; bottomMargin = dp(14) })
        val head = hbox()
        head.addView(text(title, 20f, 600), lp(0, WRAP, 1f))
        val x = icon(R.drawable.ic_close, 22f, T.p.muted)
        x.setOnClickListener { closeSheet() }
        head.addView(x)
        sheet.addView(head, lp(MATCH, WRAP))
        sheet.addView(space(14f))
        sheet.addView(body, lp(MATCH, WRAP))
        // шторка следует за пальцем при смахивании вниз; отпустили далеко или быстро — уезжает, иначе возвращается
        val sc = SheetScroll(this,
            onDrag = { f -> scrim.alpha = 1f - f },
            onRelease = { close, velocity -> if (close) closeSheet(velocity) else {
                scrim.animate().alpha(1f).setDuration(160).start()
                sheetPanel?.animate()?.translationY(0f)?.setDuration(200)?.setInterpolator(sheetEase)?.start()
            } })
        sc.addView(sheet)
        val h = (resources.displayMetrics.heightPixels * 0.8f).toInt()
        overlay.addView(sc, FrameLayout.LayoutParams(MATCH, WRAP, Gravity.BOTTOM).also { it.height = WRAP })
        sc.layoutParams = FrameLayout.LayoutParams(MATCH, WRAP, Gravity.BOTTOM)
        sc.post { if (sc.height > h) sc.layoutParams = FrameLayout.LayoutParams(MATCH, h, Gravity.BOTTOM) }
        sheetPanel = sc; sheetScrim = scrim
        if (Settings.reduceMotion(this)) { scrim.alpha = 1f; return }
        scrim.animate().alpha(1f).setDuration(220).start()
        sc.translationY = dp(300).toFloat()
        sc.animate().translationY(0f).setDuration(260).setInterpolator(sheetEase).start()
    }

    /** Закрыть шторку с уездом вниз и затуханием затемнения (200 мс); velocity — скорость жеста, px/с (0 — нажатие). */
    fun closeSheet(velocity: Float = 0f) {
        if (overlay.visibility != View.VISIBLE || sheetClosing) return
        val panel = sheetPanel; val scrim = sheetScrim
        fun finish() { overlay.visibility = View.GONE; overlay.removeAllViews(); sheetPanel = null; sheetScrim = null; sheetClosing = false }
        if (panel == null || scrim == null || Settings.reduceMotion(this)) { finish(); return }
        sheetClosing = true
        val gen = ++sheetGen
        // при быстром жесте — быстрее: оставшийся путь / скорость, но не дольше 200 и не короче 90 мс
        val remaining = panel.height - panel.translationY
        val dur = if (velocity > 0) (remaining / velocity * 1000f).toLong().coerceIn(90L, 200L) else 200L
        scrim.animate().alpha(0f).setDuration(dur).start()
        panel.animate().translationY(panel.height.toFloat()).setDuration(dur)
            .setInterpolator(android.view.animation.AccelerateInterpolator(1.4f))
            .withEndAction { if (gen == sheetGen) finish() }.start()
    }

    fun toast(s: String) = Toast.makeText(this, s, Toast.LENGTH_SHORT).show()

    // ---------- подключение ----------

    fun running() = status.optString("state", "stopped") != "stopped"

    fun toggleConnect() {
        if (running() || pending) {
            pending = false
            disconnecting = true
            disconnectingSince = System.currentTimeMillis()
            VeilVpnService.disconnect(this)
            return
        }
        connect(null)
    }

    fun connect(profileId: String?) {
        if (Profiles.all(this).isEmpty()) { selectTab(3); return }
        Store.lastError = null
        val prep = VpnService.prepare(this)
        if (prep != null) {
            pendingProfile = profileId
            @Suppress("DEPRECATION")
            startActivityForResult(prep, REQ_VPN)
        } else {
            pending = true
            VeilVpnService.connect(this, profileId)
        }
    }

    private var pendingProfile: String? = null

    /** Смена активного профиля (из шторки); при активном VPN — переподключение к выбранному. */
    fun switchProfile(id: String) {
        Settings.setActive(this, id)
        closeSheet()
        if (running() || pending) {
            pending = true
            VeilVpnService.connect(this, id)
        }
        render()
    }

    fun errorText(code: String?): String = when {
        code == null -> ""
        code == "E_OPENVPN" -> getString(R.string.err_openvpn)
        code == "E_IPSEC" -> getString(R.string.err_ipsec)
        code == "E_RELAY" -> getString(R.string.err_relay)
        code == "E_FORMAT" -> getString(R.string.err_format)
        code == "E_EMPTY" -> getString(R.string.err_empty)
        code == "E_VPN" -> getString(R.string.err_vpn)
        else -> code
    }

    /** Импорт текста (ссылка, .conf, JSON): добавляет профиль и делает его активным, если это первый. */
    fun importText(text: String, name: String?): Boolean {
        return try {
            val p = Profiles.addRaw(this, text, name)
            toast(getString(R.string.added_profile, p.name))
            if (stack.last() is AddPage || stack.last() is ProfilesPage) selectTab(if (stack.last() is AddPage) 0 else tab) else render()
            true
        } catch (e: Exception) {
            toast(errorText(e.message ?: "E_FORMAT"))
            false
        }
    }

    // ---------- периодический опрос ----------

    private fun tick() {
        status = try { JSONObject(Veilcore.status()) } catch (_: Exception) { JSONObject().put("state", "stopped") }
        val state = status.optString("state")
        if (state != "stopped") pending = false
        if (disconnecting && (state == "stopped" || System.currentTimeMillis() - disconnectingSince > 4000)) disconnecting = false
        if (pending && state == "stopped" && Store.lastError != null) pending = false
        // скорость
        val rx = status.optLong("rx"); val tx = status.optLong("tx")
        if (state == "stopped") { lastRx = -1; lastTx = -1 } else {
            if (lastRx >= 0) {
                rxHist.addLast(maxOf(rx - lastRx, 0).toFloat()); txHist.addLast(maxOf(tx - lastTx, 0).toFloat())
                while (rxHist.size > 60) rxHist.removeFirst()
                while (txHist.size > 60) txHist.removeFirst()
            }
            lastRx = rx; lastTx = tx
        }
        stack.lastOrNull()?.refresh()
    }

    // ---------- результаты активностей ----------

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        @Suppress("DEPRECATION")
        super.onActivityResult(requestCode, resultCode, data)
        when (requestCode) {
            REQ_VPN -> if (resultCode == RESULT_OK) {
                pending = true
                VeilVpnService.connect(this, pendingProfile)
            } else toast(getString(R.string.err_vpn_denied))
            REQ_FILE -> if (resultCode == RESULT_OK) data?.data?.let { importFile(it) }
            REQ_EXPORT -> if (resultCode == RESULT_OK) data?.data?.let { uri ->
                try {
                    contentResolver.openOutputStream(uri)?.use { it.write(Profiles.exportJson(this).toByteArray()) }
                    toast(getString(R.string.backup_saved))
                } catch (_: Exception) { toast(getString(R.string.err_generic)) }
            }
            REQ_IMPORT -> if (resultCode == RESULT_OK) data?.data?.let { uri ->
                try {
                    val t = contentResolver.openInputStream(uri)?.use { it.readBytes().take(1 shl 20).toByteArray().toString(Charsets.UTF_8) } ?: ""
                    val n = Profiles.importBackup(this, t)
                    toast(getString(R.string.backup_restored, n))
                    render()
                } catch (_: Exception) { toast(getString(R.string.err_format)) }
            }
            else -> {
                val r = IntentIntegrator.parseActivityResult(requestCode, resultCode, data)
                val s = r?.contents
                if (s != null) importText(s, null)
            }
        }
    }

    private fun importFile(uri: Uri) {
        try {
            val bytes = contentResolver.openInputStream(uri)?.use { it.readBytes() } ?: return
            if (bytes.size > 256 * 1024) { toast(getString(R.string.err_format)); return }
            val text = bytes.toString(Charsets.UTF_8)
            if (text.contains("\"veil_backup\"")) {
                val n = Profiles.importBackup(this, text)
                toast(getString(R.string.backup_restored, n)); selectTab(0)
            } else importText(text, null)
        } catch (_: Exception) { toast(getString(R.string.err_generic)) }
    }

    fun pickFile() {
        val i = Intent(Intent.ACTION_OPEN_DOCUMENT).addCategory(Intent.CATEGORY_OPENABLE).setType("*/*")
        @Suppress("DEPRECATION")
        startActivityForResult(i, REQ_FILE)
    }

    fun scanQr() {
        IntentIntegrator(this).setDesiredBarcodeFormats(IntentIntegrator.QR_CODE)
            .setPrompt(getString(R.string.scan_prompt)).setBeepEnabled(false).setOrientationLocked(false).initiateScan()
    }

    fun exportBackup() {
        val i = Intent(Intent.ACTION_CREATE_DOCUMENT).addCategory(Intent.CATEGORY_OPENABLE).setType("application/json").putExtra(Intent.EXTRA_TITLE, "veil-backup.json")
        @Suppress("DEPRECATION")
        startActivityForResult(i, REQ_EXPORT)
    }

    fun importBackupFile() {
        val i = Intent(Intent.ACTION_OPEN_DOCUMENT).addCategory(Intent.CATEGORY_OPENABLE).setType("*/*")
        @Suppress("DEPRECATION")
        startActivityForResult(i, REQ_IMPORT)
    }

    fun clipboardText(): String {
        val cm = getSystemService(CLIPBOARD_SERVICE) as ClipboardManager
        return cm.primaryClip?.takeIf { it.itemCount > 0 }?.getItemAt(0)?.coerceToText(this)?.toString()?.trim() ?: ""
    }

    fun applyAppearance() { recreate() }

    companion object {
        const val REQ_VPN = 1
        const val REQ_FILE = 11
        const val REQ_EXPORT = 12
        const val REQ_IMPORT = 13
    }
}

/**
 * ScrollView шторки: когда содержимое прокручено в самый верх и палец идёт вниз, шторка тянется за пальцем.
 * Отпустили (сдвиг > 80 dp или быстрый жест вниз) — onRelease(true, скорость), иначе onRelease(false, 0).
 */
private class SheetScroll(
    ctx: Context,
    val onDrag: (Float) -> Unit,
    val onRelease: (Boolean, Float) -> Unit,
) : ScrollView(ctx) {
    private val slop = android.view.ViewConfiguration.get(ctx).scaledTouchSlop
    private var downY = 0f
    private var dragging = false
    private var tracker: android.view.VelocityTracker? = null

    override fun onInterceptTouchEvent(e: android.view.MotionEvent): Boolean {
        when (e.actionMasked) {
            android.view.MotionEvent.ACTION_DOWN -> { downY = e.rawY; dragging = false }
            android.view.MotionEvent.ACTION_MOVE -> if (!dragging && scrollY == 0 && e.rawY - downY > slop) {
                dragging = true
                tracker = android.view.VelocityTracker.obtain()
                parent?.requestDisallowInterceptTouchEvent(true)
                return true
            }
        }
        return super.onInterceptTouchEvent(e)
    }

    override fun onTouchEvent(e: android.view.MotionEvent): Boolean {
        if (!dragging) return super.onTouchEvent(e)
        tracker?.addMovement(e)
        when (e.actionMasked) {
            android.view.MotionEvent.ACTION_MOVE -> {
                val dy = (e.rawY - downY - slop).coerceAtLeast(0f)
                translationY = dy
                onDrag(if (height > 0) (dy / height).coerceIn(0f, 1f) else 0f)
            }
            android.view.MotionEvent.ACTION_UP, android.view.MotionEvent.ACTION_CANCEL -> {
                tracker?.computeCurrentVelocity(1000)
                val v = tracker?.yVelocity ?: 0f
                tracker?.recycle(); tracker = null
                dragging = false
                val far = translationY > dp(80)
                val fast = v > dp(600) && translationY > dp(8)
                if (e.actionMasked == android.view.MotionEvent.ACTION_UP && (far || fast)) onRelease(true, if (fast) v else 0f)
                else onRelease(false, 0f)
            }
        }
        return true
    }
}
