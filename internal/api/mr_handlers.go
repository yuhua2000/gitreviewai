package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yuhua2000/gitreviewai/internal/crypto"
	"github.com/yuhua2000/gitreviewai/internal/gitlab"
	"github.com/yuhua2000/gitreviewai/internal/reviewer"
	"github.com/yuhua2000/gitreviewai/internal/types"
)

func (h *Handler) listMergeRequests(c *gin.Context) {
	offset, limit := parsePageParams(c)

	mrs, total, err := h.mrStore.List(c.Request.Context(), offset, limit)
	if err != nil {
		slog.Error("failed to list MRs", "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to list merge requests"))
		return
	}

	page := offset/limit + 1
	c.JSON(http.StatusOK, types.Success(types.PaginatedData{
		Data:     mrs,
		Total:    total,
		Page:     page,
		PageSize: limit,
	}))
}

func (h *Handler) getMergeRequest(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid id"))
		return
	}

	mr, err := h.mrStore.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, types.Error(types.CodeNotFound, "merge request not found"))
		return
	}

	comments, err := h.commentStore.ListByMRID(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to list comments", "error", err)
		comments = []*types.Comment{}
	}

	reports, err := h.reportStore.ListByMRID(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to list reports", "error", err)
		reports = []*types.Report{}
	}

	c.JSON(http.StatusOK, types.Success(types.MRDetailData{
		ID:           mr.ID,
		ProjectID:    mr.ProjectID,
		MRIID:        mr.MRIID,
		Title:        mr.Title,
		Description:  mr.Description,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		State:        mr.State,
		WebURL:       mr.WebURL,
		ReviewStatus: mr.ReviewStatus,
		CreatedAt:    mr.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    mr.UpdatedAt.Format(time.RFC3339),
		Comments:     comments,
		Reports:      reports,
	}))
}

func (h *Handler) submitComment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid id"))
		return
	}

	comment, err := h.commentStore.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, types.Error(types.CodeNotFound, "comment not found"))
		return
	}

	if comment.Status == "submitted" {
		c.JSON(http.StatusConflict, types.Error(types.CodeConflict, "comment already submitted"))
		return
	}

	mr, err := h.mrStore.GetByID(c.Request.Context(), comment.MRID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to get merge request"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if comment.CommentType == "line" {
		// Need diff refs for line comments - fetch from GitLab
		_, diffRefs, err := h.getGitLabClient(c.Request.Context()).GetMRChanges(ctx, mr.ProjectID, mr.MRIID)
		if err != nil {
			slog.Error("failed to get MR changes for diff refs", "error", err)
			c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to get MR changes"))
			return
		}

		draft := gitlab.DraftNote{
			Note: comment.Content,
			Position: &gitlab.Position{
				BaseSHA:      diffRefs.BaseSHA,
				StartSHA:     diffRefs.StartSHA,
				HeadSHA:      diffRefs.HeadSHA,
				PositionType: "text",
				NewPath:      comment.FilePath,
				OldPath:      comment.FilePath,
				NewLine:      comment.LineNumber,
			},
		}
		draftID, err := h.getGitLabClient(c.Request.Context()).CreateDraftNote(ctx, mr.ProjectID, mr.MRIID, draft)
		if err != nil {
			slog.Error("failed to create draft note", "error", err)
			c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to create draft note"))
			return
		}

		if err := h.getGitLabClient(c.Request.Context()).PublishDraftNote(ctx, mr.ProjectID, mr.MRIID, draftID); err != nil {
			slog.Error("failed to publish draft note", "error", err)
			c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to publish draft note"))
			return
		}

		draftID64 := int64(draftID)
		if err := h.commentStore.UpdateStatus(c.Request.Context(), id, "submitted", nil, &draftID64); err != nil {
			slog.Error("failed to update comment status", "id", id, "error", err)
			c.JSON(http.StatusOK, types.Success(types.SubmitResult{
				ID:      id,
				Status:  "submitted",
				Warning: "评论已提交但状态更新失败，请刷新页面",
			}))
			return
		}
	} else {
		noteID, err := h.getGitLabClient(c.Request.Context()).PostMRNote(ctx, mr.ProjectID, mr.MRIID, comment.Content)
		if err != nil {
			slog.Error("failed to post review comment", "error", err)
			c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to post comment"))
			return
		}

		noteID64 := int64(noteID)
		if err := h.commentStore.UpdateStatus(c.Request.Context(), id, "submitted", &noteID64, nil); err != nil {
			slog.Error("failed to update comment status", "id", id, "error", err)
			c.JSON(http.StatusOK, types.Success(types.SubmitResult{
				ID:      id,
				Status:  "submitted",
				Warning: "评论已提交但状态更新失败，请刷新页面",
			}))
			return
		}
	}

	c.JSON(http.StatusOK, types.Success(types.SubmitResult{
		ID:     id,
		Status: "submitted",
	}))
}

func (h *Handler) submitReport(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid id"))
		return
	}

	report, err := h.reportStore.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, types.Error(types.CodeNotFound, "report not found"))
		return
	}

	if report.Status == "submitted" {
		c.JSON(http.StatusConflict, types.Error(types.CodeConflict, "report already submitted"))
		return
	}

	mr, err := h.mrStore.GetByID(c.Request.Context(), report.MRID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to get merge request"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	reportBody := fmt.Sprintf(`# MR 审核报告

**审核时间:** %s

---

%s

---
*此报告由 GitReviewAI 自动生成，供开发者参考。*`,
		time.Now().Format("2006-01-02 15:04:05"),
		report.Content)

	noteID, err := h.getGitLabClient(c.Request.Context()).PostMRNote(ctx, mr.ProjectID, mr.MRIID, reportBody)
	if err != nil {
		slog.Error("failed to post report", "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to post report"))
		return
	}

	noteID64 := int64(noteID)
	if err := h.reportStore.UpdateStatus(c.Request.Context(), id, "submitted", &noteID64); err != nil {
		slog.Error("failed to update report status", "id", id, "error", err)
		c.JSON(http.StatusOK, types.Success(types.SubmitResult{
			ID:      id,
			Status:  "submitted",
			Warning: "报告已提交但状态更新失败，请刷新页面",
		}))
		return
	}

	c.JSON(http.StatusOK, types.Success(types.SubmitResult{
		ID:     id,
		Status: "submitted",
	}))
}

func (h *Handler) submitAllPending(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid id"))
		return
	}

	mr, err := h.mrStore.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, types.Error(types.CodeNotFound, "merge request not found"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// Get diff refs for line comments
	_, diffRefs, err := h.getGitLabClient(c.Request.Context()).GetMRChanges(ctx, mr.ProjectID, mr.MRIID)
	if err != nil {
		slog.Error("failed to get MR changes", "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to get MR changes"))
		return
	}

	submittedComments := 0
	submittedReports := 0
	var errors []string

	// Submit pending comments
	comments, _ := h.commentStore.ListByMRID(c.Request.Context(), id)
	for _, comment := range comments {
		if comment.Status != "pending" {
			continue
		}

		var submitErr error
		if comment.CommentType == "line" {
			draft := gitlab.DraftNote{
				Note: comment.Content,
				Position: &gitlab.Position{
					BaseSHA:      diffRefs.BaseSHA,
					StartSHA:     diffRefs.StartSHA,
					HeadSHA:      diffRefs.HeadSHA,
					PositionType: "text",
					NewPath:      comment.FilePath,
					OldPath:      comment.FilePath,
					NewLine:      comment.LineNumber,
				},
			}
			draftID, err := h.getGitLabClient(c.Request.Context()).CreateDraftNote(ctx, mr.ProjectID, mr.MRIID, draft)
			if err != nil {
				submitErr = err
			} else {
				if err := h.getGitLabClient(c.Request.Context()).PublishDraftNote(ctx, mr.ProjectID, mr.MRIID, draftID); err != nil {
					submitErr = err
				} else {
					draftID64 := int64(draftID)
					if err := h.commentStore.UpdateStatus(c.Request.Context(), comment.ID, "submitted", nil, &draftID64); err != nil {
						errors = append(errors, fmt.Sprintf("comment %d update status: %v", comment.ID, err))
					} else {
						submittedComments++
					}
				}
			}
		} else {
			noteID, err := h.getGitLabClient(c.Request.Context()).PostMRNote(ctx, mr.ProjectID, mr.MRIID, comment.Content)
			if err != nil {
				submitErr = err
			} else {
				noteID64 := int64(noteID)
				if err := h.commentStore.UpdateStatus(c.Request.Context(), comment.ID, "submitted", &noteID64, nil); err != nil {
					errors = append(errors, fmt.Sprintf("comment %d update status: %v", comment.ID, err))
				} else {
					submittedComments++
				}
			}
		}

		if submitErr != nil {
			errors = append(errors, fmt.Sprintf("comment %d: %v", comment.ID, submitErr))
		}
	}

	// Submit pending reports
	reports, _ := h.reportStore.ListByMRID(c.Request.Context(), id)
	for _, report := range reports {
		if report.Status != "pending" {
			continue
		}

		reportBody := fmt.Sprintf(`# MR 审核报告

**审核时间:** %s

---

%s

---
*此报告由 GitReviewAI 自动生成，供开发者参考。*`,
			time.Now().Format("2006-01-02 15:04:05"),
			report.Content)

		noteID, err := h.getGitLabClient(c.Request.Context()).PostMRNote(ctx, mr.ProjectID, mr.MRIID, reportBody)
		if err != nil {
			errors = append(errors, fmt.Sprintf("report %d: %v", report.ID, err))
		} else {
			noteID64 := int64(noteID)
			if err := h.reportStore.UpdateStatus(c.Request.Context(), report.ID, "submitted", &noteID64); err != nil {
				errors = append(errors, fmt.Sprintf("report %d update status: %v", report.ID, err))
			} else {
				submittedReports++
			}
		}
	}

	c.JSON(http.StatusOK, types.Success(types.SubmitAllResult{
		SubmittedComments: submittedComments,
		SubmittedReports:  submittedReports,
		Errors:            errors,
	}))
}

func (h *Handler) getMRChanges(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid id"))
		return
	}

	mr, err := h.mrStore.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, types.Error(types.CodeNotFound, "merge request not found"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	changes, _, err := h.getGitLabClient(c.Request.Context()).GetMRChanges(ctx, mr.ProjectID, mr.MRIID)
	if err != nil {
		slog.Error("failed to get MR changes", "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to get MR changes"))
		return
	}

	// Paginate
	offset, limit := parsePageParams(c)
	total := len(changes)

	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	page := offset/limit + 1
	c.JSON(http.StatusOK, types.Success(types.PaginatedData{
		Data:     changes[offset:end],
		Total:    total,
		Page:     page,
		PageSize: limit,
	}))
}

func (h *Handler) getSettings(c *gin.Context) {
	ctx := c.Request.Context()

	autoSubmit, _ := h.settingStore.GetAutoSubmit(ctx)
	gitlabURL, _ := h.settingStore.GetGitLabURL(ctx, h.cfg.GitLabURL)
	gitlabToken, _ := h.settingStore.GetGitLabToken(ctx, h.cfg.GitLabToken)
	ignorePaths, _ := h.settingStore.GetIgnorePaths(ctx, h.cfg.IgnorePaths)
	maxComments, _ := h.settingStore.GetMaxLineComments(ctx, h.cfg.MaxLineComments)
	webhookToken, _ := h.settingStore.GetWebhookToken(ctx, h.cfg.WebhookToken)
	logLevel, _ := h.settingStore.GetLogLevel(ctx, h.cfg.LogLevel)
	jwtExpiry, _ := h.settingStore.GetJWTExpiry(ctx, h.cfg.JWTExpiry)

	c.JSON(http.StatusOK, types.Success(types.SettingsData{
		GitLabURL:       gitlabURL,
		GitLabToken:     crypto.MaskSecret(gitlabToken),
		IgnorePaths:     ignorePaths,
		MaxLineComments: maxComments,
		AutoSubmit:      autoSubmit,
		WebhookToken:    crypto.MaskSecret(webhookToken),
		LogLevel:        logLevel,
		JWTExpiry:       jwtExpiry,
	}))
}

func (h *Handler) updateSettings(c *gin.Context) {
	var req types.UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid request"))
		return
	}

	ctx := c.Request.Context()

	if req.GitLabURL != nil {
		if err := h.settingStore.SetGitLabURL(ctx, *req.GitLabURL); err != nil {
			slog.Error("failed to set gitlab_url", "error", err)
		}
	}
	if req.GitLabToken != nil && *req.GitLabToken != "" {
		if err := h.settingStore.SetGitLabToken(ctx, *req.GitLabToken); err != nil {
			slog.Error("failed to set gitlab_token", "error", err)
		}
	}
	if req.IgnorePaths != nil {
		if err := h.settingStore.SetIgnorePaths(ctx, req.IgnorePaths); err != nil {
			slog.Error("failed to set ignore_paths", "error", err)
		}
	}
	if req.MaxLineComments != nil {
		if err := h.settingStore.SetMaxLineComments(ctx, *req.MaxLineComments); err != nil {
			slog.Error("failed to set max_line_comments", "error", err)
		}
	}
	if req.AutoSubmit != nil {
		if err := h.settingStore.SetAutoSubmit(ctx, *req.AutoSubmit); err != nil {
			slog.Error("failed to set auto_submit", "error", err)
		}
	}
	if req.WebhookToken != nil {
		if err := h.settingStore.SetWebhookToken(ctx, *req.WebhookToken); err != nil {
			slog.Error("failed to set webhook_token", "error", err)
		}
	}
	if req.LogLevel != nil {
		if err := h.settingStore.SetLogLevel(ctx, *req.LogLevel); err != nil {
			slog.Error("failed to set log_level", "error", err)
		}
	}
	if req.JWTExpiry != nil {
		if err := h.settingStore.SetJWTExpiry(ctx, *req.JWTExpiry); err != nil {
			slog.Error("failed to set jwt_expiry", "error", err)
		}
	}

	// Return updated settings
	h.getSettings(c)
}

func (h *Handler) listReviewLogs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid id"))
		return
	}

	logs, err := h.reviewLogStore.ListByMRID(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to list review logs", "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to list review logs"))
		return
	}

	c.JSON(http.StatusOK, types.Success(logs))
}

func (h *Handler) retryReview(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid id"))
		return
	}

	mr, err := h.mrStore.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, types.Error(types.CodeNotFound, "merge request not found"))
		return
	}

	projectID, _ := strconv.Atoi(mr.ProjectID)
	req := reviewer.ReviewRequest{
		ProjectID:    mr.ProjectID,
		MRIID:        mr.MRIID,
		ProjectName:  "",
		Description:  "",
		IsDraft:      false,
		TargetBranch: mr.TargetBranch,
		SourceBranch: mr.SourceBranch,
	}

	// Fetch project config to get name/description
	if pc, _ := h.projectConfigStore.GetByProjectID(c.Request.Context(), projectID); pc != nil {
		req.ProjectName = pc.ProjectName
		req.Description = pc.Description
	}

	go func() {
		slog.Info("retry review started", "project", req.ProjectID, "mr_iid", req.MRIID)
		if err := h.reviewer.ReviewMR(context.Background(), req); err != nil {
			slog.Error("retry review failed", "error", err)
		}
	}()

	c.JSON(http.StatusOK, types.Success(gin.H{"message": "review started"}))
}
