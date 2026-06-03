// OTA 包 Ed25519 签名 / 验签
// 私钥本地持久化到 SQLite settings 表（生产环境建议拆出到 KMS / HSM）
// 公钥通过 /api/ota/pubkey 公开给客户端验签
package ota

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/linkall/server/internal/db"
)

type Signer struct {
	priv     ed25519.PrivateKey
	pub      ed25519.PublicKey
	keyID    string
}

var S *Signer

// Init 首次启动自动生成密钥对
func Init() error {
	privB64 := db.GetSetting("ota_privkey_b64")
	pubB64 := db.GetSetting("ota_pubkey_b64")
	kid := db.GetSetting("ota_keyid")
	if privB64 == "" || pubB64 == "" {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		kid = "ota-" + time.Now().Format("20060102") + "-" + shortRand()
		_ = db.SetSetting("ota_privkey_b64", base64.StdEncoding.EncodeToString(priv))
		_ = db.SetSetting("ota_pubkey_b64", base64.StdEncoding.EncodeToString(pub))
		_ = db.SetSetting("ota_keyid", kid)
		S = &Signer{priv: priv, pub: pub, keyID: kid}
		return nil
	}
	priv, _ := base64.StdEncoding.DecodeString(privB64)
	pub, _ := base64.StdEncoding.DecodeString(pubB64)
	if len(priv) != ed25519.PrivateKeySize || len(pub) != ed25519.PublicKeySize {
		return errors.New("OTA key size mismatch")
	}
	S = &Signer{priv: priv, pub: pub, keyID: kid}
	return nil
}

func (s *Signer) SignFile(content []byte) (sig, sha256hex string, err error) {
	if s == nil {
		return "", "", errors.New("OTA signer not initialized")
	}
	sum := sha256.Sum256(content)
	sha256hex = hex.EncodeToString(sum[:])
	sig = base64.StdEncoding.EncodeToString(ed25519.Sign(s.priv, sum[:]))
	return sig, sha256hex, nil
}

// SignBytes 外部用：直接对 sha256 摘要签名
func SignBytes(sum []byte) []byte {
	if S == nil {
		return nil
	}
	return ed25519.Sign(S.priv, sum)
}

func (s *Signer) PublicKeyB64() string {
	if s == nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(s.pub)
}

func (s *Signer) KeyID() string {
	if s == nil {
		return ""
	}
	return s.keyID
}

// Rotate 重新生成密钥对（公钥变更后所有客户端需 OTA 更新内置公钥才能继续验签）
func (s *Signer) Rotate() error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	kid := "ota-" + time.Now().Format("20060102") + "-" + shortRand()
	if err := db.SetSetting("ota_privkey_b64", base64.StdEncoding.EncodeToString(priv)); err != nil {
		return err
	}
	if err := db.SetSetting("ota_pubkey_b64", base64.StdEncoding.EncodeToString(pub)); err != nil {
		return err
	}
	if err := db.SetSetting("ota_keyid", kid); err != nil {
		return err
	}
	s.priv = priv
	s.pub = pub
	s.keyID = kid
	return nil
}

func shortRand() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	const alpha = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	out := make([]byte, 0, 6)
	for _, x := range b {
		out = append(out, alpha[int(x)%len(alpha)])
	}
	return string(out)
}
