package com.linkall.app.ui.controller

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Slider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import com.linkall.app.R
import com.linkall.app.controller.VirtualInputOverlay
import com.linkall.app.controller.WebRtcController
import com.linkall.app.ui.components.Card
import com.linkall.app.ui.components.GhostButton
import com.linkall.app.ui.components.LabeledSwitch
import com.linkall.app.ui.components.MutedText
import com.linkall.app.ui.components.PrimaryButton
import com.linkall.app.ui.theme.Bg
import kotlinx.coroutines.launch
import org.webrtc.RendererCommon
import org.webrtc.SurfaceViewRenderer
import org.webrtc.VideoTrack

@Composable
fun ControllerScreen() {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val controller = remember { WebRtcController(ctx) }
    var target by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var mode by remember { mutableStateOf("anonymous") }
    var status by remember { mutableStateOf("idle") }
    var scale by remember { mutableStateOf(1f) }
    var bitrateKbps by remember { mutableStateOf(4096f) }
    var fps by remember { mutableStateOf(30f) }
    var codec by remember { mutableStateOf("h264") }
    var privacy by remember { mutableStateOf(false) }
    var showKb by remember { mutableStateOf(false) }
    var videoTrack by remember { mutableStateOf<VideoTrack?>(null) }
    var renderer by remember { mutableStateOf<SurfaceViewRenderer?>(null) }

    DisposableEffect(Unit) {
        onDispose { controller.close() }
    }

    LaunchedEffect(videoTrack, renderer) {
        videoTrack?.addSink(renderer)
    }

    Column(modifier = Modifier.fillMaxSize().padding(12.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Text(stringResource(R.string.ctrl_title), style = MaterialTheme.typography.titleLarge)

        Card {
            OutlinedTextField(value = target, onValueChange = { target = it.uppercase() }, label = { Text(stringResource(R.string.ctrl_code)) }, singleLine = true, modifier = Modifier.fillMaxWidth())
            Spacer(Modifier.height(4.dp))
            OutlinedTextField(value = password, onValueChange = { password = it }, label = { Text(stringResource(R.string.ctrl_password)) }, singleLine = true, modifier = Modifier.fillMaxWidth())
            Spacer(Modifier.height(4.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                GhostButton(stringResource(R.string.ctrl_mode_account), modifier = Modifier.weight(1f)) { mode = "account" }
                GhostButton(stringResource(R.string.ctrl_mode_anon), modifier = Modifier.weight(1f)) { mode = "anonymous" }
            }
            Spacer(Modifier.height(8.dp))
            PrimaryButton(stringResource(R.string.ctrl_start), modifier = Modifier.fillMaxWidth()) {
                status = "connecting"
                scope.launch {
                    runCatching { controller.connect(target.trim(), password, mode) { t, r ->
                        videoTrack = t
                        renderer = r
                        status = controller.status
                    } }.onFailure { status = "err: ${it.message}" }
                }
            }
            MutedText("状态: $status")
        }

        // 视频区
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(280.dp)
                .background(Bg)
        ) {
            AndroidView(
                factory = { ctx ->
                    SurfaceViewRenderer(ctx).apply {
                        setMirror(false)
                        setEnableHardwareScaler(true)
                        setScalingType(RendererCommon.ScalingType.SCALE_ASPECT_FIT)
                    }
                },
                update = { v -> renderer = v },
                modifier = Modifier.fillMaxSize()
            )
            VirtualInputOverlay(
                onMouse = { x, y, btn, down -> controller.sendMouse(x, y, btn, down) },
                onKey = { code, down -> controller.sendKey(code, down) },
                onType = { t -> controller.sendType(t) }
            )
        }

        // 参数
        Card {
            MutedText(stringResource(R.string.ctrl_scale) + ": ${(scale * 100).toInt()}%")
            Slider(value = scale, onValueChange = { scale = it }, valueRange = 0.1f..3f)
            MutedText(stringResource(R.string.ctrl_bitrate) + ": ${bitrateKbps.toInt()} kbps")
            Slider(value = bitrateKbps, onValueChange = { bitrateKbps = it }, valueRange = 512f..200_000f)
            MutedText(stringResource(R.string.ctrl_fps) + ": ${fps.toInt()}")
            Slider(value = fps, onValueChange = { fps = it }, valueRange = 15f..144f)
            LabeledSwitch(stringResource(R.string.ctrl_privacy), privacy) { v ->
                privacy = v
                controller.sendCmd("""{"op":"privacy","enabled":${v}}""")
            }
        }

        Card {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                GhostButton(stringResource(R.string.ctrl_kb), modifier = Modifier.weight(1f)) { showKb = !showKb }
                GhostButton(stringResource(R.string.ctrl_send_file), modifier = Modifier.weight(1f)) { /* TODO SAF */ }
                GhostButton(stringResource(R.string.ctrl_exit), modifier = Modifier.weight(1f)) { controller.close() }
            }
        }
    }
}
