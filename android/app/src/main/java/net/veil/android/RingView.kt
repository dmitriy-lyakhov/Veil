package net.veil.android

import android.animation.ArgbEvaluator
import android.content.Context
import android.graphics.Canvas
import android.graphics.DashPathEffect
import android.graphics.Paint
import android.graphics.Path
import android.graphics.RectF
import android.graphics.Typeface
import android.os.SystemClock
import android.view.View
import android.view.animation.PathInterpolator
import kotlin.math.PI
import kotlin.math.floor
import kotlin.math.max
import kotlin.math.min
import kotlin.math.sin

enum class RingState { IDLE, CONNECTING, CONNECTED, ERROR, DISCONNECTING }

/**
 * Кольцо подключения по дизайну (App.dc.html, MotionLab): размер 196 dp (150 при малой высоте), рамка 4 dp.
 *  - покой: рамка ink, «дыхание» 1→1,03 за 4 с;
 *  - подключение: рамка dim, дуга-спиннер 1,1 с, три волны (период 1,8 с);
 *  - подключено: залитый accent, щит, подпись и таймер цветом on-accent, две волны (4,2 с), «хлопок» 0,5 с;
 *  - отключение: переход заливки accent → прозрачная рамка ink за 0,7 с;
 *  - ошибка: пунктирная рамка accent-ink, встряска 0,5 с.
 * «Уменьшить движение» (настройка или системное «убрать анимацию»): кадр статичный, без волн и без бесконечной перерисовки.
 * Рисование идёт за пределы view (волны до 1,65 размера): родители должны иметь clipChildren = false.
 */
class RingView(ctx: Context) : View(ctx) {
    var label: String = ""
        set(v) { if (field != v) { field = v; invalidate() } }
    var timer: String = ""
        set(v) { if (field != v) { field = v; invalidate() } }
    /** Диаметр кольца в dp (196 обычный, 150 при малой высоте экрана). */
    var sizeDp: Int = 196
        set(v) { if (field != v) { field = v; requestLayout(); invalidate() } }

    private var state = RingState.IDLE
    private var stateSince = SystemClock.uptimeMillis()
    private var popAt = 0L
    private var shakeAt = 0L
    private val stroke = Paint(Paint.ANTI_ALIAS_FLAG).apply { style = Paint.Style.STROKE }
    private val fill = Paint(Paint.ANTI_ALIAS_FLAG).apply { style = Paint.Style.FILL }
    private val tp = Paint(Paint.ANTI_ALIAS_FLAG).apply { textAlign = Paint.Align.CENTER }
    private val rect = RectF()
    private val shield = Path()
    private val check = Path()
    private val ease = PathInterpolator(0.2f, 0f, 0.2f, 1f)
    private val argb = ArgbEvaluator()
    private var attached = false

    /** «Уменьшить движение» — настройка приложения или системное «убрать анимацию». */
    private fun motionOff() = Settings.reduceMotion(context)

    fun setState(s: RingState) {
        if (s == state) return
        val prev = state
        state = s
        stateSince = SystemClock.uptimeMillis()
        if (s == RingState.CONNECTED && prev == RingState.CONNECTING) popAt = stateSince
        if (s == RingState.ERROR) shakeAt = stateSince
        invalidate()
    }

    override fun onMeasure(w: Int, h: Int) {
        val s = dp(sizeDp)
        setMeasuredDimension(s, s)
    }

    override fun onAttachedToWindow() { super.onAttachedToWindow(); attached = true; postInvalidateOnAnimation() }
    override fun onDetachedFromWindow() { attached = false; super.onDetachedFromWindow() }

    /** Волны: n колец с рамкой 2 dp, масштаб 1→1,65, прозрачность 0,6→0, период d секунд, сдвиг d/n·i. */
    private fun ripples(c: Canvas, cx: Float, cy: Float, r: Float, t: Float, d: Float, n: Int) {
        for (i in 0 until n) {
            val local = t - d / n * i
            if (local < 0f) continue
            val ph = ease.getInterpolation((local / d) % 1f)
            stroke.color = T.p.accent
            stroke.alpha = ((0.6f * (1f - ph)) * 255).toInt()
            stroke.strokeWidth = dp(2).toFloat()
            stroke.pathEffect = null
            c.drawCircle(cx, cy, r * (1f + 0.65f * ph) - dp(1), stroke)
        }
        stroke.alpha = 255
    }

    override fun onDraw(c: Canvas) {
        val still = motionOff()
        val now = if (still) stateSince else SystemClock.uptimeMillis()
        val t = (now - stateSince) / 1000f
        val p = T.p
        val size = width.toFloat()
        val cx = width / 2f; val cy = height / 2f
        val r = size / 2f
        val bw = dp(4).toFloat()

        c.save()
        // встряска при ошибке: 0 → −8 → 8 → −5 → 4 → 0 (20 % шага), 0,5 с
        if (!still && state == RingState.ERROR && now - shakeAt < 500) {
            val f = (now - shakeAt) / 500f
            val keys = floatArrayOf(0f, -8f, 8f, -5f, 4f, 0f)
            val seg = min((f * 5).toInt(), 4); val u = f * 5 - seg
            c.translate(dp((keys[seg] + (keys[seg + 1] - keys[seg]) * u).toInt()).toFloat(), 0f)
        }
        // «хлопок» при подключении: 0,92 → 1,05 (60 %) → 1 за 0,5 с
        var scale = 1f
        if (!still && now - popAt < 500) {
            val f = (now - popAt) / 500f
            scale = if (f < 0.6f) 0.92f + 0.13f * ease.getInterpolation(f / 0.6f) else 1.05f - 0.05f * ease.getInterpolation((f - 0.6f) / 0.4f)
        }
        // «дыхание» в покое: 1 → 1,03 → 1 за 4 с
        if (!still && state == RingState.IDLE) scale = 1f + 0.03f * (0.5f - 0.5f * kotlin.math.cos(t * 2 * PI.toFloat() / 4f))
        if (scale != 1f) c.scale(scale, scale, cx, cy)

        var ink = p.ink            // цвет подписи
        when (state) {
            RingState.IDLE -> ring(c, cx, cy, r, bw, p.ink)
            RingState.CONNECTING -> {
                if (!still) ripples(c, cx, cy, r, t, 1.8f, 3)
                ring(c, cx, cy, r, bw, p.high)
                // спиннер: окружность радиусом 48,2 % от (размер + 8 dp), штрих 2,4 %, дуга 90 из 302,9 (≈107°)
                val big = size + dp(8)
                stroke.color = p.accent; stroke.strokeWidth = 0.024f * big; stroke.strokeCap = Paint.Cap.ROUND; stroke.pathEffect = null
                val rr = 0.482f * big
                rect.set(cx - rr, cy - rr, cx + rr, cy + rr)
                val start = if (still) -90f else (t / 1.1f * 360f) % 360f - 90f
                c.drawArc(rect, start, 90f / 302.9f * 360f, false, stroke)
                stroke.strokeCap = Paint.Cap.BUTT
            }
            RingState.CONNECTED -> {
                if (!still) ripples(c, cx, cy, r, t, 4.2f, 2)
                fill.color = p.accent
                c.drawCircle(cx, cy, r, fill)
                ink = p.onAccent
            }
            RingState.DISCONNECTING -> {
                // заливка accent → прозрачная, рамка accent → ink, текст on-accent → ink за 0,7 с (ease-in)
                val f = if (still) 1f else min(t / 0.7f, 1f).let { it * it }
                fill.color = argb.evaluate(f, p.accent, p.accent and 0x00FFFFFF) as Int
                c.drawCircle(cx, cy, r, fill)
                ring(c, cx, cy, r, bw, argb.evaluate(f, p.accent, p.ink) as Int)
                ink = argb.evaluate(f, p.onAccent, p.ink) as Int
            }
            RingState.ERROR -> {
                stroke.color = p.accentInk; stroke.strokeWidth = bw
                val circ = (2 * PI * (r - bw / 2)).toFloat()
                val dash = dp(10).toFloat(); val n = floor(circ / (dash * 1.8f)).coerceAtLeast(1f)
                val gap = circ / n - dash
                stroke.pathEffect = DashPathEffect(floatArrayOf(dash, gap), 0f)
                c.drawCircle(cx, cy, r - bw / 2, stroke)
                stroke.pathEffect = null
            }
        }

        // содержимое: щит (подключено / отключение), подпись, таймер (подключено); столбец по центру, зазор 6 dp
        val showShield = state == RingState.CONNECTED || state == RingState.DISCONNECTING
        val showTimer = state == RingState.CONNECTED && timer.isNotEmpty()
        val shieldS = dp(if (sizeDp < 160) 28 else 38).toFloat()
        val fs = max(11, min(22, floor(sizeDp * 0.62f / (max(label.length, 1) * 0.64f)).toInt()))
        tp.typeface = T.semibold; tp.textSize = dp(fs).toFloat(); tp.color = ink
        val labelH = tp.textSize * 1.1f
        val timerP = Paint(tp).apply { typeface = Typeface.create("sans-serif", Typeface.BOLD); textSize = dp(15).toFloat() }
        val timerH = timerP.textSize * 1.2f
        val gap = dp(6).toFloat()
        var total = labelH
        if (showShield) total += shieldS + gap
        if (showTimer) total += timerH + gap
        var y = cy - total / 2
        if (showShield) { drawShield(c, cx, y, shieldS, ink); y += shieldS + gap }
        c.drawText(label, cx, y + labelH * 0.78f, tp); y += labelH
        if (showTimer) { y += gap; timerP.color = ink; c.drawText(timer, cx, y + timerH * 0.78f, timerP) }
        c.restore()

        if (attached && !still) postInvalidateOnAnimation()
    }

    private fun ring(c: Canvas, cx: Float, cy: Float, r: Float, w: Float, color: Int) {
        stroke.color = color; stroke.alpha = 255; stroke.strokeWidth = w; stroke.pathEffect = null
        c.drawCircle(cx, cy, r - w / 2, stroke)
    }

    /** Щит с галочкой из набора иконок дизайна (viewBox 24, штрих 2, квадратные окончания). */
    private fun drawShield(c: Canvas, cx: Float, top: Float, size: Float, color: Int) {
        val s = size / 24f; val ox = cx - size / 2; val oy = top
        shield.reset()
        shield.moveTo(ox + 12 * s, oy + 2 * s); shield.lineTo(ox + 20 * s, oy + 5 * s); shield.lineTo(ox + 20 * s, oy + 12 * s)
        shield.lineTo(ox + 12 * s, oy + 22 * s); shield.lineTo(ox + 4 * s, oy + 12 * s); shield.lineTo(ox + 4 * s, oy + 5 * s); shield.close()
        check.reset()
        check.moveTo(ox + 8 * s, oy + 12 * s); check.lineTo(ox + 11 * s, oy + 15 * s); check.lineTo(ox + 16 * s, oy + 9 * s)
        stroke.color = color; stroke.alpha = 255; stroke.strokeWidth = 2 * s; stroke.strokeCap = Paint.Cap.SQUARE
        stroke.strokeJoin = Paint.Join.MITER; stroke.pathEffect = null
        c.drawPath(shield, stroke); c.drawPath(check, stroke)
        stroke.strokeCap = Paint.Cap.BUTT
    }
}
