package database

import (
	"context"
	"database/sql"

	"github.com/yuhua2000/gitreviewai/internal/types"
)

type CommentStore struct {
	db *sql.DB
}

func NewCommentStore(db *sql.DB) *CommentStore {
	return &CommentStore{db: db}
}

func (s *CommentStore) Create(ctx context.Context, c *types.Comment) (int64, error) {
	query := `INSERT INTO comments (mr_id, comment_type, file_path, line_number, content, diff_context, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	result, err := s.db.ExecContext(ctx, query,
		c.MRID, c.CommentType, c.FilePath, c.LineNumber, c.Content, c.DiffContext, c.Status)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *CommentStore) CreateBatch(ctx context.Context, comments []*types.Comment) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO comments (mr_id, comment_type, file_path, line_number, content, diff_context, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range comments {
		result, err := stmt.ExecContext(ctx,
			c.MRID, c.CommentType, c.FilePath, c.LineNumber, c.Content, c.DiffContext, c.Status)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		c.ID = id
	}

	return tx.Commit()
}

func (s *CommentStore) ListByMRID(ctx context.Context, mrID int64) ([]*types.Comment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, mr_id, comment_type, file_path, line_number, content, diff_context, status, gitlab_note_id, gitlab_draft_note_id, submitted_at, created_at
		FROM comments WHERE mr_id = ? ORDER BY id`, mrID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []*types.Comment
	for rows.Next() {
		c := &types.Comment{}
		err := rows.Scan(&c.ID, &c.MRID, &c.CommentType, &c.FilePath, &c.LineNumber,
			&c.Content, &c.DiffContext, &c.Status, &c.GitlabNoteID, &c.GitlabDraftNoteID, &c.SubmittedAt, &c.CreatedAt)
		if err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

func (s *CommentStore) GetByID(ctx context.Context, id int64) (*types.Comment, error) {
	c := &types.Comment{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, mr_id, comment_type, file_path, line_number, content, diff_context, status, gitlab_note_id, gitlab_draft_note_id, submitted_at, created_at
		FROM comments WHERE id = ?`, id,
	).Scan(&c.ID, &c.MRID, &c.CommentType, &c.FilePath, &c.LineNumber,
		&c.Content, &c.DiffContext, &c.Status, &c.GitlabNoteID, &c.GitlabDraftNoteID, &c.SubmittedAt, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *CommentStore) UpdateStatus(ctx context.Context, id int64, status string, noteID *int64, draftNoteID *int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE comments SET status = ?, gitlab_note_id = ?, gitlab_draft_note_id = ?, submitted_at = datetime('now') WHERE id = ?`,
		status, noteID, draftNoteID, id)
	return err
}
