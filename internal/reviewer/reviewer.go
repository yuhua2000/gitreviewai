package reviewer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/yuhua2000/gitreviewai/internal/ai"
	"github.com/yuhua2000/gitreviewai/internal/config"
	"github.com/yuhua2000/gitreviewai/internal/gitlab"
)

type Reviewer struct {
	cfg      *config.Config
	glClient *gitlab.Client
	aiClient *ai.Client
}

// reviewState holds the state shared across tool calls during a review
type reviewState struct {
	projectID      string
	sourceBranch   string
	changes        []gitlab.MRChange
	changesSent    int // number of changes sent to AI
	lineComments   []ai.LineCommentResult
	reviewComments []string
	report         string
	finished       bool
}

func New(cfg *config.Config) *Reviewer {
	return &Reviewer{
		cfg:      cfg,
		glClient: gitlab.NewClient(cfg.GitLabURL, cfg.GitLabToken),
		aiClient: ai.NewClient(cfg.OpenAIAPIKey, cfg.OpenAIModel, cfg.OpenAIBaseURL),
	}
}

// ReviewMR reviews a Merge Request
func (r *Reviewer) ReviewMR(ctx context.Context, projectID string, mrIID int) error {
	slog.Info("review started", "project", projectID, "mr_iid", mrIID)

	// 1. Get MR info
	mrInfo, err := r.glClient.GetMRInfo(ctx, projectID, mrIID)
	if err != nil {
		return fmt.Errorf("failed to get MR info: %w", err)
	}
	slog.Info("MR info retrieved", "title", mrInfo.Title, "state", mrInfo.State)

	// 2. Get MR changes
	changes, diffRefs, err := r.glClient.GetMRChanges(ctx, projectID, mrIID)
	if err != nil {
		return fmt.Errorf("failed to get MR changes: %w", err)
	}
	slog.Info("found changed files", "count", len(changes))

	// 3. Filter changes
	filteredChanges := r.filterChanges(changes)
	slog.Info("files to review after filtering", "count", len(filteredChanges))

	// 4. Initialize review state
	state := &reviewState{
		projectID:    projectID,
		sourceBranch: mrInfo.SourceBranch,
		changes:      filteredChanges,
	}

	// 5. Prepare initial changes summary (batched)
	initialSummary, remaining := r.formatInitialChangesSummary(filteredChanges, 10)

	// 6. Prepare tool handler
	toolHandler := func(name string, args json.RawMessage) (string, error) {
		return r.handleToolCall(ctx, state, name, args)
	}

	// 7. Build initial message
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

	// 8. Execute AI review (50 iterations max)
	response, err := r.aiClient.ChatWithLimit(ctx, ai.SystemPrompt(), initialMessage, toolHandler, 50)
	if err != nil {
		return fmt.Errorf("AI review failed: %w", err)
	}
	slog.Info("AI review completed", "response_length", len(response))

	// 9. Submit results to GitLab
	if err := r.submitResults(ctx, projectID, mrIID, diffRefs, state); err != nil {
		return fmt.Errorf("failed to submit results: %w", err)
	}

	slog.Info("review completed",
		"project", projectID,
		"mr_iid", mrIID,
		"line_comments", len(state.lineComments),
		"review_comments", len(state.reviewComments))

	return nil
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

	// 如果文件数量较少，全部保留
	if len(changes) <= 20 {
		return r.filterByIgnorePaths(changes)
	}

	// 文件较多时，优先保留业务代码
	var filtered []gitlab.MRChange
	for _, change := range changes {
		// 检查是否在忽略列表中
		if r.shouldIgnoreByConfig(change.NewPath) {
			continue
		}

		// 根据文件类型决定是否保留
		fileType := r.classifyFile(change.NewPath)
		switch fileType {
		case "business":
			// 业务代码始终保留
			filtered = append(filtered, change)
		case "test":
			// 测试文件只在数量不多时保留
			if stats.Test <= 10 {
				filtered = append(filtered, change)
			}
		case "config":
			// 配置文件只在数量不多时保留
			if stats.Config <= 5 {
				filtered = append(filtered, change)
			}
		case "generated":
			// 自动生成代码通常忽略
			if stats.Generated <= 3 {
				filtered = append(filtered, change)
			}
		case "docs":
			// 文档类文件忽略（非代码）
			// 但保留 API 文档
			if strings.Contains(change.NewPath, "api") || strings.Contains(change.NewPath, "swagger") {
				filtered = append(filtered, change)
			}
		default:
			// 其他文件保留
			filtered = append(filtered, change)
		}
	}

	// 如果过滤后文件太少，放宽限制
	if len(filtered) < 5 {
		return r.filterByIgnorePaths(changes)
	}

	return filtered
}

// ChangeStats holds file type statistics
type ChangeStats struct {
	Business  int
	Test      int
	Config    int
	Generated int
	Docs      int
	Other     int
}

// analyzeChanges analyzes file type statistics
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

// classifyFile classifies a file by its type
func (r *Reviewer) classifyFile(path string) string {
	lowerPath := strings.ToLower(path)

	// Test files
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

	// Auto-generated code
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

	// Config files
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

	// Documentation files
	if strings.HasSuffix(lowerPath, ".md") ||
		strings.HasSuffix(lowerPath, ".rst") ||
		strings.HasSuffix(lowerPath, ".txt") ||
		strings.HasSuffix(lowerPath, ".doc") ||
		strings.HasSuffix(lowerPath, ".pdf") {
		return "docs"
	}

	// Frontend resources
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

// filterByIgnorePaths filters by configured ignore paths
func (r *Reviewer) filterByIgnorePaths(changes []gitlab.MRChange) []gitlab.MRChange {
	var filtered []gitlab.MRChange
	for _, change := range changes {
		if !r.shouldIgnoreByConfig(change.NewPath) {
			filtered = append(filtered, change)
		}
	}
	return filtered
}

// shouldIgnoreByConfig checks if a path should be ignored based on config
func (r *Reviewer) shouldIgnoreByConfig(path string) bool {
	for _, ignorePath := range r.cfg.IgnorePaths {
		if strings.HasPrefix(path, ignorePath+"/") || path == ignorePath {
			return true
		}
	}
	return false
}

// formatInitialChangesSummary formats initial changes summary, returns summary text and remaining file count
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

// formatMoreChanges formats more changes content
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

		// 截取指定行范围
		if params.StartLine > 0 || params.EndLine > 0 {
			start := params.StartLine - 1 // 1-based to 0-based
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
			// 添加行号描述
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
			// 未指定行范围，显示文件总行数
			content = fmt.Sprintf("📄 文件: %s\n📍 共 %d 行\n---\n%s\n---", params.Path, totalLines, content)
		}

		// 限制返回长度
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
		// 搜索实现
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
		// 使用简单的 HTTP 请求
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
		// 限制返回长度
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

func (r *Reviewer) submitResults(ctx context.Context, projectID string, mrIID int,
	diffRefs *gitlab.DiffRefs, state *reviewState) error {

	// 限制行级评论数量，避免刷屏
	lineComments := state.lineComments
	if len(lineComments) > r.cfg.MaxLineComments {
		slog.Warn("行级评论数量超过限制，将截断", "total", len(lineComments), "max", r.cfg.MaxLineComments)
		lineComments = lineComments[:r.cfg.MaxLineComments]
	}

	// Submit line-level comments as draft notes and publish each one immediately
	for _, comment := range lineComments {
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
			continue
		}
		slog.Debug("draft note created", "file", comment.File, "line", comment.Line, "draft_id", draftID)

		// Publish this draft note individually (PUT /draft_notes/:id/publish)
		if err := r.glClient.PublishDraftNote(ctx, projectID, mrIID, draftID); err != nil {
			slog.Error("failed to publish draft note", "file", comment.File, "line", comment.Line, "draft_id", draftID, "error", err)
		} else {
			slog.Debug("draft note published", "file", comment.File, "line", comment.Line, "draft_id", draftID)
		}
	}

	if len(lineComments) > 0 {
		slog.Info("line comments processed", "count", len(lineComments), "total_generated", len(state.lineComments))
	}

	// Submit review comments
	for _, comment := range state.reviewComments {
		if err := r.glClient.PostMRNote(ctx, projectID, mrIID, comment); err != nil {
			slog.Error("failed to post review comment", "error", err)
		} else {
			slog.Info("review comment posted")
		}
	}

	// Submit Markdown report
	if state.report != "" {
		// 计算实际提交的行评论数量（可能因限制被截断）
		submittedLineComments := len(lineComments)
		if submittedLineComments > r.cfg.MaxLineComments {
			submittedLineComments = r.cfg.MaxLineComments
		}
		generatedLineComments := len(state.lineComments)

		reportBody := fmt.Sprintf(`# MR 审核报告

**审核时间:** %s  
**行级评论:** %d 条（生成 %d 条）  
**整体意见:** %d 条  

---

%s

---
*此报告由 GitReviewAI 自动生成，供开发者参考。*`,
			time.Now().Format("2006-01-02 15:04:05"),
			submittedLineComments,
			generatedLineComments,
			len(state.reviewComments),
			state.report)

		if err := r.glClient.PostMRNote(ctx, projectID, mrIID, reportBody); err != nil {
			slog.Error("failed to post report", "error", err)
		} else {
			slog.Info("review report posted")
		}
	}

	return nil
}
