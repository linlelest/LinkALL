// 推送 & 唤醒（FCM HTTP v1 + WoL 魔术包）
// FCM 流程：
//   1. Android 端拿 FCM token，调 POST /api/devices/fcm-token
//   2. 服务端存 fcm_tokens
//   3. Admin 触发 wake 时：先查用户设备是否在线；离线才发 FCM 推送（Android 端收到开 HostedService 走 WebRTC）
//   4. Windows 端无 FCM，靠 WoL 魔术包（如果用户在 BIOS/网卡开了 Wake-on-LAN）+ WebRTC 永续 WS 重连
// WoL 流程：
//   - Admin 调 POST /api/admin/wake
//   - 服务端取设备 IP + MAC，发魔术包到 255.255.255.255:9
package push

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// === FCM ===

// FCMClient 调 FCM HTTP v1 API 推送
// 文档：https://firebase.google.com/docs/reference/fcm/rest
type FCMClient struct {
	ProjectID  string
	OAuthToken string // Bearer access_token (server OAuth 2.0)
	HTTP       *http.Client
}

func NewFCMClient(projectID, oauthToken string) *FCMClient {
	return &FCMClient{
		ProjectID:  projectID,
		OAuthToken: oauthToken,
		HTTP: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  true,
				TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
			},
		},
	}
}

// FCMMessage 推送到单个 token
type FCMMessage struct {
	Message struct {
		Token        string            `json:"token"`
		Notification *FCMNotification  `json:"notification,omitempty"`
		Data         map[string]string `json:"data,omitempty"`
		Android      *FCMAndroid       `json:"android,omitempty"`
	} `json:"message"`
}

type FCMNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type FCMAndroid struct {
	Priority     string `json:"priority"` // "high" | "normal"
	CollapseKey  string `json:"collapse_key,omitempty"`
	TTL          string `json:"ttl,omitempty"` // e.g. "3600s"
}

// Send 发送一条 FCM 消息
func (c *FCMClient) Send(ctx context.Context, token, title, body string, data map[string]string) error {
	if c.ProjectID == "" || c.OAuthToken == "" {
		return fmt.Errorf("FCM not configured")
	}
	m := FCMMessage{}
	m.Message.Token = token
	m.Message.Notification = &FCMNotification{Title: title, Body: body}
	if data == nil {
		data = map[string]string{}
	}
	// 触发 WakeUp payload：Android 端 HostedService 收到启动信令
	data["action"] = "wake_up"
	m.Message.Data = data
	m.Message.Android = &FCMAndroid{
		Priority:    "high",
		CollapseKey: "linkall_wake",
		TTL:         "3600s",
	}
	payload, _ := json.Marshal(m)
	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", c.ProjectID)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+c.OAuthToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 || resp.StatusCode == 410 {
		// token 失效
		return fmt.Errorf("FCM token invalid (status %d)", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		return fmt.Errorf("FCM send failed: %d %s", resp.StatusCode, buf.String())
	}
	return nil
}

// === FCM token 注册 ===

// RegisterToken upsert device FCM token
func RegisterToken(database *sql.DB, deviceCode, token, appVersion string) error {
	if token == "" || deviceCode == "" {
		return fmt.Errorf("empty token/device")
	}
	_, err := database.Exec(`
		INSERT INTO fcm_tokens (id, device_code, token, platform, app_version, created_at, last_seen, revoked)
		VALUES (lower(hex(randomblob(16))), ?, ?, 'android', ?, unixepoch(), unixepoch(), 0)
		ON CONFLICT(token) DO UPDATE SET
			device_code = excluded.device_code,
			app_version = excluded.app_version,
			last_seen = unixepoch(),
			revoked = 0
	`, deviceCode, token, appVersion)
	return err
}

func GetToken(database *sql.DB, deviceCode string) (string, error) {
	var token string
	err := database.QueryRow(`SELECT token FROM fcm_tokens WHERE device_code=? AND revoked=0 ORDER BY last_seen DESC LIMIT 1`, deviceCode).Scan(&token)
	return token, err
}

func RevokeToken(database *sql.DB, deviceCode, token string) error {
	_, err := database.Exec(`UPDATE fcm_tokens SET revoked=1 WHERE device_code=? AND token=?`, deviceCode, token)
	return err
}

// === WoL 魔术包 ===

// SendMagicPacket 发送 Wake-on-LAN 魔术包
// 格式：6 字节 0xFF + 16 次目标 MAC（6 字节）= 102 字节
// 发到 broadcast IP:9（UDP）
func SendMagicPacket(broadcastAddr, mac string) error {
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ReplaceAll(mac, "-", "")
	if len(mac) != 12 {
		return fmt.Errorf("invalid MAC: %q", mac)
	}
	pkt := make([]byte, 102)
	for i := 0; i < 6; i++ {
		pkt[i] = 0xFF
	}
	macBytes := make([]byte, 6)
	for i := 0; i < 6; i++ {
		var b byte
		_, err := fmt.Sscanf(mac[2*i:2*i+2], "%02x", &b)
		if err != nil {
			return fmt.Errorf("mac parse: %w", err)
		}
		macBytes[i] = b
	}
	for i := 0; i < 16; i++ {
		copy(pkt[6+6*i:6+6*i+6], macBytes)
	}
	addr := broadcastAddr
	if !strings.Contains(addr, ":") {
		addr = net.JoinHostPort(addr, "9")
	}
	conn, err := net.Dial("udp", addr)
	if err != nil {
		// fallback 9 → 7
		addr = strings.Replace(addr, ":9", ":7", 1)
		conn, err = net.Dial("udp", addr)
		if err != nil {
			return err
		}
	}
	defer conn.Close()
	_, err = conn.Write(pkt)
	return err
}

// ResolveBroadcastAddr 从子网 ip/mask 算 broadcast
func ResolveBroadcastAddr(ip, mask string) (string, error) {
	ipv4 := net.ParseIP(ip).To4()
	m := net.IPMask(net.ParseIP(mask).To4())
	if ipv4 == nil || m == nil {
		return "255.255.255.255", nil
	}
	broadcast := net.IPv4(0, 0, 0, 0).To4()
	for i := range broadcast {
		broadcast[i] = ipv4[i] | ^m[i]
	}
	return broadcast.String(), nil
}

// GetDeviceMAC 查设备的 MAC（从 net_lease 表）
func GetDeviceMAC(database *sql.DB, deviceID string) (string, error) {
	var mac sql.NullString
	err := database.QueryRow(`SELECT mac FROM net_leases WHERE device_id=? ORDER BY leased_at DESC LIMIT 1`, deviceID).Scan(&mac)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return mac.String, err
}

// GetLastIP 查设备最近一次租约 IP
func GetLastIP(database *sql.DB, deviceID string) (string, error) {
	var ip sql.NullString
	err := database.QueryRow(`SELECT ip FROM net_leases WHERE device_id=? ORDER BY leased_at DESC LIMIT 1`, deviceID).Scan(&ip)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return ip.String, err
}

// 显式依赖 db 包（占位；通过 _ 避免 unused 警告）
var _ = sql.ErrNoRows
