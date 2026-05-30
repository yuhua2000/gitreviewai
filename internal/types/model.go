package types

import "time"

// MergeRequest 合并请求
type MergeRequest struct {
	ID           int64     `json:"id"`
	ProjectID    string    `json:"project_id"`
	MRIID        int       `json:"mr_iid"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	SourceBranch string    `json:"source_branch"`
	TargetBranch string    `json:"target_branch"`
	State        string    `json:"state"`
	WebURL       string    `json:"web_url"`
	ReviewStatus string    `json:"review_status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Comment 评论
type Comment struct {
	ID                int64      `json:"id"`
	MRID              int64      `json:"mr_id"`
	CommentType       string     `json:"comment_type"` // "line" or "review"
	FilePath          string     `json:"file_path"`
	LineNumber        int        `json:"line_number"`
	Content           string     `json:"content"`
	DiffContext       string     `json:"diff_context"`
	Status            string     `json:"status"` // "pending" or "submitted"
	GitlabNoteID      *int64     `json:"gitlab_note_id"`
	GitlabDraftNoteID *int64     `json:"gitlab_draft_note_id"`
	SubmittedAt       *time.Time `json:"submitted_at"`
	CreatedAt         time.Time  `json:"created_at"`
}

// Report 报告
type Report struct {
	ID           int64      `json:"id"`
	MRID         int64      `json:"mr_id"`
	Content      string     `json:"content"`
	Status       string     `json:"status"` // "pending" or "submitted"
	GitlabNoteID *int64     `json:"gitlab_note_id"`
	SubmittedAt  *time.Time `json:"submitted_at"`
	CreatedAt    time.Time  `json:"created_at"`
}
