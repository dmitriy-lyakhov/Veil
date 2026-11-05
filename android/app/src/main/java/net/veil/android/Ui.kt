package net.veil.android

import android.content.Context
import android.content.res.Configuration
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.graphics.drawable.RippleDrawable
import android.content.res.ColorStateList
import android.text.InputType
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.view.ViewGroup
import android.widget.EditText
import android.widget.FrameLayout
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.TextView
import java.util.Locale

/** Палитра по токенам дизайна (design/project/ds/veil/tokens.json). */
class Pal(
    val surface: Int, val raised: Int, val high: Int, val ink: Int, val muted: Int,
    val accent: Int, val onAccent: Int, val accentInk: Int, val success: Int, val scrim: Int,
)

object T {
    val dark = Pal(
        0xFF0B0B0C.toInt(), 0xFF1B1C1F.toInt(), 0xFF2A2B30.toInt(), 0xFFFFFFFF.toInt(), 0xFFA8AAB2.toInt(),
        0xFFFF393D.toInt(), 0xFF0B0B0C.toInt(), 0xFFFF393D.toInt(), 0xFF7DE0A0.toInt(), 0xC70B0B0C.toInt(),
    )
    val light = Pal(
        0xFFFFFFFF.toInt(), 0xFFF2F3F5.toInt(), 0xFFE4E6EA.toInt(), 0xFF0B0B0C.toInt(), 0xFF5B6068.toInt(),
        0xFFFF393D.toInt(), 0xFF0B0B0C.toInt(), 0xFFDB1E32.toInt(), 0xFF177A44.toInt(), 0x8C0B0B0C.toInt(),
    )
    var p: Pal = dark
    var isDark = true
    var density = 1f
    lateinit var medium: Typeface
    lateinit var semibold: Typeface
    lateinit var bold: Typeface

    fun init(ctx: Context) {
        density = ctx.resources.displayMetrics.density
        medium = ctx.resources.getFont(R.font.montserrat_medium)
        semibold = ctx.resources.getFont(R.font.montserrat_semibold)
        bold = ctx.resources.getFont(R.font.montserrat_bold)
        val mode = Settings.theme(ctx)
        val night = (ctx.resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES
        isDark = when (mode) { "dark" -> true; "light" -> false; else -> night }
        p = if (isDark) dark else light
    }
}

fun dp(v: Float): Int = (v * T.density + 0.5f).toInt()
fun dp(v: Int): Int = dp(v.toFloat())

fun rounded(color: Int, r: Float, stroke: Int = 0, strokeW: Float = 0f): GradientDrawable =
    GradientDrawable().apply {
        setColor(color)
        cornerRadius = dp(r).toFloat()
        if (stroke != 0) setStroke(dp(strokeW), stroke)
    }

fun View.ripple(bg: GradientDrawable) {
    background = RippleDrawable(ColorStateList.valueOf((T.p.ink and 0x00FFFFFF) or 0x22000000), bg, null)
}

fun Context.text(
    s: CharSequence, size: Float = 16f, weight: Int = 600, color: Int = T.p.ink, lines: Int = 0,
): TextView = TextView(this).apply {
    text = s
    setTextSize(TypedValue.COMPLEX_UNIT_SP, size)
    typeface = when (weight) { 700 -> T.bold; 600 -> T.semibold; else -> T.medium }
    setTextColor(color)
    includeFontPadding = false
    if (lines > 0) { maxLines = lines; ellipsize = android.text.TextUtils.TruncateAt.END }
    setLineSpacing(0f, 1.15f)
}

fun Context.icon(res: Int, size: Float = 24f, color: Int = T.p.ink): ImageView = ImageView(this).apply {
    setImageResource(res)
    setColorFilter(color)
    layoutParams = LinearLayout.LayoutParams(dp(size), dp(size))
}

fun lp(w: Int, h: Int, weight: Float = 0f) = LinearLayout.LayoutParams(w, h, weight)
const val MATCH = ViewGroup.LayoutParams.MATCH_PARENT
const val WRAP = ViewGroup.LayoutParams.WRAP_CONTENT

fun Context.vbox(): LinearLayout = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
fun Context.hbox(): LinearLayout = LinearLayout(this).apply { orientation = LinearLayout.HORIZONTAL; gravity = Gravity.CENTER_VERTICAL }

fun Context.space(h: Float): View = View(this).apply { layoutParams = LinearLayout.LayoutParams(1, dp(h)) }

/** Строка списка: карточка 14 dp (surface-raised): иконка, заголовок, подпись, правый элемент. */
fun Context.row(
    title: CharSequence, sub: CharSequence? = null, icon: Int? = null, trailing: View? = null,
    chevron: Boolean = false, bg: Int = T.p.raised, onClick: (() -> Unit)? = null,
): LinearLayout = hbox().apply {
    // размеры из дизайна (App.dc.html): строка 56 dp, с подписью 64, с плиткой иконки 76; поля 16, вертикаль 10 (12 у плитки)
    val tile = icon != null && bg == T.p.raised   // плитка иконки только у карточек страниц; в шторках иконка без плитки
    setPadding(dp(16), dp(if (tile) 12 else 10), dp(16), dp(if (tile) 12 else 10))
    minimumHeight = dp(if (tile) 76 else if (sub != null) 64 else 56)
    ripple(rounded(bg, 14f))
    if (tile) {
        val t = FrameLayout(this@row).apply { background = rounded(T.p.high, 11f) }
        t.addView(icon(icon!!, 22f, T.p.accentInk).also { it.layoutParams = FrameLayout.LayoutParams(dp(22), dp(22), Gravity.CENTER) })
        addView(t, LinearLayout.LayoutParams(dp(44), dp(44)).apply { rightMargin = dp(16) })
    } else if (icon != null) {
        addView(icon(icon, 22f).also { (it.layoutParams as LinearLayout.LayoutParams).rightMargin = dp(14) })
    }
    val col = vbox()
    col.addView(text(title, 16f, 600, lines = 2))
    if (sub != null) col.addView(text(sub, 13f, 500, T.p.muted, lines = 2).also { it.setPadding(0, dp(4), 0, 0) })
    addView(col, lp(0, WRAP, 1f))
    if (trailing != null) addView(trailing)
    if (chevron) addView(icon(R.drawable.ic_right, 20f, T.p.muted))
    if (onClick != null) setOnClickListener { onClick() } else isClickable = false
}

fun Context.input(hint: String, value: String = "", multiline: Boolean = false): EditText = EditText(this).apply {
    setText(value)
    this.hint = hint
    setTextSize(TypedValue.COMPLEX_UNIT_SP, 14f)
    typeface = T.medium
    setTextColor(T.p.ink)
    setHintTextColor(T.p.muted)
    background = rounded(T.p.high, 12f)
    setPadding(dp(14), dp(12), dp(14), dp(12))
    inputType = if (multiline) InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_FLAG_MULTI_LINE or InputType.TYPE_TEXT_FLAG_NO_SUGGESTIONS
    else InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_FLAG_NO_SUGGESTIONS
    if (multiline) { minLines = 3; maxLines = 6; gravity = Gravity.TOP or Gravity.START }
}

/** Кнопка: primary — заливка accent, текст on-accent; иначе surface-high. */
fun Context.button(label: CharSequence, primary: Boolean = true, onClick: () -> Unit): TextView =
    text(label, 16f, 600, if (primary) T.p.onAccent else T.p.ink).apply {
        gravity = Gravity.CENTER
        minHeight = dp(52)   // токен control: 52 dp
        setPadding(dp(16), dp(8), dp(16), dp(8))
        ripple(rounded(if (primary) T.p.accent else T.p.high, 14f))
        setOnClickListener { onClick() }
    }

class Toggle(ctx: Context, var on: Boolean, val onChange: (Boolean) -> Unit) : View(ctx) {
    private val paint = Paint(Paint.ANTI_ALIAS_FLAG)
    init {
        layoutParams = LinearLayout.LayoutParams(dp(48), dp(28))
        setOnClickListener { on = !on; invalidate(); onChange(on) }
        isClickable = true
    }
    private val check = android.graphics.Path()
    private val checkPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        style = Paint.Style.STROKE; strokeCap = Paint.Cap.SQUARE; strokeJoin = Paint.Join.MITER
    }

    /** Дизайн: дорожка 48x28 (радиус 14), бегунок 22 dp с отступом 3 dp; во включённом состоянии в бегунке галочка. */
    override fun onDraw(c: Canvas) {
        val w = width.toFloat(); val h = height.toFloat()
        paint.color = if (on) T.p.accent else T.p.high
        c.drawRoundRect(0f, 0f, w, h, h / 2, h / 2, paint)
        paint.color = if (on) T.p.onAccent else T.p.ink
        val r = dp(11).toFloat()
        val cx = if (on) w - dp(3) - r else dp(3) + r
        c.drawCircle(cx, h / 2, r, paint)
        if (on) {
            // галочка 14 dp (viewBox 24, штрих 3,2): M4 12 l5 5 l11-11
            val s = dp(14) / 24f; val ox = cx - dp(7); val oy = h / 2 - dp(7)
            check.reset()
            check.moveTo(ox + 4 * s, oy + 12 * s); check.lineTo(ox + 9 * s, oy + 17 * s); check.lineTo(ox + 20 * s, oy + 6 * s)
            checkPaint.color = T.p.accent; checkPaint.strokeWidth = 3.2f * s
            c.drawPath(check, checkPaint)
        }
    }
}

/** Радио-кнопка из дизайна: круг 22 dp с рамкой 2 dp (accent-ink у выбранной), внутри точка 10 dp. */
fun Context.radio(selected: Boolean): View {
    val box = FrameLayout(this).apply { layoutParams = LinearLayout.LayoutParams(dp(24), dp(24)) }
    val ring = View(this).apply {
        background = GradientDrawable().apply {
            shape = GradientDrawable.OVAL; setColor(Color.TRANSPARENT)
            setStroke(dp(2), if (selected) T.p.accentInk else T.p.muted)
        }
    }
    box.addView(ring, FrameLayout.LayoutParams(dp(22), dp(22), Gravity.CENTER))
    if (selected) {
        val dot = View(this).apply { background = GradientDrawable().apply { shape = GradientDrawable.OVAL; setColor(T.p.accent) } }
        box.addView(dot, FrameLayout.LayoutParams(dp(10), dp(10), Gravity.CENTER))
    }
    return box
}

/** Сегментный переключатель по дизайну: высота 52, радиус 16, поля 4; выбранный сегмент на surface-high (радиус 12), текст 15/600. */
fun Context.segmented(options: List<String>, selected: Int, onSelect: (Int) -> Unit): LinearLayout = hbox().apply {
    background = rounded(T.p.raised, 16f)
    setPadding(dp(4), dp(4), dp(4), dp(4))
    minimumHeight = dp(52)
    options.forEachIndexed { i, o ->
        val t = text(o, 15f, 600, T.p.ink)
        t.gravity = Gravity.CENTER
        if (i == selected) t.background = rounded(T.p.high, 12f)
        t.setOnClickListener { onSelect(i) }
        addView(t, lp(0, MATCH, 1f))
    }
}

fun Context.hairline(): View = View(this).apply { setBackgroundColor(T.p.high); layoutParams = LinearLayout.LayoutParams(MATCH, dp(1)) }

fun withLocale(ctx: Context): Context {
    val lang = Settings.lang(ctx)
    if (lang == "system") return ctx
    val loc = Locale(lang)
    Locale.setDefault(loc)
    val cfg = Configuration(ctx.resources.configuration)
    cfg.setLocale(loc)
    return ctx.createConfigurationContext(cfg)
}

private fun isRu() = Locale.getDefault().language == "ru"

fun fmtBytes(n: Long): String {
    val u = if (isRu()) arrayOf("Б", "КБ", "МБ", "ГБ", "ТБ") else arrayOf("B", "KB", "MB", "GB", "TB")
    var v = n.toDouble(); var i = 0
    while (v >= 1024 && i < u.size - 1) { v /= 1024; i++ }
    return if (i == 0) "$n ${u[0]}" else String.format(Locale.US, "%.1f %s", v, u[i])
}

fun fmtRate(n: Long): String = fmtBytes(n) + if (isRu()) "/с" else "/s"

fun frame(ctx: Context) = FrameLayout(ctx)
val transparent = Color.TRANSPARENT
