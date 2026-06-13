package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/yuhua2000/gitreviewai/frontend"
	"github.com/yuhua2000/gitreviewai/internal/api"
	"github.com/yuhua2000/gitreviewai/internal/config"
	"github.com/yuhua2000/gitreviewai/internal/database"
	"github.com/yuhua2000/gitreviewai/internal/reviewer"
	"github.com/yuhua2000/gitreviewai/internal/webhook"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the review server",
	Long:  `Start the HTTP server that listens for GitLab webhooks and serves the web management interface.`,
	RunE:  runServer,
}

var configPath string

func init() {
	serverCmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "config file path")
}

// parseLogLevel converts a log level string to slog.Level.
func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// runServer starts the HTTP server with webhook and web UI.
func runServer(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Validate required config
	if cfg.Password == "" {
		return fmt.Errorf("password is required")
	}
	if cfg.JWTSecret == "" {
		return fmt.Errorf("jwt_secret is required")
	}

	// Warn if business config is missing
	if cfg.GitLabToken == "" {
		slog.Warn("gitlab_token not set, configure via web UI or config.yaml")
	}
	if cfg.OpenAIAPIKey == "" {
		slog.Warn("openai_api_key not set, configure via web UI or config.yaml")
	}

	// Open database
	db, err := database.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	// Set up log level from DB (fallback to config)
	logLevelVar := new(slog.LevelVar)
	dbLogLevel, _ := database.NewSettingStore(db).GetLogLevel(context.Background(), cfg.LogLevel)
	logLevelVar.Set(parseLogLevel(dbLogLevel))
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevelVar,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.String(slog.TimeKey, a.Value.Time().Format("01-02 15:04:05"))
			}
			return a
		},
	})))

	// Create reviewer with DB access
	rev := reviewer.New(cfg, db)

	// Create handlers
	webhookHandler := webhook.NewHandler(cfg, rev, database.NewSettingStore(db))
	apiHandler := api.NewHandler(cfg, db, rev, frontend.FS)

	// Set up gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		slog.Info("request",
			"status", param.StatusCode,
			"method", param.Method,
			"path", param.Path,
			"query", param.Request.URL.RawQuery,
			"ip", param.ClientIP,
			"latency", param.Latency.String(),
			"user_agent", param.Request.UserAgent(),
		)
		return ""
	}))

	// Routes
	r.POST("/webhook", gin.WrapF(webhookHandler.HandleWebhook))
	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})
	apiHandler.RegisterRoutes(r)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("server shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	slog.Info("server stopped")
	return nil
}
