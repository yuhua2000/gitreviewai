package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yuhua2000/gitreviewai/internal/database"
	"github.com/yuhua2000/gitreviewai/internal/gitlab"
)

func (h *Handler) listMergeRequests(c *gin.Context) {
	offset, limit := parsePageParams(c)

	mrs, total, err := h.mrStore.List(c.Request.Context(), offset, limit)
	if err != nil {
		slog.Error("failed to list MRs", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list merge requests"})
		return
	}

	page := offset/limit + 1
	c.JSON(http.StatusOK, gin.H{
		"data":      mrs,
		"total":     total,
		"page":      page,
		"page_size": limit,
	})
}

func (h *Handler) getMergeRequest(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	mr, err := h.mrStore.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "merge request not found"})
		return
	}

	comments, err := h.commentStore.ListByMRID(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to list comments", "error", err)
		comments = []*database.Comment{}
	}

	reports, err := h.reportStore.ListByMRID(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to list reports", "error", err)
		reports = []*database.Report{}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":            mr.ID,
		"project_id":    mr.ProjectID,
		"mr_iid":        mr.MRIID,
		"title":         mr.Title,
		"description":   mr.Description,
		"source_branch": mr.SourceBranch,
		"target_branch": mr.TargetBranch,
		"state":         mr.State,
		"web_url":       mr.WebURL,
		"review_status": mr.ReviewStatus,
		"created_at":    mr.CreatedAt,
		"updated_at":    mr.UpdatedAt,
		"comments":      comments,
		"reports":       reports,
	})
}

func (h *Handler) submitComment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	comment, err := h.commentStore.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "comment not found"})
		return
	}

	if comment.Status == "submitted" {
		c.JSON(http.StatusConflict, gin.H{"error": "comment already submitted"})
		return
	}

	mr, err := h.mrStore.GetByID(c.Request.Context(), comment.MRID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get merge request"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if comment.CommentType == "line" {
		// Need diff refs for line comments - fetch from GitLab
		_, diffRefs, err := h.glClient.GetMRChanges(ctx, mr.ProjectID, mr.MRIID)
		if err != nil {
			slog.Error("failed to get MR changes for diff refs", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get MR changes"})
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
		draftID, err := h.glClient.CreateDraftNote(ctx, mr.ProjectID, mr.MRIID, draft)
		if err != nil {
			slog.Error("failed to create draft note", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create draft note"})
			return
		}

		if err := h.glClient.PublishDraftNote(ctx, mr.ProjectID, mr.MRIID, draftID); err != nil {
			slog.Error("failed to publish draft note", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish draft note"})
			return
		}

		draftID64 := int64(draftID)
		h.commentStore.UpdateStatus(c.Request.Context(), id, "submitted", nil, &draftID64)
	} else {
		noteID, err := h.glClient.PostMRNote(ctx, mr.ProjectID, mr.MRIID, comment.Content)
		if err != nil {
			slog.Error("failed to post review comment", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to post comment"})
			return
		}

		noteID64 := int64(noteID)
		h.commentStore.UpdateStatus(c.Request.Context(), id, "submitted", &noteID64, nil)
	}

	c.JSON(http.StatusOK, gin.H{
		"id":     id,
		"status": "submitted",
	})
}

func (h *Handler) submitReport(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	report, err := h.reportStore.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
		return
	}

	if report.Status == "submitted" {
		c.JSON(http.StatusConflict, gin.H{"error": "report already submitted"})
		return
	}

	mr, err := h.mrStore.GetByID(c.Request.Context(), report.MRID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get merge request"})
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

	noteID, err := h.glClient.PostMRNote(ctx, mr.ProjectID, mr.MRIID, reportBody)
	if err != nil {
		slog.Error("failed to post report", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to post report"})
		return
	}

	noteID64 := int64(noteID)
	h.reportStore.UpdateStatus(c.Request.Context(), id, "submitted", &noteID64)

	c.JSON(http.StatusOK, gin.H{
		"id":     id,
		"status": "submitted",
	})
}

func (h *Handler) submitAllPending(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	mr, err := h.mrStore.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "merge request not found"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// Get diff refs for line comments
	_, diffRefs, err := h.glClient.GetMRChanges(ctx, mr.ProjectID, mr.MRIID)
	if err != nil {
		slog.Error("failed to get MR changes", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get MR changes"})
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
			draftID, err := h.glClient.CreateDraftNote(ctx, mr.ProjectID, mr.MRIID, draft)
			if err != nil {
				submitErr = err
			} else {
				if err := h.glClient.PublishDraftNote(ctx, mr.ProjectID, mr.MRIID, draftID); err != nil {
					submitErr = err
				} else {
					draftID64 := int64(draftID)
					h.commentStore.UpdateStatus(c.Request.Context(), comment.ID, "submitted", nil, &draftID64)
					submittedComments++
				}
			}
		} else {
			noteID, err := h.glClient.PostMRNote(ctx, mr.ProjectID, mr.MRIID, comment.Content)
			if err != nil {
				submitErr = err
			} else {
				noteID64 := int64(noteID)
				h.commentStore.UpdateStatus(c.Request.Context(), comment.ID, "submitted", &noteID64, nil)
				submittedComments++
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

		noteID, err := h.glClient.PostMRNote(ctx, mr.ProjectID, mr.MRIID, reportBody)
		if err != nil {
			errors = append(errors, fmt.Sprintf("report %d: %v", report.ID, err))
		} else {
			noteID64 := int64(noteID)
			h.reportStore.UpdateStatus(c.Request.Context(), report.ID, "submitted", &noteID64)
			submittedReports++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"submitted_comments": submittedComments,
		"submitted_reports":  submittedReports,
		"errors":             errors,
	})
}

func (h *Handler) getMRChanges(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	mr, err := h.mrStore.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "merge request not found"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	changes, _, err := h.glClient.GetMRChanges(ctx, mr.ProjectID, mr.MRIID)
	if err != nil {
		slog.Error("failed to get MR changes", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get MR changes"})
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
	c.JSON(http.StatusOK, gin.H{
		"data":      changes[offset:end],
		"total":     total,
		"page":      page,
		"page_size": limit,
	})
}

func (h *Handler) getSettings(c *gin.Context) {
	autoSubmit, _ := h.settingStore.GetAutoSubmit(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{
		"auto_submit": autoSubmit,
	})
}

func (h *Handler) updateSettings(c *gin.Context) {
	var req struct {
		AutoSubmit *bool `json:"auto_submit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.AutoSubmit != nil {
		if err := h.settingStore.SetAutoSubmit(c.Request.Context(), *req.AutoSubmit); err != nil {
			slog.Error("failed to set auto_submit", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update settings"})
			return
		}
	}

	autoSubmit, _ := h.settingStore.GetAutoSubmit(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{
		"auto_submit": autoSubmit,
	})
}
