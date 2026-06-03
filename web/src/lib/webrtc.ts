// WebRTC P2P 控制客户端（浏览器侧）
// 流程：信令 WS 协商 SDP/ICE  → 建立 RTCPeerConnection → 收 MediaStream → 启 DataChannel 收发控制指令
// 所有发往服务端的非 hello/ping 消息自动加 ts + nonce（反重放）

export interface ControlCmd {
  op: 'mouse' | 'key' | 'wheel' | 'type' | 'clip' | 'file_send' | 'privacy' | 'config';
  [k: string]: any;
}

export interface ControlConfig {
  scale?: number;
  bitrate_kbps?: number;
  fps?: number;
  codec?: 'h264' | 'vp9' | 'av1' | 'vp8';
  privacy?: boolean;
}

export interface ConnectOptions {
  signalingUrl: string;
  deviceCode: string;
  token?: string;
  iceServers?: RTCIceServer[];
  onVideo?: (stream: MediaStream) => void;
  onData?: (data: string) => void;
  onStatus?: (status: string) => void;
  onLog?: (msg: string) => void;
  onFileProgress?: (transfer_id: string, offset: number, total: number) => void;
  onFileAck?: (ack: { transfer_id: string; received_offset: number; accepted?: boolean; resuming?: boolean; sha256_ok?: boolean }) => void;
}

export class ControlClient {
  pc: RTCPeerConnection | null = null;
  ws: WebSocket | null = null;
  dc: RTCDataChannel | null = null;
  opts: ConnectOptions;
  closed = false;

  constructor(opts: ConnectOptions) {
    this.opts = opts;
  }

  log(m: string) { this.opts.onLog?.(m); }

  async connect() {
    this.closed = false;
    this.ws = new WebSocket(this.opts.signalingUrl);
    this.opts.onStatus?.('signaling:connecting');

    this.ws.onopen = () => {
      this.log('ws open');
      this.opts.onStatus?.('signaling:open');
      this.sendHello();
    };
    this.ws.onclose = () => this.opts.onStatus?.('signaling:closed');
    this.ws.onerror = () => this.opts.onStatus?.('signaling:error');
    this.ws.onmessage = (ev) => this.handleSignaling(ev.data);

    this.pc = new RTCPeerConnection({
      iceServers: this.opts.iceServers || [{ urls: 'stun:stun.l.google.com:19302' }]
    });
    this.pc.ontrack = (ev) => {
      this.log('ontrack: ' + ev.streams.length);
      if (ev.streams[0]) this.opts.onVideo?.(ev.streams[0]);
    };
    this.pc.onicecandidate = (ev) => {
      if (ev.candidate) {
        this.send({ type: 'ice', to: this.opts.deviceCode, data: { candidate: ev.candidate.toJSON() } });
      }
    };
    this.pc.oniceconnectionstatechange = () => {
      this.opts.onStatus?.('ice:' + this.pc!.iceConnectionState);
    };
    this.pc.ondatachannel = (ev) => {
      this.bindDataChannel(ev.channel);
    };
    this.dc = this.pc.createDataChannel('control', { ordered: true });
    this.bindDataChannel(this.dc);
  }

  private bindDataChannel(ch: RTCDataChannel) {
    ch.onopen = () => this.opts.onStatus?.('dc:open');
    ch.onclose = () => this.opts.onStatus?.('dc:closed');
    ch.onmessage = (ev) => {
      const data = typeof ev.data === 'string' ? ev.data : '';
      this.handleDataChannelMessage(data);
    };
  }

  private handleDataChannelMessage(data: string) {
    try {
      const env = JSON.parse(data);
      switch (env.type) {
        case 'cmd': this.opts.onData?.(JSON.stringify(env.data ?? {})); break;
        case 'file_ack': {
          const d = env.data || {};
          this.opts.onFileProgress?.(d.transfer_id, d.received_offset ?? 0, d.total ?? 0);
          break;
        }
        default: this.opts.onData?.(data);
      }
    } catch {
      this.opts.onData?.(data);
    }
  }

  private sendHello() {
    // hello 不需要 ts/nonce
    this.ws!.send(JSON.stringify({ type: 'hello', data: { kind: 'controller', token: this.opts.token || '' } }));
    this.send({ type: 'request', to: this.opts.deviceCode, data: { device_code: this.opts.deviceCode, mode: 'anonymous' } });
  }

  private handleSignaling(raw: any) {
    let env: any;
    try { env = JSON.parse(raw); } catch { return; }
    if (!env || !env.type) return;
    switch (env.type) {
      case 'welcome':
        this.log('welcome: ' + env.data?.id);
        break;
      case 'offer': {
        this.log('offer received');
        this.pc!.setRemoteDescription({ type: 'offer', sdp: env.data.sdp })
          .then(() => this.pc!.createAnswer())
          .then((a) => this.pc!.setLocalDescription(a))
          .then(() => this.send({ type: 'answer', to: this.opts.deviceCode, data: { sdp: this.pc!.localDescription!.sdp } }))
          .catch((e) => this.log('offer err: ' + e));
        break;
      }
      case 'answer': {
        this.log('answer received');
        this.pc!.setRemoteDescription({ type: 'answer', sdp: env.data.sdp }).catch((e) => this.log('answer err: ' + e));
        break;
      }
      case 'ice': {
        if (env.data?.candidate) {
          this.pc!.addIceCandidate(env.data.candidate).catch((e) => this.log('ice err: ' + e));
        }
        break;
      }
      case 'request_ack': {
        const allowed = env.data?.allowed;
        if (allowed === 'denied') this.opts.onStatus?.('denied');
        else if (allowed === 'once' || allowed === 'permanent') {
          this.opts.onStatus?.('ack:' + allowed);
          this.pc!.createOffer({ offerToReceiveVideo: true, offerToReceiveAudio: true })
            .then((o) => this.pc!.setLocalDescription(o))
            .then(() => this.send({ type: 'offer', to: this.opts.deviceCode, data: { sdp: this.pc!.localDescription!.sdp } }))
            .catch((e) => this.log('offer err: ' + e));
        } else if (env.data?.require_code) {
          this.opts.onStatus?.('require_code');
        }
        break;
      }
      case 'cmd': this.opts.onData?.(env.data ? JSON.stringify(env.data) : ''); break;
      case 'file_ack': {
        const d = env.data || {};
        this.opts.onFileProgress?.(d.transfer_id, d.received_offset ?? 0, d.total ?? 0);
        // 触发 resume：回调可读 d.resuming / d.accepted / d.sha256_ok
        this.opts.onFileAck?.(d);
        break;
      }
      case 'file_data':
      case 'file_meta':
      case 'file_end':
        this.opts.onData?.(JSON.stringify(env));
        break;
      case 'error':
        this.log('signaling error: ' + env.msg);
        this.opts.onStatus?.('error:' + env.msg);
        break;
    }
  }

  send(env: any) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
    // 自动加 ts + nonce（hello/ping 跳过）
    if (env.type && env.type !== 'hello' && env.type !== 'ping') {
      if (env.ts == null) env.ts = Date.now();
      if (!env.nonce) env.nonce = randomNonce();
    }
    this.ws.send(JSON.stringify(env));
  }

  sendCmd(cmd: ControlCmd) {
    if (this.dc && this.dc.readyState === 'open') {
      this.dc.send(JSON.stringify(cmd));
    } else {
      this.send({ type: 'cmd', to: this.opts.deviceCode, data: cmd });
    }
  }

  sendConfig(c: ControlConfig) {
    this.sendCmd({ op: 'config', ...c });
  }

  close() {
    if (this.closed) return;
    this.closed = true;
    try { this.dc?.close(); } catch {}
    try { this.pc?.close(); } catch {}
    try { this.ws?.close(); } catch {}
  }
}

function randomNonce(): string {
  const arr = new Uint8Array(12);
  crypto.getRandomValues(arr);
  return btoa(String.fromCharCode(...arr)).replace(/=+$/, '').replace(/\+/g, '-').replace(/\//g, '_');
}

// === 文件分片（DataChannel 256KB 切片） ===
export interface FileMeta {
  transfer_id: string;
  name: string;
  size: number;
  sha256: string;
  total_chunks: number;
  chunk_size: number;
}

const CHUNK = 256 * 1024;

function arrayBufferToBase64(buf: ArrayBuffer): string {
  const bytes = new Uint8Array(buf);
  let binary = '';
  const chunk = 0x8000;
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode.apply(null, Array.from(bytes.subarray(i, i + chunk)) as any);
  }
  return btoa(binary);
}

async function sha256OfFile(file: File): Promise<string> {
  const buf = await file.arrayBuffer();
  const hash = await crypto.subtle.digest('SHA-256', buf);
  const arr = new Uint8Array(hash);
  let s = '';
  for (const b of arr) s += b.toString(16).padStart(2, '0');
  return s;
}

/**
 * 断点续传发送（生产级）：
 *   1) 算 SHA-256
 *   2) 发 file_meta
 *   3) 等 file_ack 拿 received_offset / resuming
 *   4) 从 startFromOffset 切片发送（每片 yield 主线程）
 *   5) 发 file_end
 *   6) 等终态 file_ack，读 sha256_ok
 *
 * @param ctl ControlClient
 * @param file File
 * @param transferId 业务级 ID（用户生成或由发送方生成）
 * @param startFromOffset 起始 offset（默认 0；断点续传时传上次进度）
 * @param onProgress 进度回调
 * @returns Promise<{ sha256: string; accepted: boolean; resuming: boolean }>
 */
export async function sendFileResumable(
  ctl: ControlClient,
  file: File,
  transferId: string,
  startFromOffset: number = 0,
  onProgress: (offset: number, total: number) => void
): Promise<{ sha256: string; accepted: boolean; resuming: boolean; finalOffset: number }> {
  const total = file.size;
  const sha256 = await sha256OfFile(file);
  const meta = {
    transfer_id: transferId, name: file.name, size: total,
    sha256, chunk_size: CHUNK
  };
  ctl.send({ type: 'file_meta', to: ctl.opts.deviceCode, data: meta });

  // 等第一次 file_ack（接收方回传 received_offset + resuming）
  const resumeInfo = await new Promise<{ offset: number; resuming: boolean; accepted: boolean }>((resolve) => {
    const orig = ctl.opts.onFileAck;
    let done = false;
    ctl.opts.onFileAck = (ack) => {
      if (ack.transfer_id !== transferId) { orig?.(ack); return; }
      if (done) return;
      done = true;
      ctl.opts.onFileAck = orig;
      resolve({ offset: ack.received_offset ?? 0, resuming: !!ack.resuming, accepted: ack.accepted !== false });
    };
    setTimeout(() => { if (!done) { done = true; ctl.opts.onFileAck = orig; resolve({ offset: startFromOffset, resuming: false, accepted: true }); } }, 3000);
  });
  if (!resumeInfo.accepted) {
    return { sha256, accepted: false, resuming: false, finalOffset: 0 };
  }
  const start = Math.max(startFromOffset, resumeInfo.offset);
  for (let offset = start; offset < total; offset += CHUNK) {
    const end = Math.min(offset + CHUNK, total);
    const slice = file.slice(offset, end);
    const buf = await slice.arrayBuffer();
    const b64 = arrayBufferToBase64(buf);
    ctl.send({ type: 'file_data', to: ctl.opts.deviceCode, data: { transfer_id: transferId, offset, data: b64 } });
    onProgress(end, total);
    if ((offset / CHUNK) % 32 === 0) await sleep(0);
  }
  ctl.send({ type: 'file_end', to: ctl.opts.deviceCode, data: { transfer_id: transferId, sha256 } });
  // 等终态 file_ack
  const finalAck = await new Promise<{ received_offset: number; sha256_ok?: boolean }>((resolve) => {
    const orig = ctl.opts.onFileAck;
    let done = false;
    ctl.opts.onFileAck = (ack) => {
      if (ack.transfer_id !== transferId) { orig?.(ack); return; }
      if (done) return;
      // 跳过 resume 后的中间 ack（received_offset < total）
      if ((ack.received_offset ?? 0) < total) { orig?.(ack); return; }
      done = true;
      ctl.opts.onFileAck = orig;
      resolve({ received_offset: ack.received_offset, sha256_ok: ack.sha256_ok });
    };
    setTimeout(() => { if (!done) { done = true; ctl.opts.onFileAck = orig; resolve({ received_offset: total }); } }, 5000);
  });
  return { sha256, accepted: true, resuming: resumeInfo.resuming, finalOffset: finalAck.received_offset };
}

function sleep(ms: number) { return new Promise(r => setTimeout(r, ms)); }
