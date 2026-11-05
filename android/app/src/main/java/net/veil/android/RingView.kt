package net.veil.android

import android.content.Context
import android.graphics.Canvas
import android.graphics.Paint
import android.graphics.RectF
import android.os.SystemClock
import android.view.View
import kotlin.math.PI
import kotlin.math.exp
import kotlin.math.sin

enum class RingState { IDLE, CONNECTING, CONNECTED, ERROR, DISCONNECTING }

/**
 * Кольцо подключения (дизайн: 196 dp). Анимации из MotionLab: дыхание (Idle), бегущая дуга (Connecting/Disconnecting),
 * волны (Connected), «хлопок» при установке соединения, встряска при ошибке.
 */
class RingView(ctx: Context) : View(ctx) {
    var label: String = ""
        set(v) { field = v; invalidate() }
    private var state = RingState.IDLE
    private var stateSince = SystemClock.uptimeMillis()
    private var popAt = 0L
    private var shakeAt = 0L
    private val paint = Paint(Paint.ANTI_ALIAS_FLAG).apply { style = Paint.Style.STROKE; strokeCap = Paint.Cap.BUTT }
    private val fill = Paint(Paint.ANTI_ALIAS_FLAG)
    private val tp = Paint(Paint.ANTI_ALIAS_FLAG).apply { textAlign = Paint.Align.CENTER }
    private val rect = RectF()
    private var attached = false

    /** «Уменьшить движение» (настройка приложения или системное «убрать анимацию»): кольцо рисуется статично. */
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
        val landscape = resources.configuration.orientation == android.content.res.Configuration.ORIENTATION_LANDSCAPE
        val s = if (landscape) dp(140) else dp(280)
        setMeasuredDimension(s, s)
    }

    override fun onAttachedToWindow() { super.onAttachedToWindow(); attached = true; postInvalidateOnAnimation() }
    override fun onDetachedFromWindow() { attached = false; super.onDetachedFromWindow() }

    override fun onDraw(c: Canvas) {
        val still = motionOff()
        val now = if (still) stateSince else SystemClock.uptimeMillis()
        val t = (now - stateSince) / 1000f
        val cx = width / 2f; val cy = height / 2f
        var r = dp(98).toFloat() // диаметр 196 dp
        val p = T.p

        c.save()
        // рисуем в координатах дизайна (280 dp), на меньшем кольце масштабируем
        val k = width / dp(280).toFloat()
        c.scale(k, k, cx, cy)
        // встряска при ошибке: затухающее колебание 0.6 с
        if (!still && state == RingState.ERROR && now - shakeAt < 700) {
            val st = (now - shakeAt) / 1000f
            c.translate((sin(st * 2 * PI * 7) * exp(-st * 5) * dp(10)).toFloat(), 0f)
        }
        // «хлопок»: перелёт масштаба с затуханием
        if (!still && now - popAt < 600) {
            val st = (now - popAt) / 1000f
            val s = 1f + (0.10f * exp(-st * 7) * sin(st * 2 * PI * 3.2 + PI / 2)).toFloat()
            c.scale(s, s, cx, cy)
        }

        when (state) {
            RingState.IDLE -> {
                r *= (1f + 0.015f * sin(t * 2 * PI / 4f).toFloat())
                ring(c, cx, cy, r, p.ink, 4f, 0.9f)
            }
            RingState.CONNECTING, RingState.DISCONNECTING -> {
                ring(c, cx, cy, r, p.high, 4f, 1f)
                val speed = if (state == RingState.CONNECTING) 300f else -360f
                val start = (t * speed) % 360f
                paint.color = if (state == RingState.CONNECTING) p.accentInk else p.muted
                paint.strokeWidth = dp(5).toFloat()
                rect.set(cx - r, cy - r, cx + r, cy + r)
                c.drawArc(rect, start - 90f, 100f, false, paint)
            }
            RingState.CONNECTED -> {
                // волны: два расширяющихся кольца с затуханием (период 3 с)
                for (k in 0..1) {
                    val ph = ((t / 3f) + k * 0.5f) % 1f
                    val rr = r + dp(8) + ph * dp(36)
                    paint.color = p.accent
                    paint.alpha = ((1f - ph) * 70).toInt()
                    paint.strokeWidth = dp(2).toFloat()
                    c.drawCircle(cx, cy, rr, paint)
                }
                paint.alpha = 255
                ring(c, cx, cy, r, p.accent, 6f, 1f)
            }
            RingState.ERROR -> ring(c, cx, cy, r, p.accentInk, 4f, 1f)
        }
        c.restore()

        tp.typeface = T.semibold
        tp.textSize = dp(18).toFloat() * resources.configuration.fontScale * (if (k < 1f) 0.8f else 1f)
        tp.color = when (state) {
            RingState.IDLE -> p.ink
            RingState.CONNECTED -> p.ink
            RingState.ERROR -> p.accentInk
            else -> p.muted
        }
        c.drawText(label, cx, cy + tp.textSize / 3f, tp)
        if (attached && !still) postInvalidateOnAnimation()
    }

    private fun ring(c: Canvas, cx: Float, cy: Float, r: Float, color: Int, w: Float, alpha: Float) {
        paint.color = color
        paint.alpha = (alpha * 255).toInt()
        paint.strokeWidth = dp(w).toFloat()
        c.drawCircle(cx, cy, r, paint)
        paint.alpha = 255
    }
}
