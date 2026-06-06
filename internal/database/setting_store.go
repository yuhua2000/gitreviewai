package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/yuhua2000/gitreviewai/internal/types"
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

// GetGitLabURL returns the GitLab URL from settings, or defaultVal if not set.
func (s *SettingStore) GetGitLabURL(ctx context.Context, defaultVal string) (string, error) {
	return s.Get(ctx, "gitlab_url", defaultVal)
}

// SetGitLabURL stores the GitLab URL.
func (s *SettingStore) SetGitLabURL(ctx context.Context, url string) error {
	return s.Set(ctx, "gitlab_url", url)
}

// GetGitLabToken returns the GitLab token from settings, or defaultVal if not set.
func (s *SettingStore) GetGitLabToken(ctx context.Context, defaultVal string) (string, error) {
	return s.Get(ctx, "gitlab_token", defaultVal)
}

// SetGitLabToken stores the GitLab token.
func (s *SettingStore) SetGitLabToken(ctx context.Context, token string) error {
	return s.Set(ctx, "gitlab_token", token)
}

// GetIgnorePaths returns the ignore paths from settings as a string slice.
func (s *SettingStore) GetIgnorePaths(ctx context.Context, defaultVal []string) ([]string, error) {
	val, err := s.Get(ctx, "ignore_paths", "")
	if err != nil {
		return nil, err
	}
	if val == "" {
		return defaultVal, nil
	}
	var paths []string
	if err := json.Unmarshal([]byte(val), &paths); err != nil {
		return defaultVal, nil
	}
	return paths, nil
}

// SetIgnorePaths stores ignore paths as a JSON array.
func (s *SettingStore) SetIgnorePaths(ctx context.Context, paths []string) error {
	data, err := json.Marshal(paths)
	if err != nil {
		return fmt.Errorf("marshal ignore_paths: %w", err)
	}
	return s.Set(ctx, "ignore_paths", string(data))
}

// GetMaxLineComments returns the max line comments setting.
func (s *SettingStore) GetMaxLineComments(ctx context.Context, defaultVal int) (int, error) {
	val, err := s.Get(ctx, "max_line_comments", "")
	if err != nil {
		return defaultVal, err
	}
	if val == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal, nil
	}
	return n, nil
}

// SetMaxLineComments stores the max line comments setting.
func (s *SettingStore) SetMaxLineComments(ctx context.Context, n int) error {
	return s.Set(ctx, "max_line_comments", strconv.Itoa(n))
}

// GetWebhookToken returns the webhook token from settings.
func (s *SettingStore) GetWebhookToken(ctx context.Context, defaultVal string) (string, error) {
	return s.Get(ctx, "webhook_token", defaultVal)
}

// SetWebhookToken stores the webhook token.
func (s *SettingStore) SetWebhookToken(ctx context.Context, token string) error {
	return s.Set(ctx, "webhook_token", token)
}

// GetLogLevel returns the log level from settings.
func (s *SettingStore) GetLogLevel(ctx context.Context, defaultVal string) (string, error) {
	return s.Get(ctx, "log_level", defaultVal)
}

// SetLogLevel stores the log level.
func (s *SettingStore) SetLogLevel(ctx context.Context, level string) error {
	return s.Set(ctx, "log_level", level)
}

// GetJWTExpiry returns the JWT expiry from settings.
func (s *SettingStore) GetJWTExpiry(ctx context.Context, defaultVal string) (string, error) {
	return s.Get(ctx, "jwt_expiry", defaultVal)
}

// SetJWTExpiry stores the JWT expiry.
func (s *SettingStore) SetJWTExpiry(ctx context.Context, expiry string) error {
	return s.Set(ctx, "jwt_expiry", expiry)
}

// GetGlobalSettings aggregates all global business settings.
func (s *SettingStore) GetGlobalSettings(ctx context.Context, defaults *types.GlobalSettings) (*types.GlobalSettings, error) {
	gs := &types.GlobalSettings{}

	gitlabURL, err := s.GetGitLabURL(ctx, defaults.GitLabURL)
	if err != nil {
		return nil, err
	}
	gs.GitLabURL = gitlabURL

	gitlabToken, err := s.GetGitLabToken(ctx, defaults.GitLabToken)
	if err != nil {
		return nil, err
	}
	gs.GitLabToken = gitlabToken

	ignorePaths, err := s.GetIgnorePaths(ctx, defaults.IgnorePaths)
	if err != nil {
		return nil, err
	}
	gs.IgnorePaths = ignorePaths

	maxComments, err := s.GetMaxLineComments(ctx, defaults.MaxLineComments)
	if err != nil {
		return nil, err
	}
	gs.MaxLineComments = maxComments

	autoSubmit, err := s.GetAutoSubmit(ctx)
	if err != nil {
		return nil, err
	}
	gs.AutoSubmit = autoSubmit

	return gs, nil
}
