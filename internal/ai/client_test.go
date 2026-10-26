package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// recordedChatRequest is the subset of the chat completion request we inspect.
type recordedChatRequest struct {
	Messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"messages"`
}

func messageContent(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

func isCompressionRequest(req recordedChatRequest) bool {
	return len(req.Messages) > 0 && strings.Contains(messageContent(req.Messages[0].Content), "对话压缩器")
}

func writeMockResponse(w http.ResponseWriter, content, toolName string, promptTokens, completionTokens int) {
	message := map[string]any{"role": "assistant", "content": content}
	finishReason := "stop"
	if toolName != "" {
		finishReason = "tool_calls"
		message["content"] = ""
		message["tool_calls"] = []map[string]any{{
			"id":   "call_" + toolName,
			"type": "function",
			"function": map[string]any{
				"name":      toolName,
				"arguments": "{}",
			},
		}}
	}

	resp := map[string]any{
		"id":      "chatcmpl-test",
		"object":  "chat.completion",
		"created": 0,
		"model":   "test-model",
		"choices": []map[string]any{{
			"index":         0,
			"finish_reason": finishReason,
			"logprobs":      nil,
			"message":       message,
		}},
		"usage": map[string]any{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      promptTokens + completionTokens,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// newMockClient starts an OpenAI-compatible mock server and returns a client
// pointed at it. respond is called with the 1-based request index.
func newMockClient(t *testing.T, threshold int,
	respond func(call int, req recordedChatRequest) (content, toolName string, promptTokens, completionTokens int),
) (*Client, *[]recordedChatRequest, func()) {
	t.Helper()

	var mu sync.Mutex
	var reqs []recordedChatRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req recordedChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		mu.Lock()
		reqs = append(reqs, req)
		call := len(reqs)
		mu.Unlock()

		content, toolName, promptTokens, completionTokens := respond(call, req)
		writeMockResponse(w, content, toolName, promptTokens, completionTokens)
	}))

	client := NewClient("test-key", "test-model", srv.URL, threshold)
	return client, &reqs, srv.Close
}

func TestChatLoopCompressesOnPromptTokens(t *testing.T) {
	client, reqs, closeSrv := newMockClient(t, 50, func(call int, _ recordedChatRequest) (string, string, int, int) {
		switch call {
		case 1:
			return "", "GetMoreChanges", 100, 10
		case 2: // compression request
			return "### 审核进度\n- 已审核：1 个文件", "", 20, 5
		case 3:
			return "", "FinishReview", 30, 5
		default:
			t.Errorf("unexpected request #%d", call)
			return "", "", 0, 0
		}
	})
	defer closeSrv()

	_, usage, err := client.ChatWithLimit(context.Background(), "system prompt", "initial message",
		func(string, json.RawMessage) (string, error) { return "ok", nil }, 5)
	if err != nil {
		t.Fatalf("ChatWithLimit: %v", err)
	}
	if len(*reqs) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(*reqs))
	}
	if !isCompressionRequest((*reqs)[1]) {
		t.Fatal("second request should be a compression request")
	}
	if got := len((*reqs)[2].Messages); got != 3 {
		t.Fatalf("expected compressed history of 3 messages, got %d", got)
	}
	// Compression call usage should be included in the cumulative total.
	if usage.PromptTokens != 150 || usage.CompletionTokens != 20 || usage.TotalTokens != 170 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
}

func TestChatLoopCompressionDisabled(t *testing.T) {
	client, reqs, closeSrv := newMockClient(t, 0, func(call int, _ recordedChatRequest) (string, string, int, int) {
		switch call {
		case 1:
			return "", "GetMoreChanges", 100, 10
		case 2:
			return "", "FinishReview", 30, 5
		default:
			t.Errorf("unexpected request #%d", call)
			return "", "", 0, 0
		}
	})
	defer closeSrv()

	_, _, err := client.ChatWithLimit(context.Background(), "system prompt", "initial message",
		func(string, json.RawMessage) (string, error) { return "ok", nil }, 5)
	if err != nil {
		t.Fatalf("ChatWithLimit: %v", err)
	}
	if len(*reqs) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(*reqs))
	}
	for i, req := range *reqs {
		if isCompressionRequest(req) {
			t.Fatalf("request #%d should not be a compression request when threshold is 0", i+1)
		}
	}
}

func TestChatLoopFallsBackToMessageCountWithoutUsage(t *testing.T) {
	compressionCall := 0
	client, reqs, closeSrv := newMockClient(t, 64000, func(call int, req recordedChatRequest) (string, string, int, int) {
		if isCompressionRequest(req) {
			compressionCall = call
			return "summary", "", 0, 0
		}
		if call >= 14 {
			return "", "FinishReview", 0, 0
		}
		return "", "GetMoreChanges", 0, 0
	})
	defer closeSrv()

	_, _, err := client.ChatWithLimit(context.Background(), "system prompt", "initial message",
		func(string, json.RawMessage) (string, error) { return "ok", nil }, 20)
	if err != nil {
		t.Fatalf("ChatWithLimit: %v", err)
	}
	if compressionCall != 13 {
		t.Fatalf("expected compression on request #13 (fallback message threshold), got #%d", compressionCall)
	}
	if len(*reqs) != 14 {
		t.Fatalf("expected 14 requests, got %d", len(*reqs))
	}
	if got := len((*reqs)[13].Messages); got != 3 {
		t.Fatalf("expected compressed history of 3 messages after fallback, got %d", got)
	}
}
