package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yuhua2000/gitreviewai/internal/api"
	"github.com/yuhua2000/gitreviewai/internal/config"
	"github.com/yuhua2000/gitreviewai/internal/database"
	"github.com/yuhua2000/gitreviewai/internal/reviewer"
	"github.com/yuhua2000/gitreviewai/internal/webhook"
)

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

func main() {
	// Load config
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config failed", "error", err)
		os.Exit(1)
	}

	// Validate required config
	if cfg.GitLabToken == "" {
		slog.Error("GITLAB_TOKEN is required")
		os.Exit(1)
	}
	if cfg.OpenAIAPIKey == "" {
		slog.Error("OPENAI_API_KEY is required")
		os.Exit(1)
	}
	if cfg.Password == "" {
		slog.Error("password is required")
		os.Exit(1)
	}
	if cfg.JWTSecret == "" {
		slog.Error("jwt_secret is required")
		os.Exit(1)
	}

	// Configure slog
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(cfg.LogLevel),
	})))

	// Open database
	db, err := database.Open(cfg.DBPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create reviewer with DB access
	rev := reviewer.New(cfg, db)

	// Create webhook handler
	webhookHandler := webhook.NewHandler(cfg, rev)

	// Create API handler
	apiHandler := api.NewHandler(cfg, db, rev)

	// Set up gin router
	gin.SetMode(gin.DebugMode)
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

	// Webhook route (no auth, uses X-Gitlab-Token)
	r.POST("/webhook", gin.WrapF(webhookHandler.HandleWebhook))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// API and frontend routes
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
		slog.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}
