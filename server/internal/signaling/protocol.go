// 信令消息结构 + 反重放字段
// 客户端 -> 服务端：{ "type": "...", "from": "...", "to": "...", "data": {...}, "ts": <ms>, "nonce": "<16+字符>" }
// 服务端 -> 客户端：{ "type": "...", "from": "...", "to": "...", "data": {...}, "ts": <ms>, "msg": "..." }

package signaling

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"sync"
	"time"

	"github.com/linkall/server/internal/db"
)

type Envelope struct {
	Type  string          `json:"type"`
	From  string          `json:"from,omitempty"`
	To    string          `json:"to,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
	Ts    int64           `json:"ts,omitempty"`
	Nonce string          `json:"nonce,omitempty"`
	Msg   string          `json:"msg,omitempty"`
}

const (
	MsgHello   = "hello"
	MsgWelcome = "welcome"

	MsgOffer  = "offer"
	MsgAnswer = "answer"
	MsgIce    = "ice"

	MsgRequest        = "request"
	MsgRequestAck     = "request_ack"
	MsgConnectionInfo = "connection_info"

	MsgCmd = "cmd"

	MsgPing = "ping"
	MsgPong = "pong"

	// 文件传输 v2：支持断点续传
	MsgFileMeta = "file_meta" // 初始化：{ transfer_id, name, size, sha256, chunk_size }
	MsgFileAck  = "file_ack"  // 接收方 ack：{ transfer_id, received_offset, accepted }
	MsgFileData = "file_data" // { transfer_id, offset, data(base64) }
	MsgFileEnd  = "file_end"  // { transfer_id, sha256 }

	MsgError  = "error"
	MsgClosed = "closed"
	MsgOnline = "online"
)

// NonceStore 内存 + DB 混合 nonce 池；周期清理
type NonceStore struct {
	mu     sync.Mutex
	seen   map[string]int64 // nonce -> expire_at
	stopCh chan struct{}
}

var Nonces = &NonceStore{seen: map[string]int64{}, stopCh: make(chan struct{})}

func (n *NonceStore) Start() { go n.gc() }

func (n *NonceStore) Stop() { close(n.stopCh) }

func (n *NonceStore) Add(nonce string, ttlSec int) error {
	if nonce == "" {
		return nil
	}
	expire := time.Now().Add(time.Duration(ttlSec) * time.Second).Unix()
	n.mu.Lock()
	n.seen[nonce] = expire
	n.mu.Unlock()
	_, err := db.DB.Exec(
		`INSERT OR IGNORE INTO ws_nonces(nonce, expire_at) VALUES(?,?)`,
		nonce, expire,
	)
	return err
}

func (n *NonceStore) Has(nonce string) bool {
	if nonce == "" {
		return false
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if exp, ok := n.seen[nonce]; ok && exp > time.Now().Unix() {
		return true
	}
	delete(n.seen, nonce)
	return false
}

func (n *NonceStore) gc() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-n.stopCh:
			return
		case <-t.C:
			now := time.Now().Unix()
			n.mu.Lock()
			for k, v := range n.seen {
				if v < now {
					delete(n.seen, k)
				}
			}
			n.mu.Unlock()
			_, _ = db.DB.Exec(`DELETE FROM ws_nonces WHERE expire_at < ?`, now)
		}
	}
}

func newNonce() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

var _ = sql.ErrNoRows
