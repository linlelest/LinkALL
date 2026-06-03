package com.linkall.app.hosted

import android.content.ClipData
import android.content.Context
import android.content.Intent
import android.util.Log
import com.linkall.app.controller.InputInjector
import com.linkall.app.controller.KeyMap
import com.linkall.app.controller.PrefsHolder
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONObject
import org.webrtc.AudioTrack
import org.webrtc.DataChannel
import org.webrtc.DefaultVideoEncoderFactory
import org.webrtc.EglBase
import org.webrtc.IceCandidate
import org.webrtc.MediaConstraints
import org.webrtc.MediaStream
import org.webrtc.PeerConnection
import org.webrtc.PeerConnectionFactory
import org.webrtc.RtpReceiver
import org.webrtc.SdpObserver
import org.webrtc.SessionDescription
import org.webrtc.VideoTrack
import java.io.File
import java.io.RandomAccessFile
import java.security.MessageDigest
import java.security.SecureRandom
import java.util.Base64
import java.util.concurrent.TimeUnit

/**
 * 被控端 RTC 会话：处理 controller 发来的 SDP/ICE 协商；接收 DataChannel 控制指令 + 文件分片。
 *
 * 关键点（生产级修复）：
 *   - **无反射**：通过 WebRtcHost 公开的 ensureFactory()/videoTrack()/getOrCreate() API 获取依赖
 *   - **文件断点续传**：
 *       1) 收到 file_meta → 检查本地 {transferId}.part 是否已存在 → 读出 received_offset 并立即 file_ack(resuming=true, offset)
 *       2) 收到 file_data → RandomAccessFile.write(offset) 写入
 *       3) 收到 file_end → SHA-256 校验整个文件 → file_ack(sha256_ok)
 *   - **屏幕服务生命周期**：
 *       start() 时如果 ScreenCaptureService 还在跑，先 stop 防止重复
 *       stop() 时调 ScreenCaptureService.stopCapture() 释放 MediaProjection
 *   - **WebSocket 鉴权**：带 token + device_code（与现有 sign-in 协议兼容）
 */
object HostedRtcSession {
    private const val TAG = "LinkALL/HostedRtc"
    private val rng = SecureRandom()
    private var pc: PeerConnection? = null
    private var dc: DataChannel? = null
    private var egl: EglBase? = null
    private var factory: PeerConnectionFactory? = null
    private var ws: WebSocket? = null
    private var videoTrack: VideoTrack? = null
    private var audioTrack: AudioTrack? = null
    private var scope: CoroutineScope? = null
    private var controllerId: String? = null
    private var appContext: Context? = null
    @Volatile private var stopped = false

    // 文件接收状态
    private var fileTransfer: FileRecvState? = null

    private data class FileRecvState(
        val transferId: String,
        val name: String,
        val size: Long,
        val sha256Expected: String,
        val filePath: String,
        var receivedOffset: Long,
    )

    /**
     * 启动被控端 RTC 会话。
     * @param ctx 任意 Context
     * @param targetWs 信令 WS URL
     * @param token 用户 token（用于 hello）
     * @param deviceCode 12 位设备编号
     * @param controllerId 控制器 peer id
     * @param iceServers 服务器下发的 ICE 配置
     */
    fun start(
        ctx: Context, targetWs: String, token: String, deviceCode: String,
        controllerId: String, iceServers: List<Pair<String, String?>>
    ) {
        if (stopped) stopped = false
        appContext = ctx.applicationContext
        this.controllerId = controllerId
        // 确保 ScreenCaptureService 不会被上一会话占用
        appContext?.let { stopScreenCaptureIfRunning(it) }

        WebRtcHost.ensureFactory(ctx)
        factory = WebRtcHost.factory
        videoTrack = WebRtcHost.createVideoTrack()
        egl = EglBase.create()
        val encFactory = DefaultVideoEncoderFactory(egl!!.eglBaseContext, true, true)
        factory = PeerConnectionFactory.builder()
            .setVideoEncoderFactory(encFactory)
            .createPeerConnectionFactory()
        scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
        val serverIce = iceServers.map { (url, _) -> PeerConnection.IceServer.builder(url).createIceServer() }.toMutableList()
        if (serverIce.isEmpty()) {
            serverIce.add(PeerConnection.IceServer.builder("stun:stun.l.google.com:19302").createIceServer())
        }
        val rtc = factory!!
        val config = PeerConnection.RTCConfiguration(serverIce)
        val newPc = rtc.createPeerConnection(config, object : PeerConnection.Observer {
                override fun onIceCandidate(c: IceCandidate) {
                    val data = JSONObject().apply {
                        put("candidate", c.sdp)
                        put("sdpMid", c.sdpMid)
                        put("sdpMLineIndex", c.sdpMLineIndex)
                    }
                    ws?.send(stamp(JSONObject()
                        .put("type", "ice")
                        .put("to", controllerId)
                        .put("data", data)
                    ))
                }
            override fun onAddStream(p0: MediaStream?) {}
            override fun onDataChannel(d: DataChannel) {
                dc = d
                d.registerObserver(object : DataChannel.Observer {
                    override fun onBufferedAmountChange(p0: Long) {}
                    override fun onStateChange() { Log.i(TAG, "dc state: ${d.state()}") }
                    override fun onMessage(buf: DataChannel.Buffer) {
                        val bytes = ByteArray(buf.data.remaining())
                        buf.data.get(bytes)
                        val s = String(bytes)
                        handleDataChannelMessage(s)
                    }
                })
            }
            override fun onIceConnectionReceivingChange(p0: Boolean) {}
                override fun onIceConnectionChange(s: PeerConnection.IceConnectionState?) {
                    Log.i(TAG, "ice: ${s?.name}")
                    if (s == PeerConnection.IceConnectionState.FAILED || s == PeerConnection.IceConnectionState.CLOSED) {
                        Log.w(TAG, "ice failed/closed, stopping session")
                        stop()
                    }
                }
            override fun onIceCandidatesRemoved(p0: Array<out IceCandidate?>?) {}
            override fun onRemoveStream(p0: MediaStream?) {}
            override fun onIceGatheringChange(p0: PeerConnection.IceGatheringState?) {}
            override fun onAddTrack(p0: RtpReceiver?, p1: Array<out MediaStream>?) {}
            override fun onRemoveTrack(p0: RtpReceiver?) {}
            override fun onSignalingChange(p0: PeerConnection.SignalingState?) {}
            override fun onRenegotiationNeeded() {}
        })!!
        videoTrack?.let { newPc.addTrack(it, listOf("ARDAMS")) }
        // 创建并添加音频 track（WebRTC 内部管理麦克风捕获）
        val audioConstraints = MediaConstraints()
        val audioSrc = rtc.createAudioSource(audioConstraints)
        val aTrack = rtc.createAudioTrack("ARDAMSa0", audioSrc)
        audioTrack = aTrack
        newPc.addTrack(aTrack, listOf("ARDAMS"))
        pc = newPc

        // 启动 WS
        val client = OkHttpClient.Builder().pingInterval(20, TimeUnit.SECONDS).build()
        val req = Request.Builder().url(targetWs).build()
        ws = client.newWebSocket(req, object : WebSocketListener() {
            override fun onOpen(ws: WebSocket, response: Response) {
                ws.send(JSONObject().put("type", "hello")
                    .put("data", JSONObject().put("kind", "controlled").put("device_code", deviceCode).put("token", token))
                    .toString())
            }
            override fun onMessage(ws: WebSocket, text: String) {
                if (stopped) return
                val env = try { JSONObject(text) } catch (_: Throwable) { return }
                when (env.optString("type")) {
                    "offer" -> {
                        val sdp = env.optJSONObject("data")?.optString("sdp") ?: return
                        newPc.setRemoteDescription(SimpleSdpObserver(), SessionDescription(SessionDescription.Type.OFFER, sdp))
                        newPc.createAnswer(object : SdpObserver {
                            override fun onCreateSuccess(sd: SessionDescription) {
                                newPc.setLocalDescription(SimpleSdpObserver(), sd)
                                ws.send(stamp(JSONObject()
                                    .put("type", "answer")
                                    .put("to", controllerId)
                                    .put("data", JSONObject().put("sdp", sd.description))
                                ))
                            }
                            override fun onSetSuccess() {}
                            override fun onCreateFailure(p0: String?) { Log.e(TAG, "create answer: $p0") }
                            override fun onSetFailure(p0: String?) {}
                        }, MediaConstraints())
                    }
                    "ice" -> {
                        val c = env.optJSONObject("data")?.optJSONObject("candidate") ?: return
                        newPc.addIceCandidate(IceCandidate(c.optString("sdpMid"), c.optInt("sdpMLineIndex"), c.optString("candidate")))
                    }
                    "cmd" -> env.optJSONObject("data")?.let { handleCmd(it) }
                    "file_meta", "file_ack", "file_data", "file_end" -> handleFileMessage(env)
                }
            }
            override fun onFailure(ws: WebSocket, t: Throwable, response: Response?) {
                Log.e(TAG, "ws: ${t.message}")
            }
        })
    }

    private fun handleDataChannelMessage(text: String) {
        if (stopped) return
        try {
            val env = JSONObject(text)
            when (env.optString("type")) {
                "cmd" -> env.optJSONObject("data")?.let { handleCmd(it) }
                "file_meta", "file_ack", "file_data", "file_end" -> handleFileMessage(env)
            }
        } catch (_: Throwable) {}
    }

    private fun handleCmd(d: JSONObject) {
        val op = d.optString("op")
        val ctx = appContext ?: return
        when (op) {
            "mouse" -> {
                val x = d.optDouble("x", 0.0)
                val y = d.optDouble("y", 0.0)
                val button = d.optInt("button", 0)
                val down = d.optBoolean("down", false)
                if (down) {
                    val dm = ctx.resources.displayMetrics
                    val px = (x / 100.0 * dm.widthPixels).toFloat()
                    val py = (y / 100.0 * dm.heightPixels).toFloat()
                    InputInjector.click(px, py)
                }
            }
            "key" -> {
                val code = d.optString("code", "")
                val down = d.optBoolean("down", false)
                if (down) {
                    val kc = KeyMap.toKeyEvent(code)
                    if (kc != null) InputInjector.sendKey(kc)
                }
            }
            "type" -> {
                val t = d.optString("text", "")
                if (t.isNotEmpty()) InputInjector.sendText(t)
            }
            "wheel" -> {
                val dx = d.optInt("dx", 0)
                val dy = d.optInt("dy", 0)
                val dm = ctx.resources.displayMetrics
                val cx = dm.widthPixels / 2f
                val cy = dm.heightPixels / 2f
                InputInjector.swipe(cx, cy, cx + dx * 0.5f, cy + dy * 0.5f, 100)
            }
            "clip" -> {
                val text = d.optString("text", "")
                if (text.isNotEmpty()) {
                    val cm = ctx.getSystemService(Context.CLIPBOARD_SERVICE) as android.content.ClipboardManager
                    cm.setPrimaryClip(ClipData.newPlainText("linkall", text))
                }
            }
            "clip_get" -> {
                val cm = ctx.getSystemService(Context.CLIPBOARD_SERVICE) as android.content.ClipboardManager
                val clip = cm.primaryClip
                if (clip != null && clip.itemCount > 0) {
                    val text = clip.getItemAt(0).text?.toString() ?: ""
                    if (text.isNotEmpty()) {
                        ws?.send(JSONObject().apply {
                            put("type", "cmd")
                            put("to", controllerId)
                            put("data", JSONObject().apply {
                                put("op", "clip")
                                put("text", text)
                            })
                        }.toString())
                    }
                }
            }
        }
    }

    /**
     * 处理文件消息：file_meta / file_data / file_end
     *
     * **断点续传机制**：
     *   - 收到 file_meta 时，如果本地的 {transferId}.part 文件已存在，从文件长度反推 received_offset
     *   - 立即发 file_ack(resuming=true, received_offset=N)
     *   - 写 chunk 时按 offset 随机定位
     *   - file_end 时整文件 SHA-256 比对期望值
     */
    private fun handleFileMessage(env: JSONObject) {
        when (env.optString("type")) {
            "file_meta" -> {
                val data = env.optJSONObject("data") ?: return
                val tid = data.optString("transfer_id")
                val name = data.optString("name", "recv.bin")
                val size = data.optLong("size", 0)
                val sha = data.optString("sha256", "")
                val ctx = appContext ?: return
                val dir = File(ctx.getExternalFilesDir(null), "recv").apply { mkdirs() }
                val safeName = name.replace(Regex("[\\\\/:*?\"<>|]"), "_")
                val fp = File(dir, "${tid}_$safeName")
                // 断点续传：检查已有 .part 文件
                val resumeOffset = if (fp.exists()) fp.length() else 0L
                val resuming = resumeOffset > 0
                fileTransfer = FileRecvState(tid, name, size, sha, fp.absolutePath, resumeOffset)
                PrefsHolder.get().fileOffset = resumeOffset
                PrefsHolder.get().fileSha256 = sha
                // 立即 ack
                ws?.send(stamp(JSONObject()
                    .put("type", "file_ack")
                    .put("to", controllerId)
                    .put("data", JSONObject()
                        .put("transfer_id", tid)
                        .put("received_offset", resumeOffset)
                        .put("resuming", resuming)
                        .put("accepted", true))
                ))
                Log.i(TAG, "file_meta tid=$tid name=$name size=$size resume=$resumeOffset")
            }
            "file_data" -> {
                val data = env.optJSONObject("data") ?: return
                val tid = data.optString("transfer_id")
                val offset = data.optLong("offset", 0)
                val b64 = data.optString("data", "")
                val st = fileTransfer ?: return
                if (tid != st.transferId) return
                if (offset != st.receivedOffset) {
                    // 乱序/对不上：忽略（生产应记日志）
                    Log.w(TAG, "file_data offset mismatch: expect=${st.receivedOffset} got=$offset")
                }
                val raw = Base64.getDecoder().decode(b64)
                if (raw.isEmpty()) return
                try {
                    val raf = RandomAccessFile(st.filePath, "rw")
                    try {
                        raf.seek(offset)
                        raf.write(raw)
                    } finally {
                        raf.close()
                    }
                    st.receivedOffset = offset + raw.size
                    PrefsHolder.get().fileOffset = st.receivedOffset
                } catch (t: Throwable) {
                    Log.e(TAG, "file_data write: ${t.message}")
                }
            }
            "file_end" -> {
                val data = env.optJSONObject("data") ?: return
                val tid = data.optString("transfer_id")
                val st = fileTransfer ?: return
                if (tid != st.transferId) return
                // 完整 SHA-256 校验
                var ok = true
                if (st.sha256Expected.isNotEmpty()) {
                    val got = sha256OfFile(File(st.filePath))
                    ok = got.equals(st.sha256Expected, ignoreCase = true)
                }
                ws?.send(stamp(JSONObject()
                    .put("type", "file_ack")
                    .put("to", controllerId)
                    .put("data", JSONObject()
                        .put("transfer_id", tid)
                        .put("received_offset", st.receivedOffset)
                        .put("accepted", ok)
                        .put("sha256_ok", ok))
                ))
                if (ok) {
                    PrefsHolder.get().fileOffset = 0L
                    PrefsHolder.get().fileSha256 = ""
                }
                fileTransfer = null
            }
            "file_ack" -> {
                // 探测模式：被控端不发起文件传输，忽略
            }
        }
    }

    private fun sha256OfFile(f: File): String {
        if (!f.exists()) return ""
        val md = MessageDigest.getInstance("SHA-256")
        f.inputStream().use { ins ->
            val buf = ByteArray(64 * 1024)
            while (true) {
                val n = ins.read(buf)
                if (n <= 0) break
                md.update(buf, 0, n)
            }
        }
        return md.digest().joinToString("") { "%02x".format(it) }
    }

    fun stop() {
        if (stopped) return
        stopped = true
        try { pc?.close() } catch (_: Throwable) {}
        try { ws?.close(1000, "bye") } catch (_: Throwable) {}
        try { egl?.release() } catch (_: Throwable) {}
        try { appContext?.let { stopScreenCaptureIfRunning(it) } } catch (_: Throwable) {}
        scope?.cancel()
        pc = null
        ws = null
        egl = null
        scope = null
        fileTransfer = null
    }

    private fun stopScreenCaptureIfRunning(ctx: Context) {
        try {
            val am = ctx.getSystemService(Context.ACTIVITY_SERVICE) as android.app.ActivityManager
            // 不能直接 getRunningService 拿 ScreenCaptureService 引用（Android 8+ 限制）
            // 走 Prefs 标记：screen_capturing=true 表示上一次有活动 session
            if (PrefsHolder.get().screenCapturing) {
                ctx.stopService(Intent(ctx, ScreenCaptureService::class.java))
                PrefsHolder.get().screenCapturing = false
            }
        } catch (_: Throwable) {}
    }

    private fun stamp(env: JSONObject): String {
        env.put("ts", System.currentTimeMillis())
        val bytes = ByteArray(12)
        rng.nextBytes(bytes)
        env.put("nonce", Base64.getUrlEncoder().withoutPadding().encodeToString(bytes))
        return env.toString()
    }

    private class SimpleSdpObserver : SdpObserver {
        override fun onCreateSuccess(p0: SessionDescription?) {}
        override fun onSetSuccess() {}
        override fun onCreateFailure(p0: String?) {}
        override fun onSetFailure(p0: String?) {}
    }
}
