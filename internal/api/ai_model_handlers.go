package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/yuhua2000/gitreviewai/internal/crypto"
	"github.com/yuhua2000/gitreviewai/internal/types"
)

// AIModelRequest is the request body for creating/updating an AI model.
type AIModelRequest struct {
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key"`
	ModelName string `json:"model_name"`
}

func (h *Handler) listAIModels(c *gin.Context) {
	models, err := h.aiModelStore.List(c.Request.Context())
	if err != nil {
		slog.Error("failed to list AI models", "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to list AI models"))
		return
	}

	// Mask API keys in response
	for _, m := range models {
		m.APIKey = crypto.MaskSecret(m.APIKey)
	}

	c.JSON(http.StatusOK, types.Success(models))
}

func (h *Handler) createAIModel(c *gin.Context) {
	var req AIModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid request"))
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "name is required"))
		return
	}

	m := &types.AIModel{
		Name:      req.Name,
		BaseURL:   req.BaseURL,
		APIKey:    req.APIKey,
		ModelName: req.ModelName,
		IsDefault: false,
		Enabled:   true,
	}

	id, err := h.aiModelStore.Create(c.Request.Context(), m)
	if err != nil {
		slog.Error("failed to create AI model", "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to create AI model"))
		return
	}

	created, err := h.aiModelStore.GetByID(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to get created AI model", "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to get created AI model"))
		return
	}

	created.APIKey = crypto.MaskSecret(created.APIKey)
	c.JSON(http.StatusOK, types.Success(created))
}

func (h *Handler) updateAIModel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid id"))
		return
	}

	existing, err := h.aiModelStore.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, types.Error(types.CodeNotFound, "AI model not found"))
		return
	}

	var req AIModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid request"))
		return
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.BaseURL != "" {
		existing.BaseURL = req.BaseURL
	}
	// Only update API key if it's provided and not masked
	if req.APIKey != "" && req.APIKey != "****" {
		existing.APIKey = req.APIKey
	}
	if req.ModelName != "" {
		existing.ModelName = req.ModelName
	}

	if err := h.aiModelStore.Update(c.Request.Context(), existing); err != nil {
		slog.Error("failed to update AI model", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to update AI model"))
		return
	}

	updated, err := h.aiModelStore.GetByID(c.Request.Context(), id)
	if err != nil {
		slog.Error("failed to get updated AI model", "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to get updated AI model"))
		return
	}

	updated.APIKey = crypto.MaskSecret(updated.APIKey)
	c.JSON(http.StatusOK, types.Success(updated))
}

func (h *Handler) deleteAIModel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid id"))
		return
	}

	if _, err := h.aiModelStore.GetByID(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, types.Error(types.CodeNotFound, "AI model not found"))
		return
	}

	if err := h.aiModelStore.Delete(c.Request.Context(), id); err != nil {
		slog.Error("failed to delete AI model", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to delete AI model"))
		return
	}

	c.JSON(http.StatusOK, types.Success(nil))
}

func (h *Handler) setDefaultModel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.Error(types.CodeBadRequest, "invalid id"))
		return
	}

	if _, err := h.aiModelStore.GetByID(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, types.Error(types.CodeNotFound, "AI model not found"))
		return
	}

	if err := h.aiModelStore.SetDefault(c.Request.Context(), id); err != nil {
		slog.Error("failed to set default AI model", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, types.Error(types.CodeInternalError, "failed to set default AI model"))
		return
	}

	c.JSON(http.StatusOK, types.Success(nil))
}
