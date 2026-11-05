package net.veil.android

import android.animation.Animator
import android.animation.AnimatorListenerAdapter
import android.animation.ValueAnimator
import android.content.Context
import android.view.MotionEvent
import android.view.VelocityTracker
import android.view.View
import android.view.ViewConfiguration
import android.view.ViewGroup
import android.view.animation.PathInterpolator
import android.widget.LinearLayout
import android.widget.ScrollView
import kotlin.math.abs

/**
 * Выдвижная панель сервера на главной (как в AmneziaVPN): вверху ручка, имя сервера и подпись (видны всегда), ниже — список серверов.
 * Панель имеет высоту всей области и сдвигается вниз (translationY), так что в свёрнутом виде видна только верхняя часть;
 * родитель обрезает остальное. Тянется за пальцем вверх и вниз, отпущенная — доезжает до ближайшего положения (учитывается скорость).
 * Без затемнения: главный экран просто перекрывается. t: 0 — свёрнуто, 1 — развёрнуто.
 */
class ServerDrawer(ctx: Context, val top: View, list: View) : LinearLayout(ctx) {
    var t = 0f
        private set
    var onProgress: ((Float) -> Unit)? = null

    private val scroll = ScrollView(ctx).apply { isVerticalScrollBarEnabled = false; overScrollMode = OVER_SCROLL_NEVER }
    private val slop = ViewConfiguration.get(ctx).scaledTouchSlop
    private val settleEase = PathInterpolator(0.22f, 0.9f, 0.3f, 1f)   // мягкое торможение без рывка в начале
    private val toggleEase = PathInterpolator(0.4f, 0f, 0.2f, 1f)      // стандартная кривая для нажатия
    private var anim: ValueAnimator? = null
    private var downY = 0f
    private var startT = 0f
    private var dragging = false
    private var tracker: VelocityTracker? = null

    init {
        orientation = VERTICAL
        addView(top, LayoutParams(MATCH, WRAP))
        scroll.addView(list, ViewGroup.LayoutParams(MATCH, WRAP))
        addView(scroll, LayoutParams(MATCH, 0, 1f))
    }

    /** Высота видимой части в свёрнутом состоянии (до низа верхнего блока). */
    val collapsedHeight get() = top.bottom

    /** Путь, который проходит панель между свёрнутым и развёрнутым положением, px. */
    private val range get() = (height - top.bottom).coerceAtLeast(1)

    private fun apply() {
        translationY = (1f - t) * range
        onProgress?.invoke(t)
    }

    override fun onLayout(changed: Boolean, l: Int, tp: Int, r: Int, b: Int) {
        super.onLayout(changed, l, tp, r, b)
        apply()
    }

    fun setProgress(v: Float) { t = v.coerceIn(0f, 1f); apply() }

    /** Плавно доехать до положения target (0 или 1); при «Уменьшить движение» — сразу. */
    fun animateTo(target: Float, velocity: Float = 0f, settle: Boolean = false, end: (() -> Unit)? = null) {
        anim?.cancel()
        if (Settings.reduceMotion(context) || height == 0) { setProgress(target); end?.invoke(); return }
        val dist = abs(target - t)
        // доводка после жеста: продолжает движение пальца и мягко тормозит (360–640 мс; быстрый бросок сокращает не более чем на треть);
        // переключение нажатием — стандартная кривая, 260–420 мс
        val ms = if (settle) (360f + 280f * dist).let { base -> (base * (dp(900) / maxOf(velocity, dp(900).toFloat())).coerceAtLeast(0.67f)).toLong() }
                 else (420f * dist).toLong().coerceAtLeast(260L)
        anim = ValueAnimator.ofFloat(t, target).apply {
            duration = ms; interpolator = if (settle) settleEase else toggleEase
            addUpdateListener { setProgress(it.animatedValue as Float) }
            addListener(object : AnimatorListenerAdapter() {
                private var cancelled = false
                override fun onAnimationCancel(a: Animator) { cancelled = true }
                override fun onAnimationEnd(a: Animator) { if (!cancelled) end?.invoke() }
            })
            start()
        }
    }

    fun toggle() = animateTo(if (t > 0.5f) 0f else 1f)

    override fun onInterceptTouchEvent(e: MotionEvent): Boolean {
        when (e.actionMasked) {
            MotionEvent.ACTION_DOWN -> { downY = e.rawY; startT = t; dragging = false; anim?.cancel() }
            MotionEvent.ACTION_MOVE -> if (!dragging) {
                val dy = e.rawY - downY
                // вверх — пока не развёрнуто; вниз — пока развёрнуто и список не прокручен
                val up = dy < -slop && t < 1f
                val down = dy > slop && t > 0f && scroll.scrollY == 0
                if (up || down) {
                    dragging = true
                    tracker = VelocityTracker.obtain()
                    parent?.requestDisallowInterceptTouchEvent(true)
                    return true
                }
            }
        }
        return false
    }

    override fun onTouchEvent(e: MotionEvent): Boolean {
        if (!dragging) return false
        tracker?.addMovement(e)
        when (e.actionMasked) {
            MotionEvent.ACTION_MOVE -> setProgress(startT - (e.rawY - downY) / range)
            MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> {
                tracker?.computeCurrentVelocity(1000)
                val v = tracker?.yVelocity ?: 0f   // отрицательная — вверх
                tracker?.recycle(); tracker = null
                dragging = false
                // любое движение вверх от начала жеста раскрывает панель до конца, вниз — сворачивает, даже если палец уже отпущен;
                // только явный быстрый бросок в обратную сторону меняет решение
                val moved = t - startT
                val target = when {
                    abs(v) > dp(900) -> if (v < 0) 1f else 0f
                    moved > 0f -> 1f
                    moved < 0f -> 0f
                    else -> if (startT > 0.5f) 1f else 0f
                }
                animateTo(target, abs(v), settle = true)
            }
        }
        return true
    }
}
