package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// OpenAI-compatible handlers for clients that expect /v1/chat/completions and /v1/models.

const (
	defaultOpenAIModel   = "nougat-default"
	defaultOpenAITimeout = 600
)

func openAIModelList() []openAIModel {
	raw := strings.TrimSpace(os.Getenv("NOUGAT_OPENAI_MODELS"))
	if raw == "" {
		return []openAIModel{{
			ID:        defaultOpenAIModel,
			Object:    "model",
			Created:   1118636800,
			OwnedBy:   "nougat",
		}}
	}
	var out []openAIModel
	created := time.Now().Unix()
	for _, id := range strings.Split(raw, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out = append(out, openAIModel{
			ID:        id,
			Object:    "model",
			Created:   created,
			OwnedBy:   "nougat",
		})
	}
	if len(out) == 0 {
		return []openAIModel{{ID: defaultOpenAIModel, Object: "model", Created: 1118636800, OwnedBy: "nougat"}}
	}
	return out
}

func modelAllowed(model string) bool {
	if strings.TrimSpace(model) == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("NOUGAT_OPENAI_MODEL_STRICT"))) {
	case "1", "true", "yes":
		for _, m := range openAIModelList() {
			if m.ID == model {
				return true
			}
		}
		return false
	default:
		return true
	}
}

type openAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type openAIModelsResponse struct {
	Object string        `json:"object"`
	Data   []openAIModel `json:"data"`
}

func (s *server) handleOpenAIModels(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	keyID := apiKeyIDFromContext(r.Context())
	httpStatus := http.StatusOK
	defer func() {
		if keyID == "" {
			return
		}
		_ = recordAPIUsage(s.db, keyID, UsageEvent{
			At:         time.Now().UTC(),
			Path:       r.URL.Path,
			Method:     r.Method,
			HTTPStatus: httpStatus,
			DurationMs: time.Since(t0).Milliseconds(),
		})
	}()

	if r.Method != http.MethodGet {
		httpStatus = http.StatusMethodNotAllowed
		writeOpenAIAPIError(w, httpStatus, "method not allowed", "invalid_request_error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(openAIModelsResponse{
		Object: "list",
		Data:   openAIModelList(),
	})
}

type openAIChatRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool           `json:"stream"`
}

type openAIMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type openAIAPIError struct {
	Error openAIErrBody `json:"error"`
}

type openAIErrBody struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code,omitempty"`
}

func writeOpenAIAPIError(w http.ResponseWriter, status int, msg, typ string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(openAIAPIError{
		Error: openAIErrBody{Message: msg, Type: typ},
	})
}

type openAIChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
}

type openAIChoice struct {
	Index        int           `json:"index"`
	Message      openAIRespMsg `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type openAIRespMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (s *server) handleOpenAIChatCompletions(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()
	keyID := apiKeyIDFromContext(r.Context())
	var (
		httpStatus       = http.StatusOK
		errMsg           string
		modelName        string
		finalUsage       openAIUsage
	)

	defer func() {
		if keyID == "" {
			return
		}
		_ = recordAPIUsage(s.db, keyID, UsageEvent{
			At:               time.Now().UTC(),
			Path:             r.URL.Path,
			Method:           r.Method,
			HTTPStatus:       httpStatus,
			Model:            modelName,
			PromptTokens:     finalUsage.PromptTokens,
			CompletionTokens: finalUsage.CompletionTokens,
			DurationMs:       time.Since(t0).Milliseconds(),
			ErrorSnippet:     errSnippet(errMsg, 240),
		})
	}()

	if r.Method != http.MethodPost {
		httpStatus = http.StatusMethodNotAllowed
		writeOpenAIAPIError(w, httpStatus, "method not allowed", "invalid_request_error")
		return
	}

	var req openAIChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpStatus = http.StatusBadRequest
		writeOpenAIAPIError(w, httpStatus, "Invalid JSON body", "invalid_request_error")
		return
	}
	if len(req.Messages) == 0 {
		httpStatus = http.StatusBadRequest
		writeOpenAIAPIError(w, httpStatus, "messages is required", "invalid_request_error")
		return
	}
	if req.Model == "" {
		httpStatus = http.StatusBadRequest
		writeOpenAIAPIError(w, httpStatus, "model is required", "invalid_request_error")
		return
	}
	modelName = req.Model
	if !modelAllowed(req.Model) {
		httpStatus = http.StatusBadRequest
		writeOpenAIAPIError(w, httpStatus,
			fmt.Sprintf("model %q not allowed with NOUGAT_OPENAI_MODEL_STRICT; see GET /v1/models", req.Model),
			"invalid_request_error")
		return
	}

	prompt, err := messagesToPrompt(req.Messages)
	if err != nil {
		httpStatus = http.StatusBadRequest
		writeOpenAIAPIError(w, httpStatus, err.Error(), "invalid_request_error")
		return
	}

	workDir := strings.TrimSpace(os.Getenv("NOUGAT_OPENAI_WORKDIR"))
	if workDir == "" {
		workDir = "."
	}
	timeoutSec := defaultOpenAITimeout
	if v := strings.TrimSpace(os.Getenv("NOUGAT_OPENAI_TIMEOUT_SEC")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeoutSec = n
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	stdout, stderr, _, status := runLLM(ctx, workDir, prompt)

	if ctx.Err() == context.DeadlineExceeded {
		httpStatus = http.StatusGatewayTimeout
		errMsg = "LLM subprocess timed out"
		writeOpenAIAPIError(w, httpStatus, errMsg, "server_error")
		return
	}
	if status != StatusCompleted {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = "LLM subprocess failed"
		}
		httpStatus = http.StatusBadGateway
		errMsg = msg
		writeOpenAIAPIError(w, httpStatus, msg, "server_error")
		return
	}

	content := strings.TrimSpace(stdout)
	finalUsage = estimateTokenUsage(prompt, content)

	respID, err := newChatCompletionID()
	if err != nil {
		httpStatus = http.StatusInternalServerError
		errMsg = "could not generate id"
		writeOpenAIAPIError(w, httpStatus, errMsg, "server_error")
		return
	}
	created := time.Now().Unix()

	if req.Stream {
		writeOpenAIStream(w, respID, created, req.Model, content)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(openAIChatResponse{
		ID:      respID,
		Object:  "chat.completion",
		Created: created,
		Model:   req.Model,
		Choices: []openAIChoice{{
			Index: 0,
			Message: openAIRespMsg{
				Role:    "assistant",
				Content: content,
			},
			FinishReason: "stop",
		}},
		Usage: finalUsage,
	})
}

func messagesToPrompt(msgs []openAIMessage) (string, error) {
	var b strings.Builder
	for i, m := range msgs {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		switch role {
		case "system", "user", "assistant", "tool", "developer":
		case "":
			return "", fmt.Errorf("message[%d]: missing role", i)
		default:
			return "", fmt.Errorf("unsupported role %q", m.Role)
		}
		text, err := messageContentToString(m.Content)
		if err != nil {
			return "", fmt.Errorf("message[%d] content: %w", i, err)
		}
		if role == "tool" {
			fmt.Fprintf(&b, "### tool_result\n%s\n\n", text)
			continue
		}
		fmt.Fprintf(&b, "### %s\n%s\n\n", role, text)
	}
	return strings.TrimSpace(b.String()), nil
}

func messageContentToString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", fmt.Errorf("content must be a string or a text parts array")
	}
	var b strings.Builder
	for _, p := range parts {
		switch p.Type {
		case "text":
			b.WriteString(p.Text)
		default:
			// skip image_url etc.
		}
	}
	return b.String(), nil
}

func estimateTokenUsage(prompt, completion string) openAIUsage {
	p := max(1, len([]rune(prompt))/4)
	c := max(1, len([]rune(completion))/4)
	return openAIUsage{
		PromptTokens:     p,
		CompletionTokens: c,
		TotalTokens:      p + c,
	}
}

func writeOpenAIStream(w http.ResponseWriter, id string, created int64, model, fullContent string) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeOpenAIAPIError(w, http.StatusInternalServerError, "streaming not supported", "server_error")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	send := func(choices []map[string]any) {
		raw, _ := json.Marshal(map[string]any{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   model,
			"choices": choices,
		})
		fmt.Fprintf(w, "data: %s\n\n", raw)
		fl.Flush()
	}

	send([]map[string]any{{
		"index": 0,
		"delta": map[string]any{"role": "assistant"},
	}})
	send([]map[string]any{{
		"index": 0,
		"delta": map[string]any{"content": fullContent},
	}})
	send([]map[string]any{{
		"index":         0,
		"delta":         map[string]any{},
		"finish_reason": "stop",
	}})
	fmt.Fprintf(w, "data: [DONE]\n\n")
	fl.Flush()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func newChatCompletionID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "chatcmpl-" + hex.EncodeToString(b[:]), nil
}
