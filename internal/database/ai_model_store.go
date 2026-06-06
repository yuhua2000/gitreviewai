package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/yuhua2000/gitreviewai/internal/types"
)

// AIModelStore manages AI model configurations.
type AIModelStore struct {
	db *sql.DB
}

// NewAIModelStore creates a new AIModelStore.
func NewAIModelStore(db *sql.DB) *AIModelStore {
	return &AIModelStore{db: db}
}

// Create inserts a new AI model configuration.
func (s *AIModelStore) Create(ctx context.Context, m *types.AIModel) (int64, error) {
	query := `INSERT INTO ai_models (name, base_url, api_key, model_name, is_default, enabled)
		VALUES (?, ?, ?, ?, ?, ?)`
	result, err := s.db.ExecContext(ctx, query, m.Name, m.BaseURL, m.APIKey, m.ModelName, m.IsDefault, m.Enabled)
	if err != nil {
		return 0, fmt.Errorf("insert ai_model: %w", err)
	}
	return result.LastInsertId()
}

// Update modifies an existing AI model configuration.
func (s *AIModelStore) Update(ctx context.Context, m *types.AIModel) error {
	query := `UPDATE ai_models SET name=?, base_url=?, api_key=?, model_name=?, is_default=?, enabled=?, updated_at=datetime('now')
		WHERE id=?`
	_, err := s.db.ExecContext(ctx, query, m.Name, m.BaseURL, m.APIKey, m.ModelName, m.IsDefault, m.Enabled, m.ID)
	if err != nil {
		return fmt.Errorf("update ai_model: %w", err)
	}
	return nil
}

// Delete removes an AI model by ID.
func (s *AIModelStore) Delete(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM ai_models WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete ai_model: %w", err)
	}
	return nil
}

// GetByID retrieves an AI model by ID.
func (s *AIModelStore) GetByID(ctx context.Context, id int64) (*types.AIModel, error) {
	query := `SELECT id, name, base_url, api_key, model_name, is_default, enabled, created_at, updated_at
		FROM ai_models WHERE id=?`
	m := &types.AIModel{}
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&m.ID, &m.Name, &m.BaseURL, &m.APIKey, &m.ModelName, &m.IsDefault, &m.Enabled, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get ai_model by id: %w", err)
	}
	return m, nil
}

// List returns all AI models ordered by default flag and name.
func (s *AIModelStore) List(ctx context.Context) ([]*types.AIModel, error) {
	query := `SELECT id, name, base_url, api_key, model_name, is_default, enabled, created_at, updated_at
		FROM ai_models ORDER BY is_default DESC, name ASC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list ai_models: %w", err)
	}
	defer rows.Close()

	var models []*types.AIModel
	for rows.Next() {
		m := &types.AIModel{}
		if err := rows.Scan(&m.ID, &m.Name, &m.BaseURL, &m.APIKey, &m.ModelName, &m.IsDefault, &m.Enabled, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan ai_model: %w", err)
		}
		models = append(models, m)
	}
	return models, nil
}

// GetDefault returns the default AI model, or nil if none is set.
func (s *AIModelStore) GetDefault(ctx context.Context) (*types.AIModel, error) {
	query := `SELECT id, name, base_url, api_key, model_name, is_default, enabled, created_at, updated_at
		FROM ai_models WHERE is_default=1 AND enabled=1 LIMIT 1`
	m := &types.AIModel{}
	err := s.db.QueryRowContext(ctx, query).Scan(
		&m.ID, &m.Name, &m.BaseURL, &m.APIKey, &m.ModelName, &m.IsDefault, &m.Enabled, &m.CreatedAt, &m.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get default ai_model: %w", err)
	}
	return m, nil
}

// SetDefault sets a model as default, clearing others. Uses a transaction.
func (s *AIModelStore) SetDefault(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE ai_models SET is_default=0, updated_at=datetime('now') WHERE is_default=1`); err != nil {
		return fmt.Errorf("clear defaults: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE ai_models SET is_default=1, updated_at=datetime('now') WHERE id=?`, id); err != nil {
		return fmt.Errorf("set default: %w", err)
	}

	return tx.Commit()
}
