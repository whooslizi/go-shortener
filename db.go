package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type ShortLink struct {
	ID        int64      `json:"id"`
	ShortCode string     `json:"short_code"`
	ImmichKey string     `json:"immich_key"`
	IPPURL    string     `json:"ipp_url"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

type DB struct {
	conn *sql.DB
}

func InitDB(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}

	conn, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// sqlite only supports one writer
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	schema := `
		CREATE TABLE IF NOT EXISTS short_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			short_code TEXT UNIQUE NOT NULL,
			immich_key TEXT UNIQUE NOT NULL,
			ipp_url TEXT NOT NULL,
			expires_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_short_code ON short_links(short_code);
		CREATE INDEX IF NOT EXISTS idx_immich_key ON short_links(immich_key);
		CREATE INDEX IF NOT EXISTS idx_expires_at ON short_links(expires_at);
	`
	if _, err := conn.Exec(schema); err != nil {
		return nil, fmt.Errorf("create schema: %w", err)
	}

	log.Printf("db ready: %s", dbPath)
	return &DB{conn: conn}, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) GetByShortCode(code string) (*ShortLink, error) {
	row := db.conn.QueryRow(
		"SELECT id, short_code, immich_key, ipp_url, expires_at, created_at FROM short_links WHERE short_code = ?",
		code,
	)
	return scanLink(row)
}

func (db *DB) GetByImmichKey(key string) (*ShortLink, error) {
	row := db.conn.QueryRow(
		"SELECT id, short_code, immich_key, ipp_url, expires_at, created_at FROM short_links WHERE immich_key = ?",
		key,
	)
	return scanLink(row)
}

func scanLink(row *sql.Row) (*ShortLink, error) {
	link := &ShortLink{}
	var expiresAt sql.NullTime
	err := row.Scan(&link.ID, &link.ShortCode, &link.ImmichKey, &link.IPPURL, &expiresAt, &link.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if expiresAt.Valid {
		link.ExpiresAt = &expiresAt.Time
	}
	return link, nil
}

func (db *DB) Insert(shortCode, immichKey, ippURL string, expiresAt *time.Time) error {
	var expiry interface{}
	if expiresAt != nil {
		expiry = expiresAt.UTC()
	}

	_, err := db.conn.Exec(
		"INSERT INTO short_links (short_code, immich_key, ipp_url, expires_at) VALUES (?, ?, ?, ?)",
		shortCode, immichKey, ippURL, expiry,
	)
	return err
}

func (db *DB) UpdateExpiry(immichKey string, expiresAt *time.Time) error {
	var expiry interface{}
	if expiresAt != nil {
		expiry = expiresAt.UTC()
	}

	_, err := db.conn.Exec(
		"UPDATE short_links SET expires_at = ? WHERE immich_key = ?",
		expiry, immichKey,
	)
	return err
}

func (db *DB) DeleteExpired() (int64, error) {
	result, err := db.conn.Exec(
		"DELETE FROM short_links WHERE expires_at IS NOT NULL AND expires_at < ?",
		time.Now().UTC(),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (db *DB) DeleteByImmichKey(key string) error {
	_, err := db.conn.Exec("DELETE FROM short_links WHERE immich_key = ?", key)
	return err
}

func (db *DB) GetAll() ([]ShortLink, error) {
	rows, err := db.conn.Query(
		"SELECT id, short_code, immich_key, ipp_url, expires_at, created_at FROM short_links ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []ShortLink
	for rows.Next() {
		link := ShortLink{}
		var expiresAt sql.NullTime
		if err := rows.Scan(&link.ID, &link.ShortCode, &link.ImmichKey, &link.IPPURL, &expiresAt, &link.CreatedAt); err != nil {
			return nil, err
		}
		if expiresAt.Valid {
			link.ExpiresAt = &expiresAt.Time
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func (db *DB) Count() (int64, error) {
	var count int64
	err := db.conn.QueryRow("SELECT COUNT(*) FROM short_links").Scan(&count)
	return count, err
}
