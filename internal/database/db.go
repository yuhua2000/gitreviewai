package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS merge_requests (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id    TEXT    NOT NULL,
			mr_iid        INTEGER NOT NULL,
			title         TEXT    NOT NULL DEFAULT '',
			description   TEXT    NOT NULL DEFAULT '',
			source_branch TEXT    NOT NULL DEFAULT '',
			target_branch TEXT    NOT NULL DEFAULT '',
			state         TEXT    NOT NULL DEFAULT 'open',
			web_url       TEXT    NOT NULL DEFAULT '',
			review_status TEXT    NOT NULL DEFAULT 'pending',
			created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at    DATETIME NOT NULL DEFAULT (datetime('now')),
			UNIQUE(project_id, mr_iid)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mr_project_iid ON merge_requests(project_id, mr_iid)`,
		`CREATE INDEX IF NOT EXISTS idx_mr_review_status ON merge_requests(review_status)`,
		`CREATE INDEX IF NOT EXISTS idx_mr_created_at ON merge_requests(created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS comments (
			id                   INTEGER PRIMARY KEY AUTOINCREMENT,
			mr_id                INTEGER NOT NULL,
			comment_type         TEXT    NOT NULL,
			file_path            TEXT    NOT NULL DEFAULT '',
			line_number          INTEGER NOT NULL DEFAULT 0,
			content              TEXT    NOT NULL DEFAULT '',
			diff_context         TEXT    NOT NULL DEFAULT '',
			status               TEXT    NOT NULL DEFAULT 'pending',
			gitlab_note_id       INTEGER,
			gitlab_draft_note_id INTEGER,
			submitted_at         DATETIME,
			created_at           DATETIME NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (mr_id) REFERENCES merge_requests(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_comment_mr_id ON comments(mr_id)`,
		`CREATE INDEX IF NOT EXISTS idx_comment_status ON comments(status)`,
		`CREATE TABLE IF NOT EXISTS reports (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			mr_id          INTEGER NOT NULL,
			content        TEXT    NOT NULL DEFAULT '',
			status         TEXT    NOT NULL DEFAULT 'pending',
			gitlab_note_id INTEGER,
			submitted_at   DATETIME,
			created_at     DATETIME NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (mr_id) REFERENCES merge_requests(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_report_mr_id ON reports(mr_id)`,
		`CREATE INDEX IF NOT EXISTS idx_report_status ON reports(status)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key       TEXT PRIMARY KEY,
			value     TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("exec migration: %w\nQuery: %s", err, q)
		}
	}

	return nil
}
