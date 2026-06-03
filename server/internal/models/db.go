package models

import (
	"database/sql"

	"github.com/linkall/server/internal/db"
)

// DB 暴露底层 *sql.DB 以便需要直接执行 SQL 的接口使用
func DB() *sql.DB { return db.DB }
