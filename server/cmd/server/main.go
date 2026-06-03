package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/linkall/server/internal/api"
	"github.com/linkall/server/internal/auth"
	"github.com/linkall/server/internal/config"
	"github.com/linkall/server/internal/db"
	"github.com/linkall/server/internal/ota"
	"github.com/linkall/server/internal/security"
	"github.com/linkall/server/internal/signaling"
)

func main() {
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		switch os.Args[1] {
		case "init-admin":
			runInitAdmin(os.Args[2:])
			return
		case "rotate-ota":
			runRotateOTA()
			return
		case "rotate-jwt":
			runRotateJWT()
			return
		case "help", "-h", "--help":
			printHelp()
			return
		}
	}

	cfg := config.Load()
	cfg.EnsureDirs()

	if err := db.Open(cfg.DBPath); err != nil {
		log.Fatalf("open db: %v", err)
	}

	// 初始化安全子系统
	if err := auth.KeyMgr.Init(); err != nil {
		log.Fatalf("init jwt keys: %v", err)
	}
	if err := ota.Init(); err != nil {
		log.Fatalf("init ota signer: %v", err)
	}
	signaling.Nonces.Start()
	defer signaling.Nonces.Stop()
	// 应用限流规则
	_ = security.LoadRateConfig()
	security.ReloadLimiter(security.GetRateConfig())

	if err := ensureSuperAdminInteractive(); err != nil {
		log.Fatalf("init admin: %v", err)
	}

	hub := signaling.NewHub()
	hub.OnHello = func(p *signaling.Peer, kind signaling.PeerKind, code, token string) error {
		if kind == signaling.PeerControlled {
			if code == "" || token == "" {
				return errors.New("controlled needs device_code & token")
			}
			d, _, err := modelsFindDeviceByCode(code)
			if err != nil {
				return errors.New("device not found")
			}
			if !d.AcceptConnections {
				return errors.New("device rejects connections")
			}
			_, err = auth.NewJWTFromEnv().Parse(strings.TrimPrefix(token, "Bearer "))
			if err != nil {
				return errors.New("device token invalid")
			}
			_, _ = modelsUpdateDeviceMeta(d.DeviceCode, "", "", "", "", "", "")
			modelsSetDeviceOnline(d.DeviceCode, true)
			p.DeviceCode = d.DeviceCode
		} else if kind == signaling.PeerController {
			if token != "" {
				_, err := auth.NewJWTFromEnv().Parse(strings.TrimPrefix(token, "Bearer "))
				if err != nil {
					return errors.New("controller token invalid")
				}
			}
		}
		return nil
	}
	hub.OnCmd = func(from *signaling.Peer, env *signaling.Envelope) {
		log.Printf("[cmd] from=%s to=%s len=%d", from.ID, env.To, len(env.Data))
		security.Record(security.AuditEvent{ActorID: 0, Action: "ws_cmd", IP: from.Conn.IP(), Target: env.To, Detail: string(env.Type)})
	}

	jwt := auth.NewJWTFromEnv()
	app := api.NewRouter(api.Deps{JWT: jwt, Hub: hub})

	// 静态文件（网页前端构建产物）
	staticDir := "./web"
	if _, err := os.Stat(staticDir); err == nil {
		app.Static("/", staticDir, fiber.Static{
			Index:  "index.html",
			Browse: false,
		})
		app.Get("/*", func(c *fiber.Ctx) error {
			p := c.Path()
			if strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/ws") {
				return c.Next()
			}
			return c.SendFile(staticDir + "/index.html")
		})
	}

	addr := cfg.HTTPAddr
	log.Printf("==============================================")
	log.Printf(" LinkALL Server v1.0.0  (Go %s)", "1.22+")
	log.Printf(" Public URL : %s", cfg.PublicURL)
	log.Printf(" DB         : %s", cfg.DBPath)
	log.Printf(" OTA        : %s", cfg.OTADir)
	log.Printf(" OTA KeyID  : %s", ota.S.KeyID())
	log.Printf(" Active JWT : %s", auth.KeyMgr.ActiveKid())
	log.Printf(" Listening  : %s", addr)
	log.Printf("==============================================")
	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		log.Fatal(app.ListenTLS(addr, cfg.TLSCert, cfg.TLSKey))
	} else {
		log.Fatal(app.Listen(addr))
	}
}

func runInitAdmin(args []string) {
	fs := flag.NewFlagSet("init-admin", flag.ExitOnError)
	username := fs.String("u", "", "用户名 (>=3)")
	password := fs.String("p", "", "密码 (>=8, 含字母数字)")
	_ = fs.Parse(args)
	cfg := config.Load()
	cfg.EnsureDirs()
	if err := db.Open(cfg.DBPath); err != nil {
		log.Fatalf("open db: %v", err)
	}
	if *username == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "用法: linkall-server init-admin -u <user> -p <password>")
		os.Exit(2)
	}
	if _, err := auth.CreateUser(*username, *password, true, true); err != nil {
		log.Fatalf("create: %v", err)
	}
	fmt.Printf("[init] 超级管理员已创建: %s\n", *username)
}

func runRotateOTA() {
	cfg := config.Load()
	cfg.EnsureDirs()
	if err := db.Open(cfg.DBPath); err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := ota.Init(); err != nil {
		log.Fatalf("ota init: %v", err)
	}
	if err := ota.S.Rotate(); err != nil {
		log.Fatalf("rotate: %v", err)
	}
	fmt.Printf("[ota] 新公钥: %s\nkid=%s\n", ota.S.PublicKeyB64(), ota.S.KeyID())
}

func runRotateJWT() {
	cfg := config.Load()
	cfg.EnsureDirs()
	if err := db.Open(cfg.DBPath); err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := auth.KeyMgr.Init(); err != nil {
		log.Fatalf("jwt init: %v", err)
	}
	kid, err := auth.KeyMgr.Rotate()
	if err != nil {
		log.Fatalf("rotate: %v", err)
	}
	fmt.Printf("[jwt] 新 active kid: %s\n", kid)
}

func ensureSuperAdminInteractive() error {
	var n int
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE is_super_admin=1`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if !isTerminal() {
		log.Println("[init] 非交互式终端且无超级管理员，跳过自动创建")
		log.Println("      请用 'linkall-server init-admin -u <user> -p <pass>' 创建")
		return nil
	}
	fmt.Println("=== LinkALL 首次启动：创建超级管理员 ===")
	r := bufio.NewReader(os.Stdin)
	fmt.Print("用户名 (>=3 字符): ")
	un, _ := r.ReadString('\n')
	un = strings.TrimSpace(un)
	if len(un) < 3 {
		return errors.New("用户名太短")
	}
	fmt.Print("密码 (>=8 字符, 含字母和数字): ")
	pw1, _ := r.ReadString('\n')
	pw1 = strings.TrimSpace(pw1)
	fmt.Print("确认密码: ")
	pw2, _ := r.ReadString('\n')
	pw2 = strings.TrimSpace(pw2)
	if pw1 != pw2 {
		return errors.New("两次密码不一致")
	}
	if len(pw1) < 8 {
		return errors.New("密码至少 8 位")
	}
	hasLetter, hasDigit := false, false
	for _, c := range pw1 {
		switch {
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
			hasLetter = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return errors.New("密码必须包含字母和数字")
	}
	_, err := auth.CreateUser(un, pw1, true, true)
	if err != nil {
		return err
	}
	fmt.Println("[init] 超级管理员已创建:", un)
	return nil
}

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func printHelp() {
	fmt.Print(`LinkALL Server

用法:
  linkall-server                  启动 HTTP 服务
  linkall-server init-admin -u U -p P   创建超级管理员
  linkall-server rotate-ota             重新生成 OTA 密钥对
  linkall-server rotate-jwt             轮换 JWT active kid

环境变量（详见 server/.env.example）:
  HTTP_ADDR, PUBLIC_URL, TLS_CERT, TLS_KEY
  DB_PATH, OTA_DIR, LOG_DIR
  JWT_SECRET（旧）/ 用 admin API 轮换
  ARGON2_TIME / MEMORY_KB / THREADS / KEYLEN
  STUN_URLS, TURN_URL, TURN_USER, TURN_CRED
  OFFICIAL_SERVER, MAX_CONCURRENT_SESSIONS,
  SESSION_IDLE_TIMEOUT_MIN, DATA_RETENTION_DAYS
  REQUIRE_DEVICE_CODE_DEFAULT, ALLOW_ANONYMOUS_DEFAULT
  INVITE_DEFAULT_TTL_HOURS
`)
}
