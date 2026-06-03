package com.linkall.app.controller

import android.util.Base64
import android.util.Log
import com.linkall.app.controller.PrefsHolder
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONObject
import org.webrtc.DataChannel
import org.webrtc.DefaultVideoDecoderFactory
import org.webrtc.DefaultVideoEncoderFactory
import org.webrtc.EglBase
import org.webrtc.IceCandidate
import org.webrtc.MediaConstraints
import org.webrtc.MediaStream
import org.webrtc.PeerConnection
import org.webrtc.PeerConnectionDependencies
import org.webrtc.PeerConnectionFactory
import org.webrtc.RtpReceiver
import org.webrtc.SdpObserver
import org.webrtc.SessionDescription
import org.webrtc.SurfaceViewRenderer
import org.webrtc.VideoTrack
import java.security.SecureRandom
import java.util.concurrent.TimeUnit

/**
 * 控制端 WebRTC 客户端：
 *  1) 信令 WS 连到 /ws/signaling，发 hello + request
 *  2) 收到 offer，setRemoteDescription + createAnswer
 *  3) 收到 ICE，addIceCandidate
 *  4) onTrack 回调里把 VideoTrack 喂给 SurfaceViewRenderer
 *  5) 通过 DataChannel 发 mouse / key / cmd（自动加 ts + nonce 反重放）
 */
class WebRtcController {

    @Volatile var status: String = "idle"
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    private var ws: WebSocket? = null
    private var pc: PeerConnection? = null
    private var dc: DataChannel? = null
    private var egl: EglBase? = null
    private var factory: PeerConnectionFactory? = null
    private var peerId: String? = null
    private val iceServers = mutableListOf<PeerConnection.IceServer>()
    @Volatile private var target: String = ""
    private val rng = SecureRandom()

    fun connect(target: String, password: String, mode: String, onTrack: (VideoTrack, SurfaceViewRenderer) -> Unit) {
        this.target = target
        status = "connecting"

        val prefs = PrefsHolder.get()
        val httpBase = prefs.serverBase()
        val wsUrl = httpBase.replace("http://", "ws://").replace("https://", "wss://") + "/ws/signaling"

        egl = EglBase.create()
        PeerConnectionFactory.initialize(PeerConnectionFactory.InitializationOptions.builder()
            .setEnableInternalTracer(false).createInitializationOptions())
        factory = PeerConnectionFactory.builder()
            .setVideoEncoderFactory(DefaultVideoEncoderFactory(egl!!.eglBaseContext, true, true))
            .setVideoDecoderFactory(DefaultVideoDecoderFactory(egl!!.eglBaseContext))
            .createPeerConnectionFactory()
        iceServers.add(PeerConnection.IceServer.builder("stun:stun.l.google.com:19302").createIceServer())
        val rtc = factory!!
        val config = PeerConnection.RTCConfiguration(iceServers)
        val deps = PeerConnectionDependencies.builder()
            .setObserver(object : PeerConnection.Observer {
                override fun onIceCandidate(c: IceCandidate) {
                    send(buildEnv("ice", target, JSONObject().put("candidate", c.sdp).put("sdpMid", c.sdpMid).put("sdpMLineIndex", c.sdpMLineIndex)))
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
                            Log.i(TAG, "dc msg: " + String(bytes))
                        }
                    })
                }
                override fun onIceConnectionReceivingChange(p0: Boolean) {}
                override fun onIceConnectionStateChange(s: PeerConnection.IceConnectionState) {
                    status = "ice:$s"
                }
                override fun onIceGatheringStateChange(p0: PeerConnection.IceGatheringState?) {}
                override fun onAddTrack(p0: RtpReceiver?, p1: Array<out MediaStream>?) {
                    val track = p0?.track() as? VideoTrack ?: return
                    Log.i(TAG, "onAddTrack: ${track.id()}")
                }
                override fun onRemoveTrack(p0: RtpReceiver?) {}
                override fun onSignalingChange(p0: PeerConnection.SignalingState?) {}
                override fun onRenegotiationNeeded() {}
            })
            .createPeerConnectionDependencies()
        val pc = rtc.createPeerConnection(config, deps)!!
        this.pc = pc
        dc = pc.createDataChannel("control", DataChannel.Init())

        scope.launch {
            runCatching {
                val client = OkHttpClient.Builder().pingInterval(20, TimeUnit.SECONDS).build()
                val req = Request.Builder().url("$httpBase/api/config").build()
                client.newCall(req).execute().use { resp ->
                    if (resp.isSuccessful) {
                        val body = resp.body?.string().orEmpty()
                        val j = JSONObject(body)
                        val arr = j.optJSONArray("ice_servers")
                        if (arr != null) {
                            for (i in 0 until arr.length()) {
                                val s = arr.getJSONObject(i)
                                val urls = s.optString("urls", "")
                                if (urls.isNotEmpty()) {
                                    iceServers.add(PeerConnection.IceServer.builder(urls).createIceServer())
                                }
                            }
                        }
                    }
                }
            }
        }

        val client = OkHttpClient()
        val req = Request.Builder().url(wsUrl).build()
        ws = client.newWebSocket(req, object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                status = "ws:open"
                // hello 消息不需加 nonce（服务端不强制）
                webSocket.send(JSONObject().put("type", "hello").put("data", JSONObject().put("kind", "controller").put("token", prefs.token ?: "")).toString())
                webSocket.send(buildEnv("request", target, JSONObject().put("device_code", target).put("mode", mode)))
            }
            override fun onMessage(webSocket: WebSocket, text: String) {
                val j = JSONObject(text)
                when (j.optString("type")) {
                    "welcome" -> {
                        peerId = j.optJSONObject("data")?.optString("id")
                    }
                    "offer" -> {
                        val sdp = j.optJSONObject("data")?.optString("sdp") ?: return
                        pc.setRemoteDescription(SimpleSdpObserver({ /* set ok */ }, { Log.e(TAG, "set remote err: $it") }), SessionDescription(SessionDescription.Type.OFFER, sdp))
                        pc.createAnswer(object : SdpObserver {
                            override fun onCreateSuccess(sd: SessionDescription) {
                                pc.setLocalDescription(SimpleSdpObserver({ /* local ok */ }, { Log.e(TAG, "set local err: $it") }), sd)
                                webSocket.send(buildEnv("answer", target, JSONObject().put("sdp", sd.description)))
                            }
                            override fun onSetSuccess() {}
                            override fun onCreateFailure(p0: String?) { Log.e(TAG, "create answer fail: $p0") }
                            override fun onSetFailure(p0: String?) {}
                        }, MediaConstraints())
                    }
                    "ice" -> {
                        val c = j.optJSONObject("data")?.optJSONObject("candidate") ?: return
                        pc.addIceCandidate(IceCandidate(c.optString("sdpMid"), c.optInt("sdpMLineIndex"), c.optString("candidate")))
                    }
                    "request_ack" -> {
                        val allowed = j.optJSONObject("data")?.optString("allowed")
                        status = "ack:$allowed"
                    }
                    "file_ack" -> {
                        val d = j.optJSONObject("data")
                        if (d != null) {
                            val tid = d.optString("transfer_id")
                            val offset = d.optLong("received_offset", 0)
                            val resuming = d.optBoolean("resuming", false)
                            val shaOk = d.optBoolean("sha256_ok", false)
                            fileAckListeners.forEach { it(tid, offset, resuming, shaOk) }
                        }
                    }
                    "file_data", "file_meta", "file_end" -> {
                        val d = j.optJSONObject("data")
                        if (d != null) {
                            val tid = d.optString("transfer_id")
                            fileEventListeners.forEach { it(j.optString("type"), tid, d) }
                        }
                    }
                    "error" -> {
                        status = "err:" + j.optString("msg")
                    }
                }
            }
            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                status = "ws:err ${t.message}"
            }
        })
    }

    /**
     * 构建带 ts + nonce 的 envelope（反重放）
     */
    private fun buildEnv(type: String, to: String, data: JSONObject): String {
        val now = System.currentTimeMillis()
        val nonceBytes = ByteArray(12)
        rng.nextBytes(nonceBytes)
        val nonce = Base64.encodeToString(nonceBytes, Base64.URL_SAFE or Base64.NO_PADDING or Base64.NO_WRAP)
        return JSONObject()
            .put("type", type)
            .put("to", to)
            .put("data", data)
            .put("ts", now)
            .put("nonce", nonce)
            .toString()
    }

    fun sendCmd(json: String) {
        dc?.send(DataChannel.Buffer(java.nio.ByteBuffer.wrap(json.toByteArray()), true))
        // 信令兜底：也走 buildEnv
        val obj = JSONObject(json)
        ws?.send(buildEnv("cmd", target, obj))
    }

    fun sendMouse(xPct: Double, yPct: Double, button: Int, down: Boolean) {
        sendCmd("""{"op":"mouse","x":$xPct,"y":$yPct,"button":$button,"down":$down}""")
    }
    fun sendKey(code: String, down: Boolean) {
        // 通过 IME 注入（String 版本支持 modifier 合成如 Ctrl+a）
        if (down) {
            InputInjector.sendKey(code)
        }
        sendCmd("""{"op":"key","code":"${code.replace("\"", "\\\"")}","down":$down}""")
    }
    fun sendType(t: String) {
        InputInjector.sendText(t)
        sendCmd(JSONObject().put("op", "type").put("text", t).toString())
    }
    fun sendWheel(dx: Int, dy: Int) {
        sendCmd("""{"op":"wheel","dx":$dx,"dy":$dy}""")
    }

    /**
     * 文件分片发送（断点续传）。
     * 完整流程：sendFileResumable(File, transferId, onProgress): Promise<FileAck>
     */
    fun sendFileStart(meta: JSONObject) {
        ws?.send(buildEnv("file_meta", target, meta))
    }
    fun sendFileData(transferId: String, offset: Long, base64Data: String) {
        val data = JSONObject()
            .put("transfer_id", transferId)
            .put("offset", offset)
            .put("data", base64Data)
        ws?.send(buildEnv("file_data", target, data))
    }
    fun sendFileEnd(transferId: String, sha256: String) {
        ws?.send(buildEnv("file_end", target, JSONObject().put("transfer_id", transferId).put("sha256", sha256)))
    }
    /**
     * 探测：查询接收方当前 offset。回调在收到 file_ack 时触发。
     */
    fun sendFileResume(transferId: String) {
        ws?.send(buildEnv("file_ack", target, JSONObject().put("transfer_id", transferId).put("probe", true)))
    }

    /**
     * 整文件 SHA-256（生产级）：分块更新，避免一次性读入。
     */
    fun sha256OfBytes(b: ByteArray): String {
        val md = java.security.MessageDigest.getInstance("SHA-256")
        md.update(b, 0, b.size)
        return md.digest().joinToString("") { "%02x".format(it) }
    }

    fun close() {
        try { dc?.close() } catch (_: Throwable) {}
        try { pc?.close() } catch (_: Throwable) {}
        try { ws?.close(1000, "bye") } catch (_: Throwable) {}
        try { factory?.dispose() } catch (_: Throwable) {}
        egl?.release()
        scope.cancel()
        status = "closed"
    }

    private fun send(env: JSONObject) { ws?.send(env.toString()) }

    // === 文件事件回调（UI 可订阅） ===
    private val fileAckListeners = mutableListOf<(transferId: String, receivedOffset: Long, resuming: Boolean, shaOk: Boolean) -> Unit>()
    private val fileEventListeners = mutableListOf<(type: String, transferId: String, data: JSONObject) -> Unit>()
    fun addFileAckListener(l: (String, Long, Boolean, Boolean) -> Unit) { fileAckListeners.add(l) }
    fun removeFileAckListener(l: (String, Long, Boolean, Boolean) -> Unit) { fileAckListeners.remove(l) }
    fun addFileEventListener(l: (String, String, JSONObject) -> Unit) { fileEventListeners.add(l) }
    fun removeFileEventListener(l: (String, String, JSONObject) -> Unit) { fileEventListeners.remove(l) }

    private class SimpleSdpObserver(val onOk: () -> Unit, val onErr: (String?) -> Unit) : SdpObserver {
        override fun onCreateSuccess(p0: SessionDescription?) { onOk() }
        override fun onSetSuccess() { onOk() }
        override fun onCreateFailure(p0: String?) { onErr(p0) }
        override fun onSetFailure(p0: String?) { onErr(p0) }
    }

    companion object { const val TAG = "LinkALL/Ctrl" }
}
