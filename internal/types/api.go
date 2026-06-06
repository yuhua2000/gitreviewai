package types

// PaginatedData 通用分页响应
type PaginatedData struct {
	Data     any `json:"data"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// MRDetailData MR 详情响应
type MRDetailData struct {
	ID           int64      `json:"id"`
	ProjectID    string     `json:"project_id"`
	MRIID        int        `json:"mr_iid"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	SourceBranch string     `json:"source_branch"`
	TargetBranch string     `json:"target_branch"`
	State        string     `json:"state"`
	WebURL       string     `json:"web_url"`
	ReviewStatus string     `json:"review_status"`
	CreatedAt    string     `json:"created_at"`
	UpdatedAt    string     `json:"updated_at"`
	Comments     []*Comment `json:"comments"`
	Reports      []*Report  `json:"reports"`
}

// SubmitResult 单条提交结果
type SubmitResult struct {
	ID      int64  `json:"id"`
	Status  string `json:"status"`
	Warning string `json:"warning,omitempty"`
}

// SubmitAllResult 批量提交结果
type SubmitAllResult struct {
	SubmittedComments int      `json:"submitted_comments"`
	SubmittedReports  int      `json:"submitted_reports"`
	Errors            []string `json:"errors"`
}

// SettingsData 设置响应
type SettingsData struct {
	GitLabURL       string   `json:"gitlab_url"`
	GitLabToken     string   `json:"gitlab_token"` // masked
	IgnorePaths     []string `json:"ignore_paths"`
	MaxLineComments int      `json:"max_line_comments"`
	AutoSubmit      bool     `json:"auto_submit"`
	WebhookToken    string   `json:"webhook_token"` // masked
	LogLevel        string   `json:"log_level"`
	JWTExpiry       string   `json:"jwt_expiry"`
}

// LoginData 登录响应
type LoginData struct {
	Token string `json:"token"`
}

// UpdateSettingsRequest 更新设置请求
type UpdateSettingsRequest struct {
	GitLabURL       *string  `json:"gitlab_url"`
	GitLabToken     *string  `json:"gitlab_token"`
	IgnorePaths     []string `json:"ignore_paths"`
	MaxLineComments *int     `json:"max_line_comments"`
	AutoSubmit      *bool    `json:"auto_submit"`
	WebhookToken    *string  `json:"webhook_token"`
	LogLevel        *string  `json:"log_level"`
	JWTExpiry       *string  `json:"jwt_expiry"`
}

// GlobalSettings 全局业务配置聚合
type GlobalSettings struct {
	GitLabURL       string   `json:"gitlab_url"`
	GitLabToken     string   `json:"gitlab_token"`
	IgnorePaths     []string `json:"ignore_paths"`
	MaxLineComments int      `json:"max_line_comments"`
	AutoSubmit      bool     `json:"auto_submit"`
}
