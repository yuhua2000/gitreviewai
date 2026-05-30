package api

import (
	"database/sql"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yuhua2000/gitreviewai/internal/auth"
	"github.com/yuhua2000/gitreviewai/internal/config"
	"github.com/yuhua2000/gitreviewai/internal/database"
	"github.com/yuhua2000/gitreviewai/internal/gitlab"
	"github.com/yuhua2000/gitreviewai/internal/reviewer"
)

//go:embed all:frontend/dist
var frontendFS embed.FS

type Handler struct {
	cfg          *config.Config
	mrStore      *database.MRStore
	commentStore *database.CommentStore
	reportStore  *database.ReportStore
	settingStore *database.SettingStore
	glClient     *gitlab.Client
}

func NewHandler(cfg *config.Config, db *sql.DB, rev *reviewer.Reviewer) *Handler {
	mrStore, commentStore, reportStore, settingStore := rev.GetStores()
	return &Handler{
		cfg:          cfg,
		mrStore:      mrStore,
		commentStore: commentStore,
		reportStore:  reportStore,
		settingStore: settingStore,
		glClient:     rev.GetGitLabClient(),
	}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// API routes
	api := r.Group("/api")
	{
		api.POST("/login", auth.GinLoginHandler(h.cfg.Password, h.cfg.JWTSecret, h.cfg.JWTExpiryDuration()))

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
			authorized.GET("/settings", h.getSettings)
			authorized.PUT("/settings", h.updateSettings)
		}
	}

	// Serve frontend SPA
	distFS, err := fs.Sub(frontendFS, "frontend/dist")
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
