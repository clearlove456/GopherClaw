package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	ModelID      string
	SystemPrompt string
	MaxTokens    int
	Messages     []Message
}

type Result struct {
	StopReason    string
	AssistantText string
}

type ChatClient interface {
	Create(ctx context.Context, req Request) (Result, error)
}

type OpenAICompatClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewOpenAICompatClient(apiKey, baseURL string) *OpenAICompatClient {
	return &OpenAICompatClient{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: strings.TrimSpace(baseURL),
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

type chatCompletionRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens,omitempty"`
}

type chatCompletionResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []any  `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *OpenAICompatClient) Create(ctx context.Context, req Request) (Result, error) {
	if c.apiKey == "" {
		return Result{}, fmt.Errorf("missing OPENAI_API_KEY")
	}

	payload := chatCompletionRequest{
		Model:     req.ModelID,
		MaxTokens: req.MaxTokens,
	}

	if req.SystemPrompt != "" {
		payload.Messages = append(payload.Messages, Message{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}
	payload.Messages = append(payload.Messages, req.Messages...)

	body, err := json.Marshal(payload)
	if err != nil {
		return Result{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, buildChatCompletionsURL(c.baseURL), bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Result{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed chatCompletionResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return Result{}, fmt.Errorf("unmarshal response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return Result{}, fmt.Errorf("empty choices in response")
	}

	choice := parsed.Choices[0]
	stopReason := mapFinishReason(choice.FinishReason, len(choice.Message.ToolCalls))

	return Result{
		StopReason:    stopReason,
		AssistantText: choice.Message.Content,
	}, nil
}

func buildChatCompletionsURL(baseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return "https://api.openai.com/v1/chat/completions"
	}
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/chat/completions"
	}
	return base + "/v1/chat/completions"
}

func mapFinishReason(finishReason string, toolCalls int) string {
	if toolCalls > 0 || finishReason == "tool_calls" {
		return "tool_use"
	}

	switch finishReason {
	case "", "stop", "length", "content_filter":
		return "end_turn"
	default:
		return finishReason
	}
}
