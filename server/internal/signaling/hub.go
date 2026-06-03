package signaling

import (
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/linkall/server/internal/models"
	"github.com/linkall/server/internal/security"
)

func readCryptoRand(b []byte) (int, error) { return cryptorand.Read(b) }

type PeerKind string

const (
	PeerControlled PeerKind = "controlled"
	PeerController PeerKind = "controller"
)

type Peer struct {
	ID         string
	DeviceCode string
	Kind       PeerKind
	UserID     int64
	Conn       *websocket.Conn
	Send       chan []byte
	Hub        *Hub
	closed     atomic.Bool
	onceClose  sync.Once

	// 反重放 / 限流
	lastCmdAt  atomic.Int64
	lastFileAt atomic.Int64
}

func (p *Peer) close() {
	p.onceClose.Do(func() {
		p.closed.Store(true)
		close(p.Send)
		_ = p.Conn.Close()
		p.Hub.unregister(p)
	})
}

func (p *Peer) writePump() {
	tick := time.NewTicker(20 * time.Second)
	defer tick.Stop()
	for {
		select {
		case msg, ok := <-p.Send:
			if !ok {
				return
			}
			if err := p.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				p.close()
				return
			}
		case <-tick.C:
			if err := p.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				p.close()
				return
			}
		}
	}
}

func (p *Peer) readPump() {
	defer p.close()
	rc := security.GetRateConfig()
	p.Conn.SetReadLimit(int64(rc.WSMaxMessageKB) * 1024)
	_ = p.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	p.Conn.SetPongHandler(func(string) error {
		_ = p.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, raw, err := p.Conn.ReadMessage()
		if err != nil {
			return
		}
		_ = p.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		var env Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			p.sendError("malformed json")
			continue
		}
		env.From = p.ID
		if env.Ts == 0 {
			env.Ts = time.Now().UnixMilli()
		}
		// 反重放 / 时间窗
		if env.Type != MsgHello && env.Type != MsgPing {
			if !p.validateReplay(&env, rc) {
				p.sendError("replay rejected (ts/nonce)")
				continue
			}
		}
		p.Hub.handle(p, &env)
	}
}

// validateReplay 校验：ts 在窗口内 + nonce 未重复
func (p *Peer) validateReplay(env *Envelope, rc security.RateConfig) bool {
	now := time.Now().UnixMilli()
	winMs := int64(rc.ReplayWindowSec) * 1000
	if env.Ts == 0 || abs(now-env.Ts) > winMs {
		return false
	}
	if env.Nonce == "" {
		// 没有 nonce：限流（每秒消息数）
		return p.rateLimitCheck(env, rc)
	}
	if Nonces.Has(env.Nonce) {
		return false
	}
	_ = Nonces.Add(env.Nonce, rc.ReplayWindowSec*2)
	return p.rateLimitCheck(env, rc)
}

func (p *Peer) rateLimitCheck(env *Envelope, rc security.RateConfig) bool {
	lim := Limiter()
	now := time.Now().UnixNano()
	switch env.Type {
	case MsgCmd, MsgOffer, MsgAnswer, MsgIce, MsgRequest, MsgRequestAck, MsgConnectionInfo:
		// 用秒级令牌桶
		key := p.ID
		if p.Kind == PeerControlled {
			key = "controlled:" + p.DeviceCode
		} else {
			key = "controller:" + p.ID
		}
		ok, _ := lim.Allow("ws_cmd", key)
		_ = now
		return ok
	case MsgFileMeta, MsgFileData, MsgFileAck, MsgFileEnd:
		key := p.ID
		if p.Kind == PeerControlled {
			key = "controlled:" + p.DeviceCode
		} else {
			key = "controller:" + p.ID
		}
		ok, _ := lim.Allow("ws_file", key)
		return ok
	}
	return true
}

func (p *Peer) sendError(msg string) {
	b, _ := json.Marshal(Envelope{Type: MsgError, Msg: msg, Ts: time.Now().UnixMilli()})
	select {
	case p.Send <- b:
	default:
	}
}

func (p *Peer) send(env *Envelope) {
	if p.closed.Load() {
		return
	}
	if env.Ts == 0 {
		env.Ts = time.Now().UnixMilli()
	}
	b, err := json.Marshal(env)
	if err != nil {
		return
	}
	select {
	case p.Send <- b:
	case <-time.After(2 * time.Second):
		log.Printf("[ws] send timeout peer=%s", p.ID)
	}
}

func (p *Peer) sendJSON(typ string, data interface{}) {
	raw, _ := json.Marshal(data)
	p.send(&Envelope{Type: typ, Data: raw})
}

// ===== Hub =====

type Hub struct {
	mu        sync.RWMutex
	peers     map[string]*Peer
	byCode    map[string]*Peer
	OnHello   func(p *Peer, kind PeerKind, code, token string) error
	OnCmd     func(from *Peer, env *Envelope)
	OnFile    func(from *Peer, env *Envelope)
}

func NewHub() *Hub {
	return &Hub{
		peers:  map[string]*Peer{},
		byCode: map[string]*Peer{},
	}
}

func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.peers)
}

func (h *Hub) OnlineByCode(code string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.byCode[strings.ToUpper(code)]
	return ok
}

func (h *Hub) register(p *Peer) {
	h.mu.Lock()
	h.peers[p.ID] = p
	if p.Kind == PeerControlled && p.DeviceCode != "" {
		h.byCode[p.DeviceCode] = p
	}
	h.mu.Unlock()
}

func (h *Hub) unregister(p *Peer) {
	h.mu.Lock()
	delete(h.peers, p.ID)
	if p.Kind == PeerControlled && p.DeviceCode != "" {
		if cur, ok := h.byCode[p.DeviceCode]; ok && cur.ID == p.ID {
			delete(h.byCode, p.DeviceCode)
		}
	}
	h.mu.Unlock()
}

func (h *Hub) findByCode(code string) *Peer {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.byCode[strings.ToUpper(code)]
}

// FindByCode 公开查找，给 admin handler 用
func (h *Hub) FindByCode(code string) *Peer { return h.findByCode(code) }

func (p *Peer) SendEnvelope(env *Envelope) { p.send(env) }

func (h *Hub) find(id string) *Peer {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.peers[id]
}

func (h *Hub) Broadcast(env *Envelope) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, p := range h.peers {
		p.send(env)
	}
}

func (h *Hub) handle(p *Peer, env *Envelope) {
	switch env.Type {
	case MsgHello:
		var d struct {
			Kind       PeerKind `json:"kind"`
			DeviceCode string   `json:"device_code"`
			Token      string   `json:"token"`
			UserID     int64    `json:"user_id"`
		}
		if err := json.Unmarshal(env.Data, &d); err != nil {
			p.sendError("invalid hello")
			return
		}
		if h.OnHello != nil {
			if err := h.OnHello(p, d.Kind, d.DeviceCode, d.Token); err != nil {
				p.sendError(err.Error())
				p.close()
				return
			}
		}
		p.Kind = d.Kind
		p.DeviceCode = strings.ToUpper(d.DeviceCode)
		p.UserID = d.UserID
		h.register(p)
		ack, _ := json.Marshal(map[string]any{"id": p.ID, "kind": p.Kind})
		p.send(&Envelope{Type: MsgWelcome, Data: ack})
		log.Printf("[ws] register id=%s kind=%s code=%s", p.ID, p.Kind, p.DeviceCode)

	case MsgPing:
		p.send(&Envelope{Type: MsgPong, Ts: time.Now().UnixMilli()})

	case MsgOffer, MsgAnswer, MsgIce, MsgCmd, MsgRequest, MsgRequestAck, MsgFileMeta, MsgFileAck, MsgFileData, MsgFileEnd:
		if env.To == "" {
			p.sendError("missing 'to' field")
			return
		}
		target := h.find(env.To)
		if target == nil {
			if p2 := h.findByCode(env.To); p2 != nil {
				target = p2
			}
		}
		if target == nil {
			p.sendError("target offline")
			return
		}
		if env.Type == MsgCmd && h.OnCmd != nil {
			h.OnCmd(p, env)
		}
		if env.Type == MsgFileMeta || env.Type == MsgFileAck || env.Type == MsgFileData || env.Type == MsgFileEnd {
			h.handleFile(p, target, env)
			return
		}
		target.send(env)

	default:
		p.sendError("unknown message type: " + env.Type)
	}
}

// handleFile 文件分片服务器中继：只做转发，但记录进度便于断点续传
func (h *Hub) handleFile(from, to *Peer, env *Envelope) {
	switch env.Type {
	case MsgFileMeta:
		var d struct {
			TransferID string `json:"transfer_id"`
			Name       string `json:"name"`
			Size       int64  `json:"size"`
			SHA256     string `json:"sha256"`
			ChunkSize  int64  `json:"chunk_size"`
		}
		_ = json.Unmarshal(env.Data, &d)
		// 创建 file_transfer 记录
		ft := &models.FileTransfer{
			ID:             newUUID(),
			Direction:      "c2h",
			TransferID:     d.TransferID,
			ControllerID:   from.ID,
			ControlledCode: to.DeviceCode,
			Name:           d.Name,
			Size:           d.Size,
			SHA256Expected: d.SHA256,
			ChunkSize:      d.ChunkSize,
			Status:         "open",
		}
		if from.Kind == PeerControlled {
			ft.Direction = "h2c"
		}
		// 复用：检查是否已存在（基于 transfer_id 续传）
		old, _ := models.GetFileTransferByTID(d.TransferID)
		if old != nil {
			ft.ID = old.ID
			ft.ReceivedOffset = old.ReceivedOffset
			_ = models.UpdateFileTransferProgress(old.ID, old.ReceivedOffset, "open")
			// 通知接收方从 ft.ReceivedOffset 续传
			to.sendJSON(MsgFileAck, map[string]any{
				"transfer_id":     d.TransferID,
				"received_offset": old.ReceivedOffset,
				"accepted":        true,
				"resuming":        true,
			})
			return
		}
		_ = models.CreateFileTransfer(ft)
		// 告知接收方
		to.sendJSON(MsgFileMeta, map[string]any{
			"transfer_id": d.TransferID,
			"name":        d.Name,
			"size":        d.Size,
			"sha256":      d.SHA256,
			"chunk_size":  d.ChunkSize,
		})
		return
	case MsgFileAck:
		// 接收方已收到一段，转发给发送方
		to.send(env)
		return
	case MsgFileData:
		// 收到一段，更新进度
		var d struct {
			TransferID string `json:"transfer_id"`
			Offset     int64  `json:"offset"`
		}
		_ = json.Unmarshal(env.Data, &d)
		ft, _ := models.GetFileTransferByTID(d.TransferID)
		if ft != nil {
			// 进度 = offset + 本段实际长度
			progress := d.Offset + int64(len(env.Data))
			_ = models.UpdateFileTransferProgress(ft.ID, progress, "open")
		}
		to.send(env)
		return
	case MsgFileEnd:
		var d struct {
			TransferID string `json:"transfer_id"`
		}
		_ = json.Unmarshal(env.Data, &d)
		ft, _ := models.GetFileTransferByTID(d.TransferID)
		if ft != nil {
			_ = models.CompleteFileTransfer(ft.ID)
		}
		to.send(env)
		return
	}
}

// Start 启动 peer 的读写循环（已通过 websocket.Upgrade 升级）
func (h *Hub) Start(conn *websocket.Conn) error {
	// WebSocket 连接限流（按 IP）
	lim := Limiter()
	ip := conn.IP()
	if ip != "" {
		ok, _ := lim.Allow("ws_connect", ip)
		if !ok {
			return errors.New("ws connect rate limit")
		}
	}
	p := &Peer{
		ID:   newUUID(),
		Conn: conn,
		Send: make(chan []byte, 64),
		Hub:  h,
	}
	go p.writePump()
	p.readPump()
	return nil
}

func randomID() string {
	return newUUID()
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = readCryptoRand(b)
	return formatUUID(b)
}

func formatUUID(b []byte) string {
	// 8-4-4-4-12
	if len(b) != 16 {
		return ""
	}
	h := hexEncode(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func hexEncode(b []byte) string {
	const alpha = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = alpha[v>>4]
		out[i*2+1] = alpha[v&0x0f]
	}
	return string(out)
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// Limiter 单例
var limiterOnce sync.Once
var limiterInst *security.Limiter

func Limiter() *security.Limiter {
	limiterOnce.Do(func() {
		limiterInst = security.NewLimiter()
		rc := security.LoadRateConfig()
		limiterInst.SetRule("login", rc.LoginPerWindow, rc.LoginWindowSec)
		limiterInst.SetRule("register", rc.RegisterPerWin, rc.RegisterWinSec)
		limiterInst.SetRule("device_register", rc.DevRegPerWin, rc.DevRegWinSec)
		limiterInst.SetRule("ws_connect", rc.WSConnectPerWin, rc.WSConnectWinSec)
		limiterInst.SetRule("ws_cmd", rc.WSCmdPerSec, 1)
		limiterInst.SetRule("ws_file", rc.WSFilePerSec, 1)
	})
	return limiterInst
}

func ResetLimiter() {
	limiterOnce = sync.Once{}
	limiterInst = nil
}
