// 文件传输状态模型（断点续传）
package models

import (
	"database/sql"
	"errors"
	"time"

	"github.com/linkall/server/internal/db"
)

type FileTransfer struct {
	ID              string `json:"id"`
	Direction       string `json:"direction"`
	TransferID      string `json:"transfer_id"`
	ControllerID    string `json:"controller_id"`
	ControlledCode  string `json:"controlled_code"`
	Name            string `json:"name"`
	Size            int64  `json:"size"`
	SHA256Expected  string `json:"sha256_expected"`
	ChunkSize       int64  `json:"chunk_size"`
	ReceivedOffset  int64  `json:"received_offset"`
	Status          string `json:"status"`
	FilePath        string `json:"file_path"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

func CreateFileTransfer(ft *FileTransfer) error {
	now := time.Now().Unix()
	ft.CreatedAt = now
	ft.UpdatedAt = now
	_, err := db.DB.Exec(
		`INSERT INTO file_transfers(id, direction, transfer_id, controller_id, controlled_code,
			name, size, sha256_expected, chunk_size, received_offset, status, file_path, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		ft.ID, ft.Direction, ft.TransferID, ft.ControllerID, ft.ControlledCode,
		ft.Name, ft.Size, ft.SHA256Expected, ft.ChunkSize, ft.ReceivedOffset, ft.Status, ft.FilePath,
		ft.CreatedAt, ft.UpdatedAt,
	)
	return err
}

func GetFileTransfer(id string) (*FileTransfer, error) {
	row := db.DB.QueryRow(
		`SELECT id, direction, transfer_id, controller_id, controlled_code, name, size, COALESCE(sha256_expected,''), chunk_size, received_offset, status, COALESCE(file_path,''), created_at, updated_at FROM file_transfers WHERE id=?`,
		id,
	)
	ft := &FileTransfer{}
	err := row.Scan(&ft.ID, &ft.Direction, &ft.TransferID, &ft.ControllerID, &ft.ControlledCode,
		&ft.Name, &ft.Size, &ft.SHA256Expected, &ft.ChunkSize, &ft.ReceivedOffset, &ft.Status, &ft.FilePath,
		&ft.CreatedAt, &ft.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, errors.New("transfer not found")
	}
	return ft, err
}

func GetFileTransferByTID(tid string) (*FileTransfer, error) {
	row := db.DB.QueryRow(
		`SELECT id, direction, transfer_id, controller_id, controlled_code, name, size, COALESCE(sha256_expected,''), chunk_size, received_offset, status, COALESCE(file_path,''), created_at, updated_at FROM file_transfers WHERE transfer_id=? ORDER BY updated_at DESC LIMIT 1`,
		tid,
	)
	ft := &FileTransfer{}
	err := row.Scan(&ft.ID, &ft.Direction, &ft.TransferID, &ft.ControllerID, &ft.ControlledCode,
		&ft.Name, &ft.Size, &ft.SHA256Expected, &ft.ChunkSize, &ft.ReceivedOffset, &ft.Status, &ft.FilePath,
		&ft.CreatedAt, &ft.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, errors.New("transfer not found")
	}
	return ft, err
}

func UpdateFileTransferProgress(id string, receivedOffset int64, status string) error {
	now := time.Now().Unix()
	_, err := db.DB.Exec(
		`UPDATE file_transfers SET received_offset=?, status=COALESCE(NULLIF(?, ''), status), updated_at=? WHERE id=?`,
		receivedOffset, status, now, id,
	)
	return err
}

func CompleteFileTransfer(id string) error {
	_, err := db.DB.Exec(
		`UPDATE file_transfers SET status='completed', updated_at=strftime('%s','now') WHERE id=?`,
		id,
	)
	return err
}

func AbortFileTransfer(id string) error {
	_, err := db.DB.Exec(
		`UPDATE file_transfers SET status='aborted', updated_at=strftime('%s','now') WHERE id=?`,
		id,
	)
	return err
}

func ListFileTransfers(scope string, key string, limit int) ([]FileTransfer, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT id, direction, transfer_id, controller_id, controlled_code, name, size, COALESCE(sha256_expected,''), chunk_size, received_offset, status, COALESCE(file_path,''), created_at, updated_at FROM file_transfers`
	var args []any
	if scope != "" && key != "" {
		q += ` WHERE ` + scope + `=?`
		args = append(args, key)
	}
	q += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FileTransfer{}
	for rows.Next() {
		var ft FileTransfer
		if err := rows.Scan(&ft.ID, &ft.Direction, &ft.TransferID, &ft.ControllerID, &ft.ControlledCode,
			&ft.Name, &ft.Size, &ft.SHA256Expected, &ft.ChunkSize, &ft.ReceivedOffset, &ft.Status, &ft.FilePath,
			&ft.CreatedAt, &ft.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, ft)
	}
	return out, nil
}
