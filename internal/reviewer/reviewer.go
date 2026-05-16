package reviewer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/yuhua2000/gitreviewai/internal/ai"
	"github.com/yuhua2000/gitreviewai/internal/config"
	"github.com/yuhua2000/gitreviewai/internal/database"
	"github.com/yuhua2000/gitreviewai/internal/gitlab"
)

type Reviewer struct {
	cfg          *config.Config
	glClient     *gitlab.Client
	aiClient     *ai.Client
	mrStore      *database.MRStore
	commentStore *database.CommentStore
	reportStore  *database.ReportStore
	settingStore *database.SettingStore
}

type reviewState struct {
	projectID      string
	sourceBranch   string
	changes        []gitlab.MRChange
	changesSent    int
	lineComments   []ai.LineCommentResult
	reviewComments []string
	report         string
	finished       bool
}

func New(cfg *config.Config, db *sql.DB) *Reviewer {
	return &Reviewer{
		cfg:          cfg,
		glClient:     gitlab.NewClient(cfg.GitLabURL, cfg.GitLabToken),
		aiClient:     ai.NewClient(cfg.OpenAIAPIKey, cfg.OpenAIModel, cfg.OpenAIBaseURL),
		mrStore:      database.NewMRStore(db),
		commentStore: database.NewCommentStore(db),
		reportStore:  database.NewReportStore(db),
		settingStore: database.NewSettingStore(db),
	}
}

func (r *Reviewer) ReviewMR(ctx context.Context, projectID string, mrIID int) error {
	slog.Info("review started", "project", projectID, "mr_iid", mrIID)

	// 1. Get MR info
	mrInfo, err := r.glClient.GetMRInfo(ctx, projectID, mrIID)
	if err != nil {
		return fmt.Errorf("failed to get MR info: %w", err)
	}
	slog.Info("MR info retrieved", "title", mrInfo.Title, "state", mrInfo.State)

	// 2. Upsert MR into database
	mrRecord := &database.MergeRequest{
		ProjectID:    projectID,
		MRIID:        mrIID,
		Title:        mrInfo.Title,
		Description:  mrInfo.Description,
		SourceBranch: mrInfo.SourceBranch,
		TargetBranch: mrInfo.TargetBranch,
		State:        mrInfo.State,
		WebURL:       mrInfo.WebURL,
		ReviewStatus: "reviewing",
	}
	mrID, err := r.mrStore.Upsert(ctx, mrRecord)
	if err != nil {
		slog.Error("failed to upsert MR", "error", err)
		return fmt.Errorf("failed to upsert MR: %w", err)
	}
	slog.Info("MR persisted to database", "mr_id", mrID)

	// 3. Get MR changes
	changes, diffRefs, err := r.glClient.GetMRChanges(ctx, projectID, mrIID)
	if err != nil {
		r.mrStore.UpdateReviewStatus(ctx, mrID, "failed")
		return fmt.Errorf("failed to get MR changes: %w", err)
	}
	slog.Info("found changed files", "count", len(changes))

	// 4. Filter changes
	filteredChanges := r.filterChanges(changes)
	slog.Info("files to review after filtering", "count", len(filteredChanges))

	// 5. Initialize review state
	state := &reviewState{
		projectID:    projectID,
		sourceBranch: mrInfo.SourceBranch,
		changes:      filteredChanges,
	}

	// 6. Prepare initial changes summary
	initialSummary, remaining := r.formatInitialChangesSummary(filteredChanges, 10)

	// 7. Prepare tool handler
	toolHandler := func(name string, args json.RawMessage) (string, error) {
		return r.handleToolCall(ctx, state, name, args)
	}

	// 8. Build initial message
	initialMessage := ai.FormatInitialMessage(
		mrInfo.Title,
		mrInfo.Description,
		mrInfo.SourceBranch,
		mrInfo.TargetBranch,
		initialSummary,
	)

	if remaining > 0 {
		initialMessage += fmt.Sprintf("\n\n> 注意：当前展示前 %d 个文件，还有 %d 个文件未展示。使用 GetMoreChanges 工具查看更多变更。",
			state.changesSent, remaining)
	}

	// 9. Execute AI review
	response, err := r.aiClient.ChatWithLimit(ctx, ai.SystemPrompt(), initialMessage, toolHandler, 50)
	if err != nil {
		r.mrStore.UpdateReviewStatus(ctx, mrID, "failed")
		return fmt.Errorf("AI review failed: %w", err)
	}
	slog.Info("AI review completed", "response_length", len(response))

	// 10. Persist results to database
	autoSubmit, _ := r.settingStore.GetAutoSubmit(ctx)

	lineComments := state.lineComments
	if len(lineComments) > r.cfg.MaxLineComments {
		slog.Warn("line comments exceed limit, truncating", "total", len(lineComments), "max", r.cfg.MaxLineComments)
		lineComments = lineComments[:r.cfg.MaxLineComments]
	}

	// Build diff map for context extraction
	diffMap := make(map[string]string)
	for _, change := range filteredChanges {
		diffMap[change.NewPath] = change.Diff
	}

	// 10a. Persist line comments
	for _, lc := range lineComments {
		diffContext := ExtractDiffContext(diffMap[lc.File], lc.Line, 8)
		comment := &database.Comment{
			MRID:        mrID,
			CommentType: "line",
			FilePath:    lc.File,
			LineNumber:  lc.Line,
			Content:     lc.Message,
			DiffContext: diffContext,
			Status:      "pending",
		}
		commentID, err := r.commentStore.Create(ctx, comment)
		if err != nil {
			slog.Error("failed to create line comment", "error", err)
			continue
		}
		comment.ID = commentID

		if autoSubmit {
			r.submitSingleLineComment(ctx, projectID, mrIID, diffRefs, lc, commentID)
		}
	}

	// 10b. Persist review comments
	for _, rc := range state.reviewComments {
		comment := &database.Comment{
			MRID:        mrID,
			CommentType: "review",
			Content:     rc,
			Status:      "pending",
		}
		commentID, err := r.commentStore.Create(ctx, comment)
		if err != nil {
			slog.Error("failed to create review comment", "error", err)
			continue
		}
		comment.ID = commentID

		if autoSubmit {
			r.submitSingleReviewComment(ctx, projectID, mrIID, rc, commentID)
		}
	}

	// 10c. Persist report
	if state.report != "" {
		report := &database.Report{
			MRID:    mrID,
			Content: state.report,
			Status:  "pending",
		}
		reportID, err := r.reportStore.Create(ctx, report)
		if err != nil {
			slog.Error("failed to create report", "error", err)
		} else {
			if autoSubmit {
				r.submitSingleReport(ctx, projectID, mrIID, state.report, reportID)
			}
		}
	}

	// 11. Update MR status
	r.mrStore.UpdateReviewStatus(ctx, mrID, "completed")

	slog.Info("review completed",
		"project", projectID,
		"mr_iid", mrIID,
		"mr_id", mrID,
		"line_comments", len(lineComments),
		"review_comments", len(state.reviewComments),
		"auto_submit", autoSubmit)

	return nil
}

func (r *Reviewer) submitSingleLineComment(ctx context.Context, projectID string, mrIID int,
	diffRefs *gitlab.DiffRefs, comment ai.LineCommentResult, commentID int64) {

	draft := gitlab.DraftNote{
		Note: comment.Message,
		Position: &gitlab.Position{
			BaseSHA:      diffRefs.BaseSHA,
			StartSHA:     diffRefs.StartSHA,
			HeadSHA:      diffRefs.HeadSHA,
			PositionType: "text",
			NewPath:      comment.File,
			OldPath:      comment.File,
			NewLine:      comment.Line,
		},
	}
	draftID, err := r.glClient.CreateDraftNote(ctx, projectID, mrIID, draft)
	if err != nil {
		slog.Error("failed to create draft note", "file", comment.File, "line", comment.Line, "error", err)
		return
	}

	if err := r.glClient.PublishDraftNote(ctx, projectID, mrIID, draftID); err != nil {
		slog.Error("failed to publish draft note", "file", comment.File, "line", comment.Line, "draft_id", draftID, "error", err)
		return
	}

	draftID64 := int64(draftID)
	r.commentStore.UpdateStatus(ctx, commentID, "submitted", nil, &draftID64)
	slog.Info("line comment submitted", "file", comment.File, "line", comment.Line, "draft_id", draftID)
}

func (r *Reviewer) submitSingleReviewComment(ctx context.Context, projectID string, mrIID int,
	content string, commentID int64) {

	noteID, err := r.glClient.PostMRNote(ctx, projectID, mrIID, content)
	if err != nil {
		slog.Error("failed to post review comment", "error", err)
		return
	}

	noteID64 := int64(noteID)
	r.commentStore.UpdateStatus(ctx, commentID, "submitted", &noteID64, nil)
	slog.Info("review comment submitted", "note_id", noteID)
}

func (r *Reviewer) submitSingleReport(ctx context.Context, projectID string, mrIID int,
	content string, reportID int64) {

	reportBody := fmt.Sprintf(`# MR 审核报告

**审核时间:** %s

---

%s

---
*此报告由 GitReviewAI 自动生成，供开发者参考。*`,
		time.Now().Format("2006-01-02 15:04:05"),
		content)

	noteID, err := r.glClient.PostMRNote(ctx, projectID, mrIID, reportBody)
	if err != nil {
		slog.Error("failed to post report", "error", err)
		return
	}

	noteID64 := int64(noteID)
	r.reportStore.UpdateStatus(ctx, reportID, "submitted", &noteID64)
	slog.Info("report submitted", "note_id", noteID)
}

// GetStores returns the database stores for use by API handlers
func (r *Reviewer) GetStores() (*database.MRStore, *database.CommentStore, *database.ReportStore, *database.SettingStore) {
	return r.mrStore, r.commentStore, r.reportStore, r.settingStore
}

// GetGitLabClient returns the GitLab client for use by API handlers
func (r *Reviewer) GetGitLabClient() *gitlab.Client {
	return r.glClient
}

// filterChanges intelligently filters changed files
func (r *Reviewer) filterChanges(changes []gitlab.MRChange) []gitlab.MRChange {
	stats := r.analyzeChanges(changes)
	slog.Info("file analysis",
		"total", len(changes),
		"business", stats.Business,
		"test", stats.Test,
		"config", stats.Config,
		"generated", stats.Generated,
		"docs", stats.Docs)

	if len(changes) <= 20 {
		return r.filterByIgnorePaths(changes)
	}

	var filtered []gitlab.MRChange
	for _, change := range changes {
		if r.shouldIgnoreByConfig(change.NewPath) {
			continue
		}

		fileType := r.classifyFile(change.NewPath)
		switch fileType {
		case "business":
			filtered = append(filtered, change)
		case "test":
			if stats.Test <= 10 {
				filtered = append(filtered, change)
			}
		case "config":
			if stats.Config <= 5 {
				filtered = append(filtered, change)
			}
		case "generated":
			if stats.Generated <= 3 {
				filtered = append(filtered, change)
			}
		case "docs":
			if strings.Contains(change.NewPath, "api") || strings.Contains(change.NewPath, "swagger") {
				filtered = append(filtered, change)
			}
		default:
			filtered = append(filtered, change)
		}
	}

	if len(filtered) < 5 {
		return r.filterByIgnorePaths(changes)
	}

	return filtered
}

type ChangeStats struct {
	Business  int
	Test      int
	Config    int
	Generated int
	Docs      int
	Other     int
}

func (r *Reviewer) analyzeChanges(changes []gitlab.MRChange) ChangeStats {
	stats := ChangeStats{}
	for _, change := range changes {
		switch r.classifyFile(change.NewPath) {
		case "business":
			stats.Business++
		case "test":
			stats.Test++
		case "config":
			stats.Config++
		case "generated":
			stats.Generated++
		case "docs":
			stats.Docs++
		default:
			stats.Other++
		}
	}
	return stats
}

func (r *Reviewer) classifyFile(path string) string {
	lowerPath := strings.ToLower(path)

	if strings.HasSuffix(lowerPath, "_test.go") ||
		strings.HasSuffix(lowerPath, ".test.js") ||
		strings.HasSuffix(lowerPath, ".test.ts") ||
		strings.HasSuffix(lowerPath, ".test.tsx") ||
		strings.HasSuffix(lowerPath, ".spec.js") ||
		strings.HasSuffix(lowerPath, ".spec.ts") ||
		strings.Contains(lowerPath, "/test/") ||
		strings.Contains(lowerPath, "/tests/") ||
		strings.Contains(lowerPath, "__tests__/") {
		return "test"
	}

	if strings.HasSuffix(lowerPath, ".pb.go") ||
		strings.HasSuffix(lowerPath, ".pb.gw.go") ||
		strings.HasSuffix(lowerPath, "_grpc.pb.go") ||
		strings.HasSuffix(lowerPath, ".swagger.json") ||
		strings.Contains(lowerPath, "/mock/") ||
		strings.Contains(lowerPath, "/mocks/") ||
		strings.Contains(lowerPath, "generated") ||
		strings.Contains(lowerPath, "auto_generated") {
		return "generated"
	}

	if strings.HasSuffix(lowerPath, ".yaml") ||
		strings.HasSuffix(lowerPath, ".yml") ||
		strings.HasSuffix(lowerPath, ".json") ||
		strings.HasSuffix(lowerPath, ".toml") ||
		strings.HasSuffix(lowerPath, ".ini") ||
		strings.HasSuffix(lowerPath, ".conf") ||
		strings.HasSuffix(lowerPath, ".env") ||
		strings.HasSuffix(lowerPath, ".env.example") ||
		strings.HasSuffix(lowerPath, "go.mod") ||
		strings.HasSuffix(lowerPath, "go.sum") ||
		strings.HasSuffix(lowerPath, "package.json") ||
		strings.HasSuffix(lowerPath, "package-lock.json") ||
		strings.HasSuffix(lowerPath, "yarn.lock") ||
		strings.HasSuffix(lowerPath, "Makefile") ||
		strings.HasSuffix(lowerPath, "Dockerfile") {
		return "config"
	}

	if strings.HasSuffix(lowerPath, ".md") ||
		strings.HasSuffix(lowerPath, ".rst") ||
		strings.HasSuffix(lowerPath, ".txt") ||
		strings.HasSuffix(lowerPath, ".doc") ||
		strings.HasSuffix(lowerPath, ".pdf") {
		return "docs"
	}

	if strings.HasSuffix(lowerPath, ".css") ||
		strings.HasSuffix(lowerPath, ".scss") ||
		strings.HasSuffix(lowerPath, ".less") ||
		strings.HasSuffix(lowerPath, ".svg") ||
		strings.HasSuffix(lowerPath, ".png") ||
		strings.HasSuffix(lowerPath, ".jpg") ||
		strings.HasSuffix(lowerPath, ".gif") ||
		strings.HasSuffix(lowerPath, ".ico") {
		return "resource"
	}

	return "business"
}

func (r *Reviewer) filterByIgnorePaths(changes []gitlab.MRChange) []gitlab.MRChange {
	var filtered []gitlab.MRChange
	for _, change := range changes {
		if !r.shouldIgnoreByConfig(change.NewPath) {
			filtered = append(filtered, change)
		}
	}
	return filtered
}

func (r *Reviewer) shouldIgnoreByConfig(path string) bool {
	for _, ignorePath := range r.cfg.IgnorePaths {
		if strings.HasPrefix(path, ignorePath+"/") || path == ignorePath {
			return true
		}
	}
	return false
}

func (r *Reviewer) formatInitialChangesSummary(changes []gitlab.MRChange, maxFiles int) (string, int) {
	var sb strings.Builder
	count := 0
	remaining := 0

	for i, change := range changes {
		if count >= maxFiles {
			remaining = len(changes) - i
			break
		}

		status := "modified"
		if change.NewFile {
			status = "new"
		} else if change.DeletedFile {
			status = "deleted"
		} else if change.RenamedFile {
			status = "renamed"
		}
		sb.WriteString(fmt.Sprintf("### %s (%s)\n", change.NewPath, status))

		diff := change.Diff
		if len(diff) > 3000 {
			diff = diff[:3000] + "\n... (diff 已截断，请使用 ReadFile 查看完整内容)"
		}
		sb.WriteString("```diff\n")
		sb.WriteString(diff)
		sb.WriteString("\n```\n\n")

		count++
	}

	return sb.String(), remaining
}

func (r *Reviewer) formatMoreChanges(state *reviewState, batchSize int) (string, int) {
	var sb strings.Builder
	count := 0
	remaining := 0
	start := state.changesSent

	for i := start; i < len(state.changes); i++ {
		if count >= batchSize {
			remaining = len(state.changes) - i
			break
		}

		change := state.changes[i]
		status := "modified"
		if change.NewFile {
			status = "new"
		} else if change.DeletedFile {
			status = "deleted"
		} else if change.RenamedFile {
			status = "renamed"
		}
		sb.WriteString(fmt.Sprintf("### %s (%s)\n", change.NewPath, status))

		diff := change.Diff
		if len(diff) > 3000 {
			diff = diff[:3000] + "\n... (diff 已截断，请使用 ReadFile 查看完整内容)"
		}
		sb.WriteString("```diff\n")
		sb.WriteString(diff)
		sb.WriteString("\n```\n\n")

		count++
		state.changesSent = i + 1
	}

	return sb.String(), remaining
}

func (r *Reviewer) handleToolCall(ctx context.Context, state *reviewState,
	name string, args json.RawMessage) (string, error) {

	slog.Debug("tool call received", "tool", name, "args", string(args))

	switch name {
	case "FinishReview":
		state.finished = true
		slog.Info("FinishReview called")
		return "审核已完成，感谢你的工作！", nil

	case "GetMoreChanges":
		if state.changesSent >= len(state.changes) {
			slog.Debug("all changes already sent")
			return "所有变更文件已经展示完毕，没有更多内容了。", nil
		}

		moreContent, remaining := r.formatMoreChanges(state, 10)
		result := fmt.Sprintf("以下是更多变更文件（已展示 %d/%d 个文件）：\n\n%s",
			state.changesSent, len(state.changes), moreContent)

		if remaining > 0 {
			result += fmt.Sprintf("\n> 还有 %d 个文件未展示，如需继续查看请再次调用 GetMoreChanges。", remaining)
		} else {
			result += "\n> 所有变更文件已展示完毕。"
		}

		slog.Debug("GetMoreChanges", "sent", state.changesSent, "total", len(state.changes), "remaining", remaining)
		return result, nil

	case "ReadFile":
		var params struct {
			Path      string `json:"path"`
			StartLine int    `json:"start_line,omitempty"`
			EndLine   int    `json:"end_line,omitempty"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("解析参数失败: %w", err)
		}
		slog.Debug("ReadFile", "path", params.Path, "start_line", params.StartLine, "end_line", params.EndLine)
		content, err := r.glClient.GetFileContent(ctx, state.projectID, state.sourceBranch, params.Path)
		if err != nil {
			slog.Warn("ReadFile failed", "path", params.Path, "error", err)
			return fmt.Sprintf("读取文件失败: %v", err), nil
		}

		lines := strings.Split(content, "\n")
		totalLines := len(lines)

		if params.StartLine > 0 || params.EndLine > 0 {
			start := params.StartLine - 1
			if start < 0 {
				start = 0
			}
			if start >= totalLines {
				return "指定行范围超出文件长度", nil
			}

			end := params.EndLine
			if end <= 0 || end > totalLines {
				end = totalLines
			}
			if end <= start {
				return "无效行范围", nil
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("📄 文件: %s\n📍 行号: %d-%d (共 %d 行)\n---\n", params.Path, start+1, end, totalLines))
			for i := start; i < end && i < totalLines; i++ {
				if i > start {
					sb.WriteString("\n")
				}
				sb.WriteString(lines[i])
			}
			sb.WriteString("\n---")
			content = sb.String()
		} else {
			content = fmt.Sprintf("📄 文件: %s\n📍 共 %d 行\n---\n%s\n---", params.Path, totalLines, content)
		}

		if len(content) > 15000 {
			content = content[:15000] + "\n\n... (内容已截断，文件过大)"
		}
		return content, nil

	case "FindInFile":
		var params struct {
			Path    string `json:"path"`
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("解析参数失败: %w", err)
		}
		slog.Debug("FindInFile", "path", params.Path, "pattern", params.Pattern)
		content, err := r.glClient.GetFileContent(ctx, state.projectID, state.sourceBranch, params.Path)
		if err != nil {
			slog.Warn("FindInFile failed", "path", params.Path, "error", err)
			return fmt.Sprintf("读取文件失败: %v", err), nil
		}
		var matches []string
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.Contains(line, params.Pattern) {
				matches = append(matches, fmt.Sprintf("第 %d 行: %s", i+1, strings.TrimSpace(line)))
			}
		}
		if len(matches) == 0 {
			return "未找到匹配内容", nil
		}
		if len(matches) > 30 {
			matches = matches[:30]
			matches = append(matches, "... (仅显示前 30 个匹配结果)")
		}
		return strings.Join(matches, "\n"), nil

	case "GetURL":
		var params struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("解析参数失败: %w", err)
		}
		slog.Debug("GetURL", "url", params.URL)
		client := &http.Client{Timeout: 15 * time.Second}
		req, err := http.NewRequestWithContext(ctx, "GET", params.URL, nil)
		if err != nil {
			return fmt.Sprintf("Failed to create request: %v", err), nil
		}
		resp, err := client.Do(req)
		if err != nil {
			slog.Warn("GetURL failed", "url", params.URL, "error", err)
			return fmt.Sprintf("获取 URL 失败: %v", err), nil
		}
		defer resp.Body.Close()
		buf := make([]byte, 8000)
		n, _ := resp.Body.Read(buf)
		return string(buf[:n]), nil

	case "AddLineComment":
		var comment ai.LineCommentResult
		if err := json.Unmarshal(args, &comment); err != nil {
			return "", fmt.Errorf("解析参数失败: %w", err)
		}
		state.lineComments = append(state.lineComments, comment)
		slog.Info("line comment added", "file", comment.File, "line", comment.Line)
		return fmt.Sprintf("已添加行级评论: %s 第 %d 行", comment.File, comment.Line), nil

	case "AddReviewComment":
		var comment ai.ReviewCommentResult
		if err := json.Unmarshal(args, &comment); err != nil {
			return "", fmt.Errorf("解析参数失败: %w", err)
		}
		state.reviewComments = append(state.reviewComments, comment.Message)
		slog.Info("review comment added")
		return "已添加整体审核意见", nil

	case "GenerateMDReport":
		var params struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return "", fmt.Errorf("解析参数失败: %w", err)
		}
		state.report = params.Content
		slog.Info("MD report generated", "length", len(params.Content))
		return "报告已记录。", nil

	default:
		slog.Warn("unknown tool", "name", name)
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}
