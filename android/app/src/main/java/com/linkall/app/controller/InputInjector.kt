package com.linkall.app.controller

import android.util.Log
import android.view.KeyEvent
import com.linkall.app.hosted.LinkALLAccessibilityService
import com.linkall.app.hosted.LinkALLIme

/**
 * 全局键鼠注入器（生产级）。
 *
 * 优先级：
 *   1) 字符流（text）→ 只能走 IME（commitText）
 *   2) 控制键（key）→ 优先 IME（sendKeyEvent），回退 a11y（performGlobalAction 或回 KEYCODE_）
 *   3) 点击 / 滑动 → 只能 a11y
 *
 * **IME 状态广播**：isImeActive() 暴露给上层（ControllerScreen）显示状态；
 *   若 IME 未启用，sendKey/sendText 返回 false 让 UI 弹"请先启用 LinkALLIme"提示。
 */
object InputInjector {
    private const val TAG = "LinkALL/Input"
    @Volatile private var a11y: LinkALLAccessibilityService? = null
    @Volatile private var ime: LinkALLIme? = null

    fun attach(s: LinkALLAccessibilityService) { a11y = s }
    fun detach() { a11y = null }
    fun attachIme(s: LinkALLIme) { ime = s }
    fun detachIme() { ime = null }

    /** IME 是否绑定到当前 InputMethodManager */
    fun isImeActive(): Boolean = ime != null

    fun click(x: Float, y: Float): Boolean {
        val s = a11y ?: return false
        s.click(x, y); return true
    }
    fun swipe(x1: Float, y1: Float, x2: Float, y2: Float, durationMs: Long = 200): Boolean {
        val s = a11y ?: return false
        s.swipe(x1, y1, x2, y2, durationMs); return true
    }
    fun global(id: Int): Boolean = a11y?.performGlobal(id) ?: false

    /**
     * 发送控制键。code 形如 "a" / "Enter" / "Ctrl+a" / "Shift+Tab"。
     * IME 未启用时返回 false。
     */
    fun sendKey(code: String): Boolean {
        // 解析 modifier 前缀
        val parts = code.split("+").map { it.trim() }
        val mods = mutableSetOf<KeyModifier>()
        var mainCode = code
        if (parts.size > 1) {
            mainCode = parts.last()
            for (m in parts.dropLast(1)) {
                when (m.lowercase()) {
                    "ctrl", "control" -> mods.add(KeyModifier.CTRL)
                    "alt" -> mods.add(KeyModifier.ALT)
                    "shift" -> mods.add(KeyModifier.SHIFT)
                    "meta", "win", "cmd" -> mods.add(KeyModifier.META)
                }
            }
        }
        val keyCode = KeyMap.toKeyEvent(mainCode) ?: return false
        return ime?.injectKeyWithMods(keyCode, mods) ?: false
    }

    /**
     * 发送 Raw 整数 keyCode（已有）
     */
    fun sendKey(keyCode: Int): Boolean {
        return ime?.injectKey(keyCode) ?: false
    }

    fun sendText(text: String): Boolean {
        return ime?.injectText(text) ?: false
    }

    fun sendEnter(): Boolean = ime?.injectEnter() ?: false
    fun sendBackspace(): Boolean = ime?.injectDelete() ?: false
}

enum class KeyModifier { CTRL, ALT, SHIFT, META }

/**
 * 把控制端发来的 code 字符串映射到 KeyEvent.KEYCODE_*
 * - 完整 US 键盘布局：a-z / A-Z / 0-9 / 常用符号
 * - 控制键：Enter / Tab / Escape / Arrow / F1-F12
 * - 中文 / 日文输入：依赖 IME 自带的多语言模式；本映射不直接处理 IME composition
 */
object KeyMap {
    fun toKeyEvent(code: String): Int? = when (code) {
        // 控制键
        "Enter" -> KeyEvent.KEYCODE_ENTER
        "Backspace" -> KeyEvent.KEYCODE_DEL
        "Tab" -> KeyEvent.KEYCODE_TAB
        "Escape" -> KeyEvent.KEYCODE_ESCAPE
        "Space" -> KeyEvent.KEYCODE_SPACE
        "ArrowUp", "Up" -> KeyEvent.KEYCODE_DPAD_UP
        "ArrowDown", "Down" -> KeyEvent.KEYCODE_DPAD_DOWN
        "ArrowLeft", "Left" -> KeyEvent.KEYCODE_DPAD_LEFT
        "ArrowRight", "Right" -> KeyEvent.KEYCODE_DPAD_RIGHT
        "Home" -> KeyEvent.KEYCODE_HOME
        "Back" -> KeyEvent.KEYCODE_BACK
        "Delete" -> KeyEvent.KEYCODE_FORWARD_DEL
        "Insert" -> KeyEvent.KEYCODE_INSERT
        "PageUp" -> KeyEvent.KEYCODE_PAGE_UP
        "PageDown" -> KeyEvent.KEYCODE_PAGE_DOWN
        "CapsLock" -> KeyEvent.KEYCODE_CAPS_LOCK
        "NumLock" -> KeyEvent.KEYCODE_NUM_LOCK
        "ScrollLock" -> KeyEvent.KEYCODE_SCROLL_LOCK
        "PrintScreen" -> KeyEvent.KEYCODE_PRINTSCREEN
        "Pause", "Break" -> KeyEvent.KEYCODE_BREAK
        "ContextMenu" -> KeyEvent.KEYCODE_MENU
        // 字母
        "a" -> KeyEvent.KEYCODE_A; "A" -> KeyEvent.KEYCODE_A
        "b" -> KeyEvent.KEYCODE_B; "B" -> KeyEvent.KEYCODE_B
        "c" -> KeyEvent.KEYCODE_C; "C" -> KeyEvent.KEYCODE_C
        "d" -> KeyEvent.KEYCODE_D; "D" -> KeyEvent.KEYCODE_D
        "e" -> KeyEvent.KEYCODE_E; "E" -> KeyEvent.KEYCODE_E
        "f" -> KeyEvent.KEYCODE_F; "F" -> KeyEvent.KEYCODE_F
        "g" -> KeyEvent.KEYCODE_G; "G" -> KeyEvent.KEYCODE_G
        "h" -> KeyEvent.KEYCODE_H; "H" -> KeyEvent.KEYCODE_H
        "i" -> KeyEvent.KEYCODE_I; "I" -> KeyEvent.KEYCODE_I
        "j" -> KeyEvent.KEYCODE_J; "J" -> KeyEvent.KEYCODE_J
        "k" -> KeyEvent.KEYCODE_K; "K" -> KeyEvent.KEYCODE_K
        "l" -> KeyEvent.KEYCODE_L; "L" -> KeyEvent.KEYCODE_L
        "m" -> KeyEvent.KEYCODE_M; "M" -> KeyEvent.KEYCODE_M
        "n" -> KeyEvent.KEYCODE_N; "N" -> KeyEvent.KEYCODE_N
        "o" -> KeyEvent.KEYCODE_O; "O" -> KeyEvent.KEYCODE_O
        "p" -> KeyEvent.KEYCODE_P; "P" -> KeyEvent.KEYCODE_P
        "q" -> KeyEvent.KEYCODE_Q; "Q" -> KeyEvent.KEYCODE_Q
        "r" -> KeyEvent.KEYCODE_R; "R" -> KeyEvent.KEYCODE_R
        "s" -> KeyEvent.KEYCODE_S; "S" -> KeyEvent.KEYCODE_S
        "t" -> KeyEvent.KEYCODE_T; "T" -> KeyEvent.KEYCODE_T
        "u" -> KeyEvent.KEYCODE_U; "U" -> KeyEvent.KEYCODE_U
        "v" -> KeyEvent.KEYCODE_V; "V" -> KeyEvent.KEYCODE_V
        "w" -> KeyEvent.KEYCODE_W; "W" -> KeyEvent.KEYCODE_W
        "x" -> KeyEvent.KEYCODE_X; "X" -> KeyEvent.KEYCODE_X
        "y" -> KeyEvent.KEYCODE_Y; "Y" -> KeyEvent.KEYCODE_Y
        "z" -> KeyEvent.KEYCODE_Z; "Z" -> KeyEvent.KEYCODE_Z
        // 数字
        "0" -> KeyEvent.KEYCODE_0; "1" -> KeyEvent.KEYCODE_1
        "2" -> KeyEvent.KEYCODE_2; "3" -> KeyEvent.KEYCODE_3
        "4" -> KeyEvent.KEYCODE_4; "5" -> KeyEvent.KEYCODE_5
        "6" -> KeyEvent.KEYCODE_6; "7" -> KeyEvent.KEYCODE_7
        "8" -> KeyEvent.KEYCODE_8; "9" -> KeyEvent.KEYCODE_9
        // 数字小键盘
        "Numpad0" -> KeyEvent.KEYCODE_NUMPAD_0; "Numpad1" -> KeyEvent.KEYCODE_NUMPAD_1
        "Numpad2" -> KeyEvent.KEYCODE_NUMPAD_2; "Numpad3" -> KeyEvent.KEYCODE_NUMPAD_3
        "Numpad4" -> KeyEvent.KEYCODE_NUMPAD_4; "Numpad5" -> KeyEvent.KEYCODE_NUMPAD_5
        "Numpad6" -> KeyEvent.KEYCODE_NUMPAD_6; "Numpad7" -> KeyEvent.KEYCODE_NUMPAD_7
        "Numpad8" -> KeyEvent.KEYCODE_NUMPAD_8; "Numpad9" -> KeyEvent.KEYCODE_NUMPAD_9
        "NumpadAdd" -> KeyEvent.KEYCODE_NUMPAD_ADD
        "NumpadSubtract" -> KeyEvent.KEYCODE_NUMPAD_SUBTRACT
        "NumpadMultiply" -> KeyEvent.KEYCODE_NUMPAD_MULTIPLY
        "NumpadDivide" -> KeyEvent.KEYCODE_NUMPAD_DIVIDE
        "NumpadEnter" -> KeyEvent.KEYCODE_NUMPAD_ENTER
        "NumpadDot" -> KeyEvent.KEYCODE_NUMPAD_DOT
        // 符号
        "Minus" -> KeyEvent.KEYCODE_MINUS
        "Equal" -> KeyEvent.KEYCODE_EQUALS
        "Comma" -> KeyEvent.KEYCODE_COMMA
        "Period" -> KeyEvent.KEYCODE_PERIOD
        "Slash" -> KeyEvent.KEYCODE_SLASH
        "Backslash" -> KeyEvent.KEYCODE_BACKSLASH
        "Semicolon" -> KeyEvent.KEYCODE_SEMICOLON
        "Quote" -> KeyEvent.KEYCODE_APOSTROPHE
        "BracketLeft" -> KeyEvent.KEYCODE_LEFT_BRACKET
        "BracketRight" -> KeyEvent.KEYCODE_RIGHT_BRACKET
        "Backquote" -> KeyEvent.KEYCODE_GRAVE
        // F1-F12
        "F1" -> KeyEvent.KEYCODE_F1; "F2" -> KeyEvent.KEYCODE_F2
        "F3" -> KeyEvent.KEYCODE_F3; "F4" -> KeyEvent.KEYCODE_F4
        "F5" -> KeyEvent.KEYCODE_F5; "F6" -> KeyEvent.KEYCODE_F6
        "F7" -> KeyEvent.KEYCODE_F7; "F8" -> KeyEvent.KEYCODE_F8
        "F9" -> KeyEvent.KEYCODE_F9; "F10" -> KeyEvent.KEYCODE_F10
        "F11" -> KeyEvent.KEYCODE_F11; "F12" -> KeyEvent.KEYCODE_F12
        else -> null
    }
}
