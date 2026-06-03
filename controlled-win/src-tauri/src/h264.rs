// H.264 软编码（openh264）
// 接收 BGRA 帧 -> 转 I420 -> 编码 H.264 NALUs -> 输出 AVCC 格式
//
// 关于 webrtc-rs 期望的 H.264 数据格式：
//   - 当前实现是直接把整个 access unit 塞进 Sample
//   - webrtc-rs 的 H264Packetizer 内部会自动按 RTP 切片
//   - 关键：SDP 中应声明 packetization-mode=1（默认），数据用 AVCC（带 4 字节长度前缀）
//   - openh264 输出的是 Annex-B（00 00 00 01 起始码），需要剥离并改成 AVCC
//
// 因此我们在 Sample 中传入 length-prefixed NALU 数组（每个 NALU 前 4 字节长度），
// H264Packetizer 会正确处理。
use anyhow::Result;
use bytes::{Bytes, BytesMut, BufMut};
use openh264::encoder::{Encoder, EncoderConfig};
use openh264::formats::YUVSource;
use parking_lot::Mutex;
use std::sync::Arc;

pub struct EncodedFrame {
    pub data: Bytes,
    pub is_keyframe: bool,
    pub width: u32,
    pub height: u32,
}

pub struct H264Encoder {
    enc: Mutex<Encoder>,
    width: u32,
    height: u32,
    bitrate_kbps: u32,
    fps: u32,
}

struct I420Frame<'a> {
    w: i32,
    h: i32,
    y: &'a [u8],
    u: &'a [u8],
    v: &'a [u8],
}

impl<'a> YUVSource for I420Frame<'a> {
    fn width(&self) -> i32 { self.w }
    fn height(&self) -> i32 { self.h }
    fn y(&self) -> &[u8] { self.y }
    fn u(&self) -> &[u8] { self.u }
    fn v(&self) -> &[u8] { self.v }
    fn y_stride(&self) -> i32 { self.w }
    fn u_stride(&self) -> i32 { self.w / 2 }
    fn v_stride(&self) -> i32 { self.w / 2 }
}

impl H264Encoder {
    pub fn new(width: u32, height: u32, _bitrate_kbps: u32, _fps: u32) -> Result<Arc<Self>> {
        let config = EncoderConfig {
            width,
            height,
        };
        let enc = Encoder::with_config(config)?;
        Ok(Arc::new(Self {
            enc: Mutex::new(enc),
            width,
            height,
            bitrate_kbps,
            fps,
        }))
    }

    pub fn reconfig(&self, _bitrate_kbps: u32, _fps: u32) -> Result<()> {
        // openh264 0.3.2 EncoderConfig only supports width/height;
        // bitrate/framerate are auto-configured.  Drop & recreate is the only path.
        let mut enc = self.enc.lock();
        let config = EncoderConfig {
            width: self.width,
            height: self.height,
        };
        *enc = Encoder::with_config(config)?;
        Ok(())
    }

    /// encode 接收 BGRA（与 scrap 输出一致），输出 AVCC 格式
    pub fn encode_bgra(&self, bgra: &[u8], w: u32, h: u32) -> Result<Vec<EncodedFrame>> {
        let yuv = bgra_to_i420(bgra, w, h);
        let mut enc = self.enc.lock();
        let frame = build_oh_frame(&yuv, w, h);
        let bitstream = enc.encode(&frame)?;
        let raw = bitstream.as_ref().to_vec();
        let nals = split_annexb(&raw);
        let mut avcc = BytesMut::new();
        for n in nals {
            if n.len() < 3 { continue; }
            avcc.put_u32(n.len() as u32);
            avcc.extend_from_slice(&n);
        }
        let first_nal_type = avcc.as_ref().get(4).map(|b| b & 0x1f).unwrap_or(0);
        let is_key = first_nal_type == 5;
        Ok(vec![EncodedFrame {
            data: avcc.freeze(),
            is_keyframe: is_key,
            width: w,
            height: h,
        }])
    }
}

// BGRA -> I420 (BT.601 limited range)
pub fn bgra_to_i420(bgra: &[u8], w: u32, h: u32) -> Vec<u8> {
    let w = w as usize;
    let h = h as usize;
    let mut y = vec![0u8; w * h];
    let mut u = vec![0u8; (w / 2) * (h / 2)];
    let mut v = vec![0u8; (w / 2) * (h / 2)];
    for j in 0..h {
        for i in 0..w {
            let off = (j * w + i) * 4;
            let b = bgra[off] as f32;
            let g = bgra[off + 1] as f32;
            let r = bgra[off + 2] as f32;
            // BT.601 limited
            let yy = (0.257 * r + 0.504 * g + 0.098 * b + 16.0).clamp(0.0, 255.0) as u8;
            y[j * w + i] = yy;
            if (i & 1) == 0 && (j & 1) == 0 {
                let uu = (-0.148 * r - 0.291 * g + 0.439 * b + 128.0).clamp(0.0, 255.0) as u8;
                let vv = (0.439 * r - 0.368 * g - 0.071 * b + 128.0).clamp(0.0, 255.0) as u8;
                u[(j / 2) * (w / 2) + (i / 2)] = uu;
                v[(j / 2) * (w / 2) + (i / 2)] = vv;
            }
        }
    }
    let mut out = Vec::with_capacity(y.len() + u.len() + v.len());
    out.extend_from_slice(&y);
    out.extend_from_slice(&u);
    out.extend_from_slice(&v);
    out
}

fn split_annexb(annexb: &[u8]) -> Vec<Vec<u8>> {
    let mut out = Vec::new();
    let mut i = 0;
    while i < annexb.len() {
        if i + 3 < annexb.len() && annexb[i] == 0 && annexb[i + 1] == 0 && annexb[i + 2] == 1 {
            let start = i + 3;
            let mut end = annexb.len();
            for j in start..annexb.len() - 3 {
                if annexb[j] == 0 && annexb[j + 1] == 0 && annexb[j + 2] == 1 {
                    end = j;
                    break;
                }
            }
            out.push(annexb[start..end].to_vec());
            i = end;
        } else if i + 4 < annexb.len() && annexb[i] == 0 && annexb[i + 1] == 0 && annexb[i + 2] == 0 && annexb[i + 3] == 1 {
            let start = i + 4;
            let mut end = annexb.len();
            for j in start..annexb.len() - 3 {
                if annexb[j] == 0 && annexb[j + 1] == 0 && annexb[j + 2] == 1 {
                    end = j;
                    break;
                }
            }
            out.push(annexb[start..end].to_vec());
            i = end;
        } else {
            i += 1;
        }
    }
    out
}

fn build_oh_frame<'a>(i420: &'a [u8], w: u32, h: u32) -> I420Frame<'a> {
    let y_plane_len = (w * h) as usize;
    let uv_plane_len = (w * h / 4) as usize;
    let y = &i420[0..y_plane_len];
    let u = &i420[y_plane_len..y_plane_len + uv_plane_len];
    let v = &i420[y_plane_len + uv_plane_len..y_plane_len + 2 * uv_plane_len];
    I420Frame { w: w as i32, h: h as i32, y, u, v }
}
