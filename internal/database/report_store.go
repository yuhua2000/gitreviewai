package database

import (
	"context"
	"database/sql"
	"time"
)

type Report struct {
	ID           int64      `json:"id"`
	MRID         int64      `json:"mr_id"`
	Content      string     `json:"content"`
	Status       string     `json:"status"` // "pending" or "submitted"
	GitlabNoteID *int64     `json:"gitlab_note_id"`
	SubmittedAt  *time.Time `json:"submitted_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

type ReportStore struct {
	db *sql.DB
}

func NewReportStore(db *sql.DB) *ReportStore {
	return &ReportStore{db: db}
}

func (s *ReportStore) Create(ctx context.Context, r *Report) (int64, error) {
	query := `INSERT INTO reports (mr_id, content, status) VALUES (?, ?, ?)`
	result, err := s.db.ExecContext(ctx, query, r.MRID, r.Content, r.Status)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *ReportStore) ListByMRID(ctx context.Context, mrID int64) ([]*Report, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, mr_id, content, status, gitlab_note_id, submitted_at, created_at
		FROM reports WHERE mr_id = ? ORDER BY id`, mrID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []*Report
	for rows.Next() {
		r := &Report{}
		err := rows.Scan(&r.ID, &r.MRID, &r.Content, &r.Status, &r.GitlabNoteID, &r.SubmittedAt, &r.CreatedAt)
		if err != nil {
			return nil, err
		}
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

func (s *ReportStore) GetByID(ctx context.Context, id int64) (*Report, error) {
	r := &Report{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, mr_id, content, status, gitlab_note_id, submitted_at, created_at
		FROM reports WHERE id = ?`, id,
	).Scan(&r.ID, &r.MRID, &r.Content, &r.Status, &r.GitlabNoteID, &r.SubmittedAt, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (s *ReportStore) UpdateStatus(ctx context.Context, id int64, status string, noteID *int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE reports SET status = ?, gitlab_note_id = ?, submitted_at = datetime('now') WHERE id = ?`,
		status, noteID, id)
	return err
}
