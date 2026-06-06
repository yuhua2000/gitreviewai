package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yuhua2000/gitreviewai/internal/types"
)

// RuleRequest is the request body for creating or updating a review rule.
type RuleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // error, warning, info
}

func (h *Handler) listRules(c *gin.Context) {
	rules, err := h.ruleStore.List(c.Request.Context())
	if err != nil {
		slog.Error("failed to list rules", "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to list rules"))
		return
	}

	c.JSON(http.StatusOK, types.Success(rules))
}

func (h *Handler) createRule(c *gin.Context) {
	var req RuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid request body"))
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "name is required"))
		return
	}

	if !isValidSeverity(req.Severity) {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "severity must be one of: error, warning, info"))
		return
	}

	ruleID := "custom-" + generateRuleID(req.Name)

	rule := &types.ReviewRule{
		RuleID:      ruleID,
		Name:        req.Name,
		Description: req.Description,
		Severity:    req.Severity,
		Enabled:     true,
	}

	id, err := h.ruleStore.Create(c.Request.Context(), rule)
	if err != nil {
		slog.Error("failed to create rule", "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to create rule"))
		return
	}

	created, err := h.ruleStore.GetByID(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to get created rule", "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "rule created but failed to retrieve"))
		return
	}

	c.JSON(http.StatusCreated, types.Success(created))
}

func (h *Handler) updateRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid id"))
		return
	}

	existing, err := h.ruleStore.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, types.Error(types.CodeNotFound, "rule not found"))
		return
	}

	var req RuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid request body"))
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "name is required"))
		return
	}

	if !isValidSeverity(req.Severity) {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "severity must be one of: error, warning, info"))
		return
	}

	existing.Name = req.Name
	existing.Description = req.Description
	existing.Severity = req.Severity

	if err := h.ruleStore.Update(c.Request.Context(), existing); err != nil {
		slog.Error("failed to update rule", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to update rule"))
		return
	}

	updated, err := h.ruleStore.GetByID(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to get updated rule", "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "rule updated but failed to retrieve"))
		return
	}

	c.JSON(http.StatusOK, types.Success(updated))
}

func (h *Handler) deleteRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid id"))
		return
	}

	if err := h.ruleStore.Delete(c.Request.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found or is builtin") {
			c.JSON(http.StatusForbidden, types.Error(types.CodeConflict, "cannot delete builtin rules"))
			return
		}
		slog.Error("failed to delete rule", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to delete rule"))
		return
	}

	c.JSON(http.StatusOK, types.Success(nil))
}

func (h *Handler) toggleRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid id"))
		return
	}

	existing, err := h.ruleStore.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, types.Error(types.CodeNotFound, "rule not found"))
		return
	}

	newEnabled := !existing.Enabled
	if err := h.ruleStore.Toggle(c.Request.Context(), id, newEnabled); err != nil {
		slog.Error("failed to toggle rule", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to toggle rule"))
		return
	}

	updated, err := h.ruleStore.GetByID(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to get toggled rule", "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "rule toggled but failed to retrieve"))
		return
	}

	c.JSON(http.StatusOK, types.Success(updated))
}

// isValidSeverity checks if the severity is one of the allowed values.
func isValidSeverity(s string) bool {
	switch s {
	case "error", "warning", "info":
		return true
	default:
		return false
	}
}

// generateRuleID creates a unique rule ID from a name using a sanitized name and timestamp.
func generateRuleID(name string) string {
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		if r == ' ' {
			return '-'
		}
		return -1
	}, strings.ToLower(strings.TrimSpace(name)))

	sanitized = strings.Trim(sanitized, "-_")
	if sanitized == "" {
		return fmt.Sprintf("%d", time.Now().UnixMilli())
	}
	return fmt.Sprintf("%s-%d", sanitized, time.Now().UnixMilli())
}
