// 审计日志
package security

import (
	"database/sql"
	"log"
	"time"

	"github.com/linkall/server/internal/db"
)

type AuditEvent struct {
	ActorID  int64
	Action   string
	Target   string
	IP       string
	Detail   string
}

func Record(ev AuditEvent) {
	if ev.Action == "" {
		return
	}
	_, err := db.DB.Exec(
		`INSERT INTO audit_logs(actor_id, action, target, ip, detail, created_at) VALUES(?,?,?,?,?,?)`,
		ev.ActorID, ev.Action, ev.Target, ev.IP, ev.Detail, time.Now().Unix(),
	)
	if err != nil {
		log.Printf("[audit] insert: %v", err)
	}
}

func List(limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := db.DB.Query(
		`SELECT id, COALESCE(actor_id,0), action, COALESCE(target,''), COALESCE(ip,''), COALESCE(detail,''), created_at
		 FROM audit_logs ORDER BY id DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, actor int64
		var action, target, ip, detail string
		var ts int64
		if err := rows.Scan(&id, &actor, &action, &target, &ip, &detail, &ts); err != nil {
			if err == sql.ErrNoRows {
				break
			}
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "actor_id": actor, "action": action, "target": target,
			"ip": ip, "detail": detail, "created_at": ts,
		})
	}
	return out, nil
}
