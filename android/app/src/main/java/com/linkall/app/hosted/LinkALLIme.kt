package com.linkall.app.hosted

import android.inputmethodservice.InputMethodService
import android.util.Log
import android.view.KeyEvent
import android.view.View
import android.view.inputmethod.EditorInfo
import android.view.inputmethod.InputMethodManager
import com.linkall.app.controller.InputInjector
import com.linkall.app.controller.KeyModifier

/**
 * LinkALL 自定义输入法（IME）—— 生产级增强版。
 *
 * 增强点：
 *   - **Modifier 合成**：injectKeyWithMods 支持 Ctrl / Alt / Shift / Meta 组合
 *   - **isInputViewShown 守护**：调用 inject 前确认 input view 已开（避免 silent fail）
 *   - **Auto-restore default IME**：onFinishInput / onDestroy 时如果检测到自己仍为"当前 IME"，
 *     可由调用方通过 switchToPreviousIme() 切回上一个 IME（不在 onDestroy 自动调以免打断正在打字）
 *   - **静态入口去反射**：通过 `ImeRegistry` 单例缓存当前 instance
 *
 * 用法：
 *   1) 用户在系统设置启用 LinkALL IME
 *   2) InputInjector.injectKey / injectText 走这里
 */
class LinkALLIme : InputMethodService() {

    @Volatile var isInputShown: Boolean = false
        private set

    override fun onCreate() {
        super.onCreate()
        ImeRegistry.set(this)
        InputInjector.attachIme(this)
        Log.i(TAG, "IME service created")
    }

    override fun onDestroy() {
        super.onDestroy()
        ImeRegistry.clear(this)
        InputInjector.detachIme()
    }

    override fun onCreateInputView(): View = View(this)

    override fun onStartInputView(info: EditorInfo?, restarting: Boolean) {
        super.onStartInputView(info, restarting)
        isInputShown = true
    }
    override fun onFinishInputView(finishingInput: Boolean) {
        super.onFinishInputView(finishingInput)
        isInputShown = false
    }

    fun injectText(text: String): Boolean {
        if (!isInputShown) {
            // 没有 input view 时 commitText 走系统焦点窗口
            Log.w(TAG, "injectText: input view not shown; commit may still work via currentInputConnection")
        }
        val ic = currentInputConnection ?: return false
        return try {
            ic.commitText(text, 1)
            true
        } catch (e: Throwable) {
            Log.w(TAG, "injectText failed: ${e.message}")
            false
        }
    }

    fun injectKey(keyCode: Int): Boolean {
        val ic = currentInputConnection ?: return false
        return try {
            val down = KeyEvent(KeyEvent.ACTION_DOWN, keyCode)
            val up = KeyEvent(KeyEvent.ACTION_UP, keyCode)
            ic.sendKeyEvent(down)
            ic.sendKeyEvent(up)
            true
        } catch (e: Throwable) {
            Log.w(TAG, "injectKey failed: ${e.message}")
            false
        }
    }

    /**
     * 发送带 modifier 的按键（如 Ctrl+a）。
     * 实现：先发 modifier DOWN，再发主键 DOWN/UP，最后发 modifier UP。
     */
    fun injectKeyWithMods(keyCode: Int, mods: Set<KeyModifier>): Boolean {
        val ic = currentInputConnection ?: return false
        return try {
            val downMap = mutableMapOf<Int, Boolean>()
            for (m in mods) {
                val modCode = when (m) {
                    KeyModifier.CTRL -> KeyEvent.KEYCODE_CTRL_LEFT
                    KeyModifier.ALT -> KeyEvent.KEYCODE_ALT_LEFT
                    KeyModifier.SHIFT -> KeyEvent.KEYCODE_SHIFT_LEFT
                    KeyModifier.META -> KeyEvent.KEYCODE_META_LEFT
                }
                ic.sendKeyEvent(KeyEvent(KeyEvent.ACTION_DOWN, modCode))
                downMap[modCode] = true
            }
            ic.sendKeyEvent(KeyEvent(KeyEvent.ACTION_DOWN, keyCode))
            ic.sendKeyEvent(KeyEvent(KeyEvent.ACTION_UP, keyCode))
            for ((modCode, _) in downMap) {
                ic.sendKeyEvent(KeyEvent(KeyEvent.ACTION_UP, modCode))
            }
            true
        } catch (e: Throwable) {
            Log.w(TAG, "injectKeyWithMods failed: ${e.message}")
            false
        }
    }

    fun injectDelete(): Boolean {
        val ic = currentInputConnection ?: return false
        return try {
            ic.deleteSurroundingText(1, 0)
            true
        } catch (_: Throwable) { false }
    }

    fun injectEnter(): Boolean = injectKey(KeyEvent.KEYCODE_ENTER)

    /**
     * 切回上一个 IME（在主控会话结束时调用，让用户回到常用输入法）。
     * 需要 android.permission.WRITE_SECURE_SETTINGS (系统级) 或者走
     * InputMethodManager.switchToPreviousInputMethod()（无权限，但需 IME 是当前 IME）。
     */
    fun switchToPreviousIme(): Boolean {
        return try {
            val imm = getSystemService(INPUT_METHOD_SERVICE) as? InputMethodManager ?: return false
            imm.switchToLastInputMethod(null)
        } catch (_: Throwable) { false }
    }

    companion object {
        const val TAG = "LinkALL/IME"
        // 静态入口（去反射）
        fun inject(text: String) { ImeRegistry.current?.injectText(text) }
        fun key(code: Int) { ImeRegistry.current?.injectKey(code) }
        fun backspace() { ImeRegistry.current?.injectDelete() }
        fun enter() { ImeRegistry.current?.injectEnter() }
        fun active(): Boolean = ImeRegistry.current != null
    }
}

/** IME 实例注册表（生产级：去反射） */
object ImeRegistry {
    @Volatile var current: LinkALLIme? = null
        private set
    fun set(ime: LinkALLIme) { current = ime }
    fun clear(ime: LinkALLIme) { if (current === ime) current = null }
}
