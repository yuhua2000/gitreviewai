package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/yuhua2000/gitreviewai/internal/types"
)

// ProjectConfigStore manages project-level configurations.
type ProjectConfigStore struct {
	db *sql.DB
}

// NewProjectConfigStore creates a new ProjectConfigStore.
func NewProjectConfigStore(db *sql.DB) *ProjectConfigStore {
	return &ProjectConfigStore{db: db}
}

// GetByProjectID retrieves a project config by GitLab project ID.
func (s *ProjectConfigStore) GetByProjectID(ctx context.Context, gitlabProjectID int) (*types.ProjectConfig, error) {
	query := `SELECT id, gitlab_project_id, project_name, description, ai_model_id, auto_submit,
		skip_draft, target_branches, ignore_paths, max_line_comments, custom_prompt, enabled, created_at, updated_at
		FROM project_configs WHERE gitlab_project_id=?`
	pc := &types.ProjectConfig{}
	err := s.db.QueryRowContext(ctx, query, gitlabProjectID).Scan(
		&pc.ID, &pc.GitlabProjectID, &pc.ProjectName, &pc.Description, &pc.AIModelID,
		&pc.AutoSubmit, &pc.SkipDraft, &pc.TargetBranches, &pc.IgnorePaths,
		&pc.MaxLineComments, &pc.CustomPrompt, &pc.Enabled, &pc.CreatedAt, &pc.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get project_config: %w", err)
	}
	return pc, nil
}

// Create inserts a new project configuration.
func (s *ProjectConfigStore) Create(ctx context.Context, pc *types.ProjectConfig) (int64, error) {
	query := `INSERT INTO project_configs (gitlab_project_id, project_name, description, ai_model_id,
		auto_submit, skip_draft, target_branches, ignore_paths, max_line_comments, custom_prompt, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := s.db.ExecContext(ctx, query,
		pc.GitlabProjectID, pc.ProjectName, pc.Description, pc.AIModelID,
		pc.AutoSubmit, pc.SkipDraft, pc.TargetBranches, pc.IgnorePaths,
		pc.MaxLineComments, pc.CustomPrompt, pc.Enabled,
	)
	if err != nil {
		return 0, fmt.Errorf("insert project_config: %w", err)
	}
	return result.LastInsertId()
}

// Update modifies an existing project configuration.
func (s *ProjectConfigStore) Update(ctx context.Context, pc *types.ProjectConfig) error {
	query := `UPDATE project_configs SET project_name=?, description=?, ai_model_id=?,
		auto_submit=?, skip_draft=?, target_branches=?, ignore_paths=?, max_line_comments=?,
		custom_prompt=?, enabled=?, updated_at=datetime('now')
		WHERE id=?`
	_, err := s.db.ExecContext(ctx, query,
		pc.ProjectName, pc.Description, pc.AIModelID,
		pc.AutoSubmit, pc.SkipDraft, pc.TargetBranches, pc.IgnorePaths,
		pc.MaxLineComments, pc.CustomPrompt, pc.Enabled, pc.ID,
	)
	if err != nil {
		return fmt.Errorf("update project_config: %w", err)
	}
	return nil
}

// List returns project configs with pagination.
func (s *ProjectConfigStore) List(ctx context.Context, offset, limit int) ([]*types.ProjectConfig, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM project_configs`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count project_configs: %w", err)
	}

	query := `SELECT id, gitlab_project_id, project_name, description, ai_model_id, auto_submit,
		skip_draft, target_branches, ignore_paths, max_line_comments, custom_prompt, enabled, created_at, updated_at
		FROM project_configs ORDER BY updated_at DESC LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list project_configs: %w", err)
	}
	defer rows.Close()

	var configs []*types.ProjectConfig
	for rows.Next() {
		pc := &types.ProjectConfig{}
		if err := rows.Scan(&pc.ID, &pc.GitlabProjectID, &pc.ProjectName, &pc.Description, &pc.AIModelID,
			&pc.AutoSubmit, &pc.SkipDraft, &pc.TargetBranches, &pc.IgnorePaths,
			&pc.MaxLineComments, &pc.CustomPrompt, &pc.Enabled, &pc.CreatedAt, &pc.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan project_config: %w", err)
		}
		configs = append(configs, pc)
	}
	return configs, total, nil
}

// GetOrCreate retrieves a project config or auto-creates one with defaults.
func (s *ProjectConfigStore) GetOrCreate(ctx context.Context, gitlabProjectID int, projectName, description string) (*types.ProjectConfig, error) {
	pc, err := s.GetByProjectID(ctx, gitlabProjectID)
	if err != nil {
		return nil, err
	}
	if pc != nil {
		// Update cached fields if changed
		if pc.ProjectName != projectName || pc.Description != description {
			pc.ProjectName = projectName
			pc.Description = description
			if updateErr := s.Update(ctx, pc); updateErr != nil {
				// Log but don't fail
				_ = updateErr
			}
		}
		return pc, nil
	}

	// Auto-create with defaults
	pc = &types.ProjectConfig{
		GitlabProjectID: gitlabProjectID,
		ProjectName:     projectName,
		Description:     description,
		AutoSubmit:      false,
		SkipDraft:       true,
		TargetBranches:  "[]",
		IgnorePaths:     "[]",
		Enabled:         true,
	}
	id, err := s.Create(ctx, pc)
	if err != nil {
		return nil, err
	}
	pc.ID = id
	return pc, nil
}

// GetRuleOverrides returns rule overrides for a project.
func (s *ProjectConfigStore) GetRuleOverrides(ctx context.Context, projectConfigID int64) ([]*types.ProjectRuleOverride, error) {
	query := `SELECT id, project_config_id, rule_id, enabled FROM project_rule_overrides WHERE project_config_id=?`
	rows, err := s.db.QueryContext(ctx, query, projectConfigID)
	if err != nil {
		return nil, fmt.Errorf("list rule overrides: %w", err)
	}
	defer rows.Close()

	var overrides []*types.ProjectRuleOverride
	for rows.Next() {
		o := &types.ProjectRuleOverride{}
		if err := rows.Scan(&o.ID, &o.ProjectConfigID, &o.RuleID, &o.Enabled); err != nil {
			return nil, fmt.Errorf("scan rule override: %w", err)
		}
		overrides = append(overrides, o)
	}
	return overrides, nil
}

// SetRuleOverrides replaces all rule overrides for a project using a transaction.
func (s *ProjectConfigStore) SetRuleOverrides(ctx context.Context, projectConfigID int64, overrides []*types.ProjectRuleOverride) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Delete existing overrides
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_rule_overrides WHERE project_config_id=?`, projectConfigID); err != nil {
		return fmt.Errorf("delete overrides: %w", err)
	}

	// Insert new overrides
	if len(overrides) > 0 {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO project_rule_overrides (project_config_id, rule_id, enabled) VALUES (?, ?, ?)`)
		if err != nil {
			return fmt.Errorf("prepare insert: %w", err)
		}
		defer stmt.Close()

		for _, o := range overrides {
			if _, err := stmt.ExecContext(ctx, projectConfigID, o.RuleID, o.Enabled); err != nil {
				return fmt.Errorf("insert override: %w", err)
			}
		}
	}

	return tx.Commit()
}

// GetTargetBranches returns the target branches as a string slice.
func (s *ProjectConfigStore) GetTargetBranches(pc *types.ProjectConfig) []string {
	if pc.TargetBranches == "" || pc.TargetBranches == "[]" {
		return nil
	}
	var branches []string
	if err := json.Unmarshal([]byte(pc.TargetBranches), &branches); err != nil {
		return nil
	}
	return branches
}

// GetIgnorePaths returns the ignore paths as a string slice.
func (s *ProjectConfigStore) GetIgnorePaths(pc *types.ProjectConfig) []string {
	if pc.IgnorePaths == "" || pc.IgnorePaths == "[]" {
		return nil
	}
	var paths []string
	if err := json.Unmarshal([]byte(pc.IgnorePaths), &paths); err != nil {
		return nil
	}
	return paths
}
