package database

import (
	"context"
	"database/sql"

	"github.com/yuhua2000/gitreviewai/internal/types"
)

type MRStore struct {
	db *sql.DB
}

func NewMRStore(db *sql.DB) *MRStore {
	return &MRStore{db: db}
}

func (s *MRStore) Upsert(ctx context.Context, mr *types.MergeRequest) (int64, error) {
	query := `INSERT INTO merge_requests (project_id, mr_iid, title, description, source_branch, target_branch, state, web_url, review_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, mr_iid) DO UPDATE SET
			title = excluded.title,
			description = excluded.description,
			source_branch = excluded.source_branch,
			target_branch = excluded.target_branch,
			state = excluded.state,
			web_url = excluded.web_url,
			review_status = excluded.review_status,
			updated_at = datetime('now')
		RETURNING id`

	var id int64
	err := s.db.QueryRowContext(ctx, query,
		mr.ProjectID, mr.MRIID, mr.Title, mr.Description,
		mr.SourceBranch, mr.TargetBranch, mr.State, mr.WebURL, mr.ReviewStatus,
	).Scan(&id)
	return id, err
}

func (s *MRStore) GetByID(ctx context.Context, id int64) (*types.MergeRequest, error) {
	query := `SELECT id, project_id, mr_iid, title, description, source_branch, target_branch,
		state, web_url, review_status, error_message, created_at, updated_at
		FROM merge_requests WHERE id = ?`
	return s.scanMR(s.db.QueryRowContext(ctx, query, id))
}

func (s *MRStore) GetByProjectAndIID(ctx context.Context, projectID string, mrIID int) (*types.MergeRequest, error) {
	query := `SELECT id, project_id, mr_iid, title, description, source_branch, target_branch,
		state, web_url, review_status, error_message, created_at, updated_at
		FROM merge_requests WHERE project_id = ? AND mr_iid = ?`
	return s.scanMR(s.db.QueryRowContext(ctx, query, projectID, mrIID))
}

func (s *MRStore) List(ctx context.Context, offset, limit int) ([]*types.MergeRequest, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM merge_requests`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, mr_iid, title, description, source_branch, target_branch,
		state, web_url, review_status, error_message, created_at, updated_at
		FROM merge_requests ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var mrs []*types.MergeRequest
	for rows.Next() {
		mr, err := s.scanMRRow(rows)
		if err != nil {
			return nil, 0, err
		}
		mrs = append(mrs, mr)
	}
	return mrs, total, rows.Err()
}

// UpdateReviewStatus updates the review status and optionally the error message.
func (s *MRStore) UpdateReviewStatus(ctx context.Context, id int64, status string, errorMsg string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE merge_requests SET review_status = ?, error_message = ?, updated_at = datetime('now') WHERE id = ?`,
		status, errorMsg, id)
	return err
}

func (s *MRStore) scanMR(row *sql.Row) (*types.MergeRequest, error) {
	mr := &types.MergeRequest{}
	err := row.Scan(&mr.ID, &mr.ProjectID, &mr.MRIID, &mr.Title, &mr.Description,
		&mr.SourceBranch, &mr.TargetBranch, &mr.State, &mr.WebURL, &mr.ReviewStatus,
		&mr.ErrorMessage, &mr.CreatedAt, &mr.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return mr, nil
}

func (s *MRStore) scanMRRow(rows *sql.Rows) (*types.MergeRequest, error) {
	mr := &types.MergeRequest{}
	err := rows.Scan(&mr.ID, &mr.ProjectID, &mr.MRIID, &mr.Title, &mr.Description,
		&mr.SourceBranch, &mr.TargetBranch, &mr.State, &mr.WebURL, &mr.ReviewStatus,
		&mr.ErrorMessage, &mr.CreatedAt, &mr.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return mr, nil
}
