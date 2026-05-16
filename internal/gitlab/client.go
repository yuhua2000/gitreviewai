package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type MRInfo struct {
	IID          int    `json:"iid"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	State        string `json:"state"`
	WebURL       string `json:"web_url"`
}

type MRChange struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	Diff        string `json:"diff"`
	NewFile     bool   `json:"new_file"`
	RenamedFile bool   `json:"renamed_file"`
	DeletedFile bool   `json:"deleted_file"`
}

type MRChangesResponse struct {
	Changes  []MRChange `json:"changes"`
	DiffRefs *DiffRefs  `json:"diff_refs"`
}

type LineComment struct {
	Body     string    `json:"body"`
	Position *Position `json:"position,omitempty"`
}

type Position struct {
	BaseSHA      string `json:"base_sha"`
	StartSHA     string `json:"start_sha"`
	HeadSHA      string `json:"head_sha"`
	PositionType string `json:"position_type"`
	NewPath      string `json:"new_path"`
	OldPath      string `json:"old_path"`
	NewLine      int    `json:"new_line,omitempty"`
	OldLine      int    `json:"old_line,omitempty"`
}

type DiffRefs struct {
	BaseSHA  string `json:"base_sha"`
	StartSHA string `json:"start_sha"`
	HeadSHA  string `json:"head_sha"`
}

type DraftNote struct {
	Note     string    `json:"note"`
	Position *Position `json:"position,omitempty"`
}

// DraftNoteResult holds the response from creating a draft note
type DraftNoteResult struct {
	ID int `json:"id"`
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) GetMRInfo(ctx context.Context, projectID string, mrIID int) (*MRInfo, error) {
	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d", url.PathEscape(projectID), mrIID)
	var mr MRInfo
	if err := c.get(ctx, path, &mr); err != nil {
		return nil, fmt.Errorf("failed to get MR info: %w", err)
	}
	return &mr, nil
}

func (c *Client) GetMRChanges(ctx context.Context, projectID string, mrIID int) ([]MRChange, *DiffRefs, error) {
	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/changes", url.PathEscape(projectID), mrIID)

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get MR changes: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}

	var fullResp MRChangesResponse
	if err := json.Unmarshal(body, &fullResp); err != nil {
		return nil, nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return fullResp.Changes, fullResp.DiffRefs, nil
}

type MRNoteResult struct {
	ID int `json:"id"`
}

func (c *Client) PostMRNote(ctx context.Context, projectID string, mrIID int, body string) (int, error) {
	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/notes", url.PathEscape(projectID), mrIID)
	payload := map[string]string{"body": body}
	var result MRNoteResult
	if err := c.post(ctx, path, payload, &result); err != nil {
		return 0, err
	}
	return result.ID, nil
}

func (c *Client) CreateDraftNote(ctx context.Context, projectID string, mrIID int, note DraftNote) (int, error) {
	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/draft_notes", url.PathEscape(projectID), mrIID)
	slog.Debug("creating draft note", "url", c.baseURL+path)
	var result DraftNoteResult
	if err := c.post(ctx, path, note, &result); err != nil {
		return 0, err
	}
	return result.ID, nil
}

func (c *Client) PublishDraftNotes(ctx context.Context, projectID string, mrIID int) error {
	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/draft_notes/bulk_publish", url.PathEscape(projectID), mrIID)
	slog.Debug("bulk publishing draft notes", "url", c.baseURL+path)
	return c.post(ctx, path, nil, nil)
}

// PublishDraftNote publishes a single draft note by its ID
func (c *Client) PublishDraftNote(ctx context.Context, projectID string, mrIID int, draftNoteID int) error {
	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/draft_notes/%d/publish",
		url.PathEscape(projectID), mrIID, draftNoteID)
	slog.Debug("publishing draft note", "url", c.baseURL+path, "draft_note_id", draftNoteID)
	return c.put(ctx, path)
}

func (c *Client) GetFileContent(ctx context.Context, projectID, ref, filePath string) (string, error) {
	path := fmt.Sprintf("/api/v4/projects/%s/repository/files/%s/raw?ref=%s",
		url.PathEscape(projectID),
		url.PathEscape(filePath),
		url.QueryEscape(ref),
	)

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get file content: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}

func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	return nil
}

func (c *Client) post(ctx context.Context, path string, payload interface{}, result interface{}) error {
	var bodyReader io.Reader

	if payload != nil {
		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
		slog.Debug("POST request", "url", c.baseURL+path, "body", string(jsonBytes))
		bodyReader = strings.NewReader(string(jsonBytes))
	} else {
		slog.Debug("POST request", "url", c.baseURL+path, "body", "nil")
	}

	resp, err := c.doRequest(ctx, "POST", path, bodyReader)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if result != nil {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

func (c *Client) put(ctx context.Context, path string) error {
	slog.Debug("PUT request", "url", c.baseURL+path)
	resp, err := c.doRequest(ctx, "PUT", path, nil)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return resp, nil
}
