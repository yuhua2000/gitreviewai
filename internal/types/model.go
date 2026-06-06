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
	ErrorMessage string    `json:"error_message"`
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

// AIModel AI 模型配置
type AIModel struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	BaseURL   string    `json:"base_url"`
	APIKey    string    `json:"api_key"`
	ModelName string    `json:"model_name"`
	IsDefault bool      `json:"is_default"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ReviewRule 审计规则
type ReviewRule struct {
	ID          int64     `json:"id"`
	RuleID      string    `json:"rule_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"` // error, warning, info
	IsBuiltin   bool      `json:"is_builtin"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ProjectConfig 项目配置
type ProjectConfig struct {
	ID              int64     `json:"id"`
	GitlabProjectID int       `json:"gitlab_project_id"`
	ProjectName     string    `json:"project_name"`
	Description     string    `json:"description"`
	AIModelID       *int64    `json:"ai_model_id"`
	AutoSubmit      bool      `json:"auto_submit"`
	SkipDraft       bool      `json:"skip_draft"`
	TargetBranches  string    `json:"target_branches"` // JSON array
	IgnorePaths     string    `json:"ignore_paths"`    // JSON array
	MaxLineComments *int      `json:"max_line_comments"`
	CustomPrompt    string    `json:"custom_prompt"`
	Enabled         bool      `json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ProjectRuleOverride 项目规则覆盖
type ProjectRuleOverride struct {
	ID              int64  `json:"id"`
	ProjectConfigID int64  `json:"project_config_id"`
	RuleID          string `json:"rule_id"`
	Enabled         bool   `json:"enabled"`
}

// ReviewLog 审计历史记录
type ReviewLog struct {
	ID               int64     `json:"id"`
	MRID             int64     `json:"mr_id"`
	Status           string    `json:"status"` // running, success, failed
	ErrorMessage     string    `json:"error_message"`
	ModelName        string    `json:"model_name"`
	RulesCount       int       `json:"rules_count"`
	CommentsCount    int       `json:"comments_count"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	DurationMs       int64     `json:"duration_ms"`
	CreatedAt        time.Time `json:"created_at"`
}
