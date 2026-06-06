package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/yuhua2000/gitreviewai/internal/config"
	"github.com/yuhua2000/gitreviewai/internal/database"
	"github.com/yuhua2000/gitreviewai/internal/reviewer"
)

type Handler struct {
	cfg          *config.Config
	reviewer     *reviewer.Reviewer
	settingStore *database.SettingStore
}

type WebhookEvent struct {
	ObjectKind string     `json:"object_kind"`
	Project    Project    `json:"project"`
	ObjectAttr ObjectAttr `json:"object_attributes"`
}

type Project struct {
	ID                int    `json:"id"`
	PathWithNamespace string `json:"path_with_namespace"`
	Name              string `json:"name"`
	Description       string `json:"description"`
}

type ObjectAttr struct {
	IID            int    `json:"iid"`
	State          string `json:"state"`
	Action         string `json:"action"`
	Title          string `json:"title"`
	TargetBranch   string `json:"target_branch"`
	SourceBranch   string `json:"source_branch"`
	WorkInProgress bool   `json:"work_in_progress"`
}

func NewHandler(cfg *config.Config, rev *reviewer.Reviewer, settingStore *database.SettingStore) *Handler {
	return &Handler{
		cfg:          cfg,
		reviewer:     rev,
		settingStore: settingStore,
	}
}

// HandleWebhook handles GitLab Webhook requests
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	slog.Debug("webhook request received", "method", r.Method, "remote_addr", r.RemoteAddr)

	// Validate token (check DB first, fall back to config)
	webhookToken, _ := h.settingStore.GetWebhookToken(r.Context(), h.cfg.WebhookToken)
	if webhookToken != "" {
		token := r.Header.Get("X-Gitlab-Token")
		if token != webhookToken {
			slog.Warn("token validation failed", "remote_addr", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Parse event
	var event WebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		slog.Error("failed to parse webhook event", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Only handle MR events
	if event.ObjectKind != "merge_request" {
		slog.Debug("non-MR event ignored", "object_kind", event.ObjectKind)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Event type %s ignored", event.ObjectKind)
		return
	}

	// Only handle open and update actions
	if event.ObjectAttr.Action != "open" && event.ObjectAttr.Action != "update" {
		slog.Debug("MR event ignored", "action", event.ObjectAttr.Action)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "MR action %s ignored", event.ObjectAttr.Action)
		return
	}

	slog.Info("MR event received",
		"project", event.Project.PathWithNamespace,
		"mr_iid", event.ObjectAttr.IID,
		"mr_title", event.ObjectAttr.Title,
		"action", event.ObjectAttr.Action)

	// Process review asynchronously
	go func() {
		req := reviewer.ReviewRequest{
			ProjectID:    strconv.Itoa(event.Project.ID),
			MRIID:        event.ObjectAttr.IID,
			ProjectName:  event.Project.Name,
			Description:  event.Project.Description,
			IsDraft:      event.ObjectAttr.WorkInProgress,
			TargetBranch: event.ObjectAttr.TargetBranch,
			SourceBranch: event.ObjectAttr.SourceBranch,
		}
		slog.Info("review started", "project", event.Project.PathWithNamespace, "mr_iid", event.ObjectAttr.IID)
		if err := h.reviewer.ReviewMR(context.Background(), req); err != nil {
			slog.Error("review failed", "project", event.Project.PathWithNamespace, "mr_iid", event.ObjectAttr.IID, "error", err)
		} else {
			slog.Info("review completed", "project", event.Project.PathWithNamespace, "mr_iid", event.ObjectAttr.IID)
		}
	}()

	// Return 200 immediately
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Review started for MR %d", event.ObjectAttr.IID)
}
