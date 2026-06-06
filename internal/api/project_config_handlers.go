package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/yuhua2000/gitreviewai/internal/types"
)

// UpdateProjectConfigRequest is the request body for updating a project config.
type UpdateProjectConfigRequest struct {
	AIModelID       *int64  `json:"ai_model_id"`
	AutoSubmit      *bool   `json:"auto_submit"`
	SkipDraft       *bool   `json:"skip_draft"`
	TargetBranches  *string `json:"target_branches"`
	IgnorePaths     *string `json:"ignore_paths"`
	MaxLineComments *int    `json:"max_line_comments"`
	CustomPrompt    *string `json:"custom_prompt"`
	Enabled         *bool   `json:"enabled"`
}

// UpdateProjectRulesRequest is the request body for batch updating rule overrides.
type UpdateProjectRulesRequest struct {
	Rules []struct {
		RuleID  string `json:"rule_id"`
		Enabled bool   `json:"enabled"`
	} `json:"rules"`
}

// ProjectConfigDetailData is the response for getProjectConfig.
type ProjectConfigDetailData struct {
	ProjectConfig *types.ProjectConfig         `json:"project_config"`
	RuleOverrides []*types.ProjectRuleOverride `json:"rule_overrides"`
	AIModel       *types.AIModel               `json:"ai_model,omitempty"`
}

func (h *Handler) listProjectConfigs(c *gin.Context) {
	offset, limit := parsePageParams(c)

	configs, total, err := h.projectConfigStore.List(c.Request.Context(), offset, limit)
	if err != nil {
		slog.Error("failed to list project configs", "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to list project configs"))
		return
	}

	page := offset/limit + 1
	c.JSON(http.StatusOK, types.Success(types.PaginatedData{
		Data:     configs,
		Total:    total,
		Page:     page,
		PageSize: limit,
	}))
}

func (h *Handler) getProjectConfig(c *gin.Context) {
	gitlabProjectID, err := strconv.Atoi(c.Param("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid project_id"))
		return
	}

	pc, err := h.projectConfigStore.GetByProjectID(c.Request.Context(), gitlabProjectID)
	if err != nil {
		slog.Error("failed to get project config", "project_id", gitlabProjectID, "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to get project config"))
		return
	}
	if pc == nil {
		c.JSON(http.StatusNotFound, types.Error(types.CodeNotFound, "project config not found"))
		return
	}

	ruleOverrides, err := h.projectConfigStore.GetRuleOverrides(c.Request.Context(), pc.ID)
	if err != nil {
		slog.Error("failed to get rule overrides", "project_config_id", pc.ID, "error", err)
		ruleOverrides = []*types.ProjectRuleOverride{}
	}

	resp := ProjectConfigDetailData{
		ProjectConfig: pc,
		RuleOverrides: ruleOverrides,
	}

	if pc.AIModelID != nil {
		aiModel, err := h.aiModelStore.GetByID(c.Request.Context(), *pc.AIModelID)
		if err != nil {
			slog.Error("failed to get ai model", "ai_model_id", *pc.AIModelID, "error", err)
		} else {
			resp.AIModel = aiModel
		}
	}

	c.JSON(http.StatusOK, types.Success(resp))
}

func (h *Handler) updateProjectConfig(c *gin.Context) {
	gitlabProjectID, err := strconv.Atoi(c.Param("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid project_id"))
		return
	}

	pc, err := h.projectConfigStore.GetByProjectID(c.Request.Context(), gitlabProjectID)
	if err != nil {
		slog.Error("failed to get project config", "project_id", gitlabProjectID, "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to get project config"))
		return
	}
	if pc == nil {
		c.JSON(http.StatusNotFound, types.Error(types.CodeNotFound, "project config not found"))
		return
	}

	var req UpdateProjectConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid request body"))
		return
	}

	// Apply partial updates
	if req.AIModelID != nil {
		pc.AIModelID = req.AIModelID
	}
	if req.AutoSubmit != nil {
		pc.AutoSubmit = *req.AutoSubmit
	}
	if req.SkipDraft != nil {
		pc.SkipDraft = *req.SkipDraft
	}
	if req.TargetBranches != nil {
		pc.TargetBranches = *req.TargetBranches
	}
	if req.IgnorePaths != nil {
		pc.IgnorePaths = *req.IgnorePaths
	}
	if req.MaxLineComments != nil {
		pc.MaxLineComments = req.MaxLineComments
	}
	if req.CustomPrompt != nil {
		pc.CustomPrompt = *req.CustomPrompt
	}
	if req.Enabled != nil {
		pc.Enabled = *req.Enabled
	}

	if err := h.projectConfigStore.Update(c.Request.Context(), pc); err != nil {
		slog.Error("failed to update project config", "project_id", gitlabProjectID, "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to update project config"))
		return
	}

	c.JSON(http.StatusOK, types.Success(pc))
}

func (h *Handler) updateProjectRules(c *gin.Context) {
	gitlabProjectID, err := strconv.Atoi(c.Param("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid project_id"))
		return
	}

	pc, err := h.projectConfigStore.GetByProjectID(c.Request.Context(), gitlabProjectID)
	if err != nil {
		slog.Error("failed to get project config", "project_id", gitlabProjectID, "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to get project config"))
		return
	}
	if pc == nil {
		c.JSON(http.StatusNotFound, types.Error(types.CodeNotFound, "project config not found"))
		return
	}

	var req UpdateProjectRulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid request body"))
		return
	}

	// Convert request to override models
	overrides := make([]*types.ProjectRuleOverride, 0, len(req.Rules))
	for _, r := range req.Rules {
		if r.RuleID == "" {
			continue
		}
		overrides = append(overrides, &types.ProjectRuleOverride{
			ProjectConfigID: pc.ID,
			RuleID:          r.RuleID,
			Enabled:         r.Enabled,
		})
	}

	if err := h.projectConfigStore.SetRuleOverrides(c.Request.Context(), pc.ID, overrides); err != nil {
		slog.Error("failed to set rule overrides", "project_config_id", pc.ID, "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to update rule overrides"))
		return
	}

	// Return updated overrides
	updatedOverrides, err := h.projectConfigStore.GetRuleOverrides(c.Request.Context(), pc.ID)
	if err != nil {
		slog.Error("failed to get updated rule overrides", "project_config_id", pc.ID, "error", err)
		updatedOverrides = overrides
	}

	c.JSON(http.StatusOK, types.Success(updatedOverrides))
}
