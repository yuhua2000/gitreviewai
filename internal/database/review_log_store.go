package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/yuhua2000/gitreviewai/internal/types"
)

// ReviewLogStore manages review audit logs.
type ReviewLogStore struct {
	db *sql.DB
}

// NewReviewLogStore creates a new ReviewLogStore.
func NewReviewLogStore(db *sql.DB) *ReviewLogStore {
	return &ReviewLogStore{db: db}
}

// Create inserts a new review log and returns its ID.
func (s *ReviewLogStore) Create(ctx context.Context, log *types.ReviewLog) (int64, error) {
	query := `INSERT INTO review_logs (mr_id, status, model_name, rules_count)
		VALUES (?, ?, ?, ?)`
	result, err := s.db.ExecContext(ctx, query, log.MRID, log.Status, log.ModelName, log.RulesCount)
	if err != nil {
		return 0, fmt.Errorf("insert review_log: %w", err)
	}
	return result.LastInsertId()
}

// UpdateResult updates a review log with the final result.
func (s *ReviewLogStore) UpdateResult(ctx context.Context, id int64, result *types.ReviewLog) error {
	query := `UPDATE review_logs SET status=?, error_message=?, comments_count=?,
		prompt_tokens=?, completion_tokens=?, total_tokens=?, duration_ms=?
		WHERE id=?`
	_, err := s.db.ExecContext(ctx, query,
		result.Status, result.ErrorMessage, result.CommentsCount,
		result.PromptTokens, result.CompletionTokens, result.TotalTokens, result.DurationMs,
		id,
	)
	if err != nil {
		return fmt.Errorf("update review_log: %w", err)
	}
	return nil
}

// ListByMRID returns all review logs for a given MR, newest first.
func (s *ReviewLogStore) ListByMRID(ctx context.Context, mrID int64) ([]*types.ReviewLog, error) {
	query := `SELECT id, mr_id, status, error_message, model_name, rules_count,
		comments_count, prompt_tokens, completion_tokens, total_tokens, duration_ms, created_at
		FROM review_logs WHERE mr_id=? ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, query, mrID)
	if err != nil {
		return nil, fmt.Errorf("list review_logs: %w", err)
	}
	defer rows.Close()

	var logs []*types.ReviewLog
	for rows.Next() {
		l := &types.ReviewLog{}
		if err := rows.Scan(&l.ID, &l.MRID, &l.Status, &l.ErrorMessage, &l.ModelName,
			&l.RulesCount, &l.CommentsCount, &l.PromptTokens, &l.CompletionTokens,
			&l.TotalTokens, &l.DurationMs, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan review_log: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
