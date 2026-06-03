package com.linkall.app.hosted

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.GestureDescription
import android.graphics.Path
import android.os.Build
import android.util.Log
import android.view.KeyEvent
import com.linkall.app.controller.InputInjector
import android.view.accessibility.AccessibilityEvent
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow

/**
 * 键鼠注入服务。LinkALL 通过此服务实现对被控端 Android 设备的远程控制。
 *
 * - 控制器发来 mouse / key 指令时，主进程把数据写入 InputInjector，
 *   InputInjector 借助 Service 暴露的全局方法触发 performGlobalAction / dispatchGesture / KeyEvent 注入。
 *
 * 注意：
 *   - 在 Android 10+ 上 `flagInjectEvents` 才能在主线程外触发；本服务配置中已开启。
 *   - 真正的 dispatchKeyEvent 需要 KEYCODE_*，前端 WebSocket 端发来的 code 需要映射为 KeyEvent.KEYCODE_*。
 */
class LinkALLAccessibilityService : AccessibilityService() {

    override fun onServiceConnected() {
        super.onServiceConnected()
        instance = this
        InputInjector.attach(this)
        Log.i(TAG, "AccessibilityService connected")
    }

    override fun onUnbind(intent: android.content.Intent?): Boolean {
        instance = null
        InputInjector.detach()
        return super.onUnbind(intent)
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) {}

    override fun onInterrupt() {}

    override fun onKeyEvent(event: KeyEvent?): Boolean {
        // 这里通常返回 false（不消费），由系统继续分发
        return false
    }

    /** 全局动作（HOME / BACK / RECENTS / NOTIF / POWER 等） */
    fun performGlobal(id: Int): Boolean = performGlobalAction(id)

    /** 模拟滑动手势 */
    fun swipe(x1: Float, y1: Float, x2: Float, y2: Float, durationMs: Long) {
        val p = Path().apply { moveTo(x1, y1); lineTo(x2, y2) }
        val desc = GestureDescription.Builder()
            .addStroke(GestureDescription.StrokeDescription(p, 0, durationMs))
            .build()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
            dispatchGesture(desc, null, null)
        }
    }

    /** 模拟点击 */
    fun click(x: Float, y: Float) {
        val p = Path().apply { moveTo(x, y) }
        val desc = GestureDescription.Builder()
            .addStroke(GestureDescription.StrokeDescription(p, 0, 80))
            .build()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
            dispatchGesture(desc, null, null)
        }
    }

    companion object {
        const val TAG = "LinkALL/A11y"
        @Volatile var instance: LinkALLAccessibilityService? = null
    }
}
