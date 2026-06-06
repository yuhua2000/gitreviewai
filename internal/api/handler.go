package api

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yuhua2000/gitreviewai/internal/auth"
	"github.com/yuhua2000/gitreviewai/internal/config"
	"github.com/yuhua2000/gitreviewai/internal/database"
	"github.com/yuhua2000/gitreviewai/internal/gitlab"
	"github.com/yuhua2000/gitreviewai/internal/reviewer"
)

type Handler struct {
	cfg                *config.Config
	mrStore            *database.MRStore
	commentStore       *database.CommentStore
	reportStore        *database.ReportStore
	settingStore       *database.SettingStore
	aiModelStore       *database.AIModelStore
	ruleStore          *database.ReviewRuleStore
	projectConfigStore *database.ProjectConfigStore
	reviewLogStore     *database.ReviewLogStore
	reviewer           *reviewer.Reviewer
	frontendFS         embed.FS
}

func NewHandler(cfg *config.Config, db *sql.DB, rev *reviewer.Reviewer, frontendFS embed.FS) *Handler {
	mrStore, commentStore, reportStore, settingStore := rev.GetStores()
	return &Handler{
		cfg:                cfg,
		mrStore:            mrStore,
		commentStore:       commentStore,
		reportStore:        reportStore,
		settingStore:       settingStore,
		aiModelStore:       database.NewAIModelStore(db),
		ruleStore:          database.NewReviewRuleStore(db),
		projectConfigStore: database.NewProjectConfigStore(db),
		reviewLogStore:     database.NewReviewLogStore(db),
		reviewer:           rev,
		frontendFS:         frontendFS,
	}
}

// getGitLabClient creates a GitLab client from current settings.
func (h *Handler) getGitLabClient(ctx context.Context) *gitlab.Client {
	gitlabURL, _ := h.settingStore.GetGitLabURL(ctx, h.cfg.GitLabURL)
	gitlabToken, _ := h.settingStore.GetGitLabToken(ctx, h.cfg.GitLabToken)
	return gitlab.NewClient(gitlabURL, gitlabToken)
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// API routes
	api := r.Group("/api")
	{
		api.POST("/login", auth.GinLoginHandler(h.cfg.Password, h.cfg.JWTSecret, func() time.Duration {
			expiryStr, _ := h.settingStore.GetJWTExpiry(context.Background(), h.cfg.JWTExpiry)
			d, err := time.ParseDuration(expiryStr)
			if err != nil {
				return 24 * time.Hour
			}
			return d
		}))

		// Authenticated routes
		authorized := api.Group("")
		authorized.Use(auth.GinMiddleware(h.cfg.JWTSecret))
		{
			authorized.GET("/mrs", h.listMergeRequests)
			authorized.GET("/mrs/:id", h.getMergeRequest)
			authorized.GET("/mrs/:id/changes", h.getMRChanges)
			authorized.POST("/comments/:id/submit", h.submitComment)
			authorized.POST("/reports/:id/submit", h.submitReport)
			authorized.POST("/mrs/:id/submit-all", h.submitAllPending)
			authorized.GET("/mrs/:id/review-logs", h.listReviewLogs)
			authorized.POST("/mrs/:id/retry", h.retryReview)
			authorized.GET("/settings", h.getSettings)
			authorized.PUT("/settings", h.updateSettings)

			// AI model management
			authorized.GET("/ai-models", h.listAIModels)
			authorized.POST("/ai-models", h.createAIModel)
			authorized.PUT("/ai-models/:id", h.updateAIModel)
			authorized.DELETE("/ai-models/:id", h.deleteAIModel)
			authorized.POST("/ai-models/:id/set-default", h.setDefaultModel)

			// Review rule management
			authorized.GET("/rules", h.listRules)
			authorized.POST("/rules", h.createRule)
			authorized.PUT("/rules/:id", h.updateRule)
			authorized.DELETE("/rules/:id", h.deleteRule)
			authorized.PUT("/rules/:id/toggle", h.toggleRule)

			// Project config management
			authorized.GET("/project-configs", h.listProjectConfigs)
			authorized.GET("/project-configs/:project_id", h.getProjectConfig)
			authorized.PUT("/project-configs/:project_id", h.updateProjectConfig)
			authorized.PUT("/project-configs/:project_id/rules", h.updateProjectRules)
		}
	}

	// Serve frontend SPA
	distFS, err := fs.Sub(h.frontendFS, "dist")
	if err != nil {
		slog.Warn("frontend not embedded", "error", err)
		return
	}

	// Read index.html for SPA fallback
	indexHTML, _ := fs.ReadFile(distFS, "index.html")

	// File server for static assets
	fileServer := http.FileServer(http.FS(distFS))

	// All non-API routes serve the SPA
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip API routes
		if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/webhook") || strings.HasPrefix(path, "/health") {
			c.Status(http.StatusNotFound)
			return
		}

		// Try to serve static file
		if path != "/" {
			filePath := strings.TrimPrefix(path, "/")
			if _, err := fs.Stat(distFS, filePath); err == nil {
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}

		// SPA fallback: serve index.html
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})
}

func parsePageParams(c *gin.Context) (int, int) {
	page := 1
	pageSize := 20

	if p := c.Query("page"); p != "" {
		if v, err := parsePositiveInt(p); err == nil && v > 0 {
			page = v
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if v, err := parsePositiveInt(ps); err == nil && v > 0 && v <= 100 {
			pageSize = v
		}
	}

	return (page - 1) * pageSize, pageSize
}

func parsePositiveInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, nil
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
