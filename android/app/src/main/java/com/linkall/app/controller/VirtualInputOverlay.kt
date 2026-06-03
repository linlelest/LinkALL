package com.linkall.app.controller

import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.unit.IntSize

/**
 * 覆盖在视频上层的透明触控层。点击 = 鼠标左键单击，拖动 = 鼠标移动 + 按住。
 *
 * onMouse 回调里 controller 把它转成 `mouse` cmd 发送给被控端。
 */
@Composable
fun VirtualInputOverlay(
    onMouse: (xPct: Double, yPct: Double, button: Int, down: Boolean) -> Unit,
    onKey: (code: String, down: Boolean) -> Unit,
    onType: (String) -> Unit
) {
    var size = remember { mutableStateOf(IntSize.Zero) }
    Box(
        Modifier
            .fillMaxSize()
            .background(Color.Transparent)
            .onSizeChanged { size.value = it }
            .pointerInput(Unit) {
                detectTapGestures(
                    onTap = { off ->
                        val w = size.value.width.toDouble().coerceAtLeast(1.0)
                        val h = size.value.height.toDouble().coerceAtLeast(1.0)
                        onMouse(off.x / w * 100.0, off.y / h * 100.0, 0, true)
                        onMouse(off.x / w * 100.0, off.y / h * 100.0, 0, false)
                    },
                    onLongPress = { off ->
                        val w = size.value.width.toDouble().coerceAtLeast(1.0)
                        val h = size.value.height.toDouble().coerceAtLeast(1.0)
                        onMouse(off.x / w * 100.0, off.y / h * 100.0, 2, true)
                    }
                )
            }
            .pointerInput(Unit) {
                detectDragGestures(
                    onDragStart = { off ->
                        val w = size.value.width.toDouble().coerceAtLeast(1.0)
                        val h = size.value.height.toDouble().coerceAtLeast(1.0)
                        onMouse(off.x / w * 100.0, off.y / h * 100.0, 0, true)
                    },
                    onDrag = { change, _ ->
                        val w = size.value.width.toDouble().coerceAtLeast(1.0)
                        val h = size.value.height.toDouble().coerceAtLeast(1.0)
                        val pos: Offset = change.position
                        onMouse(pos.x / w * 100.0, pos.y / h * 100.0, 0, true)
                    },
                    onDragEnd = {
                        onMouse(50.0, 50.0, 0, false)
                    }
                )
            }
    )
}

import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
