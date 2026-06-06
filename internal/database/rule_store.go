package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/yuhua2000/gitreviewai/internal/types"
)

// ReviewRuleStore manages review rules (builtin + custom).
type ReviewRuleStore struct {
	db *sql.DB
}

// NewReviewRuleStore creates a new ReviewRuleStore.
func NewReviewRuleStore(db *sql.DB) *ReviewRuleStore {
	return &ReviewRuleStore{db: db}
}

// List returns all review rules.
func (s *ReviewRuleStore) List(ctx context.Context) ([]*types.ReviewRule, error) {
	query := `SELECT id, rule_id, name, description, severity, is_builtin, enabled, created_at, updated_at
		FROM review_rules ORDER BY CASE severity WHEN 'error' THEN 1 WHEN 'warning' THEN 2 ELSE 3 END, id ASC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list review_rules: %w", err)
	}
	defer rows.Close()

	var rules []*types.ReviewRule
	for rows.Next() {
		r := &types.ReviewRule{}
		if err := rows.Scan(&r.ID, &r.RuleID, &r.Name, &r.Description, &r.Severity, &r.IsBuiltin, &r.Enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan review_rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// GetByID retrieves a review rule by ID.
func (s *ReviewRuleStore) GetByID(ctx context.Context, id int64) (*types.ReviewRule, error) {
	query := `SELECT id, rule_id, name, description, severity, is_builtin, enabled, created_at, updated_at
		FROM review_rules WHERE id=?`
	r := &types.ReviewRule{}
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&r.ID, &r.RuleID, &r.Name, &r.Description, &r.Severity, &r.IsBuiltin, &r.Enabled, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get review_rule by id: %w", err)
	}
	return r, nil
}

// Create inserts a new custom review rule.
func (s *ReviewRuleStore) Create(ctx context.Context, r *types.ReviewRule) (int64, error) {
	query := `INSERT INTO review_rules (rule_id, name, description, severity, is_builtin, enabled)
		VALUES (?, ?, ?, ?, 0, ?)`
	result, err := s.db.ExecContext(ctx, query, r.RuleID, r.Name, r.Description, r.Severity, r.Enabled)
	if err != nil {
		return 0, fmt.Errorf("insert review_rule: %w", err)
	}
	return result.LastInsertId()
}

// Update modifies a review rule.
func (s *ReviewRuleStore) Update(ctx context.Context, r *types.ReviewRule) error {
	query := `UPDATE review_rules SET name=?, description=?, severity=?, enabled=?, updated_at=datetime('now')
		WHERE id=?`
	_, err := s.db.ExecContext(ctx, query, r.Name, r.Description, r.Severity, r.Enabled, r.ID)
	if err != nil {
		return fmt.Errorf("update review_rule: %w", err)
	}
	return nil
}

// Delete removes a review rule. Only custom rules (is_builtin=0) can be deleted.
func (s *ReviewRuleStore) Delete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM review_rules WHERE id=? AND is_builtin=0`, id)
	if err != nil {
		return fmt.Errorf("delete review_rule: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("rule not found or is builtin")
	}
	return nil
}

// Toggle enables or disables a rule.
func (s *ReviewRuleStore) Toggle(ctx context.Context, id int64, enabled bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE review_rules SET enabled=?, updated_at=datetime('now') WHERE id=?`, enabled, id)
	if err != nil {
		return fmt.Errorf("toggle review_rule: %w", err)
	}
	return nil
}

// GetEnabledRules returns all globally enabled rules.
func (s *ReviewRuleStore) GetEnabledRules(ctx context.Context) ([]*types.ReviewRule, error) {
	query := `SELECT id, rule_id, name, description, severity, is_builtin, enabled, created_at, updated_at
		FROM review_rules WHERE enabled=1 ORDER BY CASE severity WHEN 'error' THEN 1 WHEN 'warning' THEN 2 ELSE 3 END, id ASC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get enabled rules: %w", err)
	}
	defer rows.Close()

	var rules []*types.ReviewRule
	for rows.Next() {
		r := &types.ReviewRule{}
		if err := rows.Scan(&r.ID, &r.RuleID, &r.Name, &r.Description, &r.Severity, &r.IsBuiltin, &r.Enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan review_rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// GetEnabledRulesForProject returns enabled rules considering project overrides.
func (s *ReviewRuleStore) GetEnabledRulesForProject(ctx context.Context, projectConfigID int64) ([]*types.ReviewRule, error) {
	query := `SELECT r.id, r.rule_id, r.name, r.description, r.severity, r.is_builtin, r.enabled, r.created_at, r.updated_at
		FROM review_rules r
		LEFT JOIN project_rule_overrides o ON o.rule_id = r.rule_id AND o.project_config_id = ?
		WHERE COALESCE(o.enabled, r.enabled) = 1
		ORDER BY CASE r.severity WHEN 'error' THEN 1 WHEN 'warning' THEN 2 ELSE 3 END, r.id ASC`
	rows, err := s.db.QueryContext(ctx, query, projectConfigID)
	if err != nil {
		return nil, fmt.Errorf("get enabled rules for project: %w", err)
	}
	defer rows.Close()

	var rules []*types.ReviewRule
	for rows.Next() {
		r := &types.ReviewRule{}
		if err := rows.Scan(&r.ID, &r.RuleID, &r.Name, &r.Description, &r.Severity, &r.IsBuiltin, &r.Enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan review_rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}
