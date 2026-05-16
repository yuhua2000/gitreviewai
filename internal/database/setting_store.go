package database

import (
	"context"
	"database/sql"
)

type SettingStore struct {
	db *sql.DB
}

func NewSettingStore(db *sql.DB) *SettingStore {
	return &SettingStore{db: db}
}

func (s *SettingStore) Get(ctx context.Context, key, defaultValue string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return defaultValue, nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

func (s *SettingStore) Set(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')`,
		key, value)
	return err
}

func (s *SettingStore) GetAutoSubmit(ctx context.Context) (bool, error) {
	val, err := s.Get(ctx, "auto_submit", "false")
	if err != nil {
		return false, err
	}
	return val == "true", nil
}

func (s *SettingStore) SetAutoSubmit(ctx context.Context, enabled bool) error {
	val := "false"
	if enabled {
		val = "true"
	}
	return s.Set(ctx, "auto_submit", val)
}
