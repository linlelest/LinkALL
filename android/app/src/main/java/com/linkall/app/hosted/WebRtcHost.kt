package com.linkall.app.hosted

import android.util.Log
import org.webrtc.DataChannel
import org.webrtc.IceCandidate
import org.webrtc.MediaStream
import org.webrtc.PeerConnection
import org.webrtc.PeerConnectionFactory
import org.webrtc.SdpObserver
import org.webrtc.SessionDescription
import org.webrtc.VideoCapturer
import org.webrtc.VideoSource
import org.webrtc.VideoTrack
import org.webrtc.CapturerObserver

/**
 * 被控端 WebRTC 主机：维护 PeerConnectionFactory、PeerConnection、VideoSource/VideoTrack。
 *
 * 帧推流链路：
 *   ScreenCaptureService 抓 Image（RGBA_8888）
 *   -> Image.planes[0].buffer 转 ByteBuffer
 *   -> WebRtcHost.addFrame(width, height, rotation, yuv420Bytes)
 *   -> VideoSource.getCapturerObserver().onFrameCaptured(VideoFrame(I420Buffer))
 *
 * 真实工程中的 I420 数据需从 RGBA 通过 BT.601 limited-range 公式转换。
 */
object WebRtcHost {
    /** 公开：HostedRtcSession 等需要复用 factory 时直接拿 */
    @JvmStatic var factory: PeerConnectionFactory? = null
        private set
    private var videoSource: VideoSource? = null
    @Volatile private var running = false

    fun ensureFactory() {
        if (factory != null) return
        PeerConnectionFactory.initialize(PeerConnectionFactory.InitializationOptions.builder()
            .setEnableInternalTracer(false)
            .createInitializationOptions())
        factory = PeerConnectionFactory.builder().createPeerConnectionFactory()
        videoSource = factory!!.createVideoSource(false) // 不需要 screencast 优化
        running = true
        Log.i(TAG, "webrtc factory + video source created")
    }

    fun createVideoTrack(): VideoTrack? {
        val s = videoSource ?: return null
        val f = factory ?: return null
        val track = f.createVideoTrack("ARDAMSv0", s)
        return track
    }

    fun capturerObserver(): CapturerObserver? = videoSource?.capturerObserver

    /**
     * 推一帧（I420 格式）。
     * @param width  帧宽
     * @param height 帧高
     * @param rotation 旋转角度（0/90/180/270）
     * @param i420    YUV420 (NV12/I420) 数据
     */
    fun addFrame(width: Int, height: Int, rotation: Int, i420: java.nio.ByteBuffer) {
        val obs = capturerObserver() ?: return
        if (!running) return
        try {
            val yuv = org.webrtc.JavaI420Buffer.wrap(
                width, height,
                i420,                  // Y plane
                width,                 // Y stride
                i420.duplicate().position(width * height).slice(),  // U
                width / 2,
                i420.duplicate().position(width * height + width * height / 4).slice(), // V
                width / 2
            )
            val frame = org.webrtc.VideoFrame(yuv, rotation, /* tsNs */ System.nanoTime())
            obs.onFrameCaptured(frame)
        } catch (e: Throwable) {
            Log.w(TAG, "addFrame failed: ${e.message}")
        }
    }

    // === Frame sink API（生产级：单一注册，避免重复推） ===
    private val frameSinks = mutableListOf<org.webrtc.VideoSink>()

    /** 注册 VideoSink（替代旧 addFrameListener） */
    @Synchronized
    fun addFrameSink(sink: org.webrtc.VideoSink) {
        if (!frameSinks.contains(sink)) {
            frameSinks.add(sink)
            videoSource?.addSink(sink)
        }
    }

    @Synchronized
    fun removeFrameSink(sink: org.webrtc.VideoSink) {
        if (frameSinks.remove(sink)) {
            videoSource?.removeSink(sink)
        }
    }

    /**
     * 用 VideoSink 推一帧（推荐路径）。调用方实现 VideoSink.onFrame 拿到 VideoFrame。
     * ScreenCaptureService 应在 service start 时调一次 addFrameSink；stop 时 removeFrameSink。
     */
    fun addFrameFromCapture(width: Int, height: Int, rotation: Int, i420: java.nio.ByteBuffer) {
        try {
            val yuv = org.webrtc.JavaI420Buffer.wrap(
                width, height,
                i420,
                width,
                i420.duplicate().position(width * height).slice(),
                width / 2,
                i420.duplicate().position(width * height + width * height / 4).slice(),
                width / 2
            )
            val frame = org.webrtc.VideoFrame(yuv, rotation, System.nanoTime())
            synchronized(frameSinks) {
                for (s in frameSinks) {
                    try { s.onFrame(frame) } catch (_: Throwable) {}
                }
            }
        } catch (e: Throwable) {
            Log.w(TAG, "addFrameFromCapture failed: ${e.message}")
        }
    }

    fun stop() {
        running = false
        try { videoSource?.dispose() } catch (_: Throwable) {}
        try { factory?.dispose() } catch (_: Throwable) {}
        videoSource = null
        factory = null
    }

    class NoopSdpObserver : SdpObserver {
        override fun onCreateSuccess(p0: SessionDescription?) {}
        override fun onSetSuccess() {}
        override fun onCreateFailure(p0: String?) {}
        override fun onSetFailure(p0: String?) {}
    }

    const val TAG = "LinkALL/WebRtcHost"
}
