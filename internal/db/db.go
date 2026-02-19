package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func Migrate(db *sql.DB) error {
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("migration error: %w\nSQL: %s", err, m)
		}
	}
	return nil
}

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL REFERENCES users(id),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS servers (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		game TEXT NOT NULL,
		container_id TEXT,
		image TEXT NOT NULL,
		ports TEXT NOT NULL DEFAULT '[]',
		env TEXT NOT NULL DEFAULT '{}',
		volumes TEXT NOT NULL DEFAULT '{}',
		memory_limit INTEGER DEFAULT 0,
		cpu_limit REAL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'stopped',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS stats (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
		cpu_percent REAL,
		memory_bytes INTEGER,
		memory_limit INTEGER,
		disk_bytes INTEGER,
		network_rx INTEGER,
		network_tx INTEGER,
		recorded_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_stats_server_time ON stats(server_id, recorded_at)`,
	`CREATE TABLE IF NOT EXISTS backups (
		id TEXT PRIMARY KEY,
		server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
		filename TEXT NOT NULL,
		size_bytes INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS schedules (
		id TEXT PRIMARY KEY,
		server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		cron_expr TEXT NOT NULL,
		action TEXT NOT NULL,
		enabled INTEGER DEFAULT 1,
		last_run DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
}
