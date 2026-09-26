package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string          `json:"model"`
	Messages    []chatMessage   `json:"messages"`
	Temperature float64         `json:"temperature"`
	Usage       map[string]bool `json:"usage,omitempty"`
	MaxTokens   int             `json:"max_tokens"`
}

type chatResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage json.RawMessage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type LLMUsage struct {
	PromptTokens     int      `json:"prompt_tokens,omitempty"`
	CompletionTokens int      `json:"completion_tokens,omitempty"`
	TotalTokens      int      `json:"total_tokens,omitempty"`
	CostUSD          *float64 `json:"cost_usd,omitempty"`
}

type LLMResult struct {
	Text  string
	Usage LLMUsage
}

func callLLM(s Seat, prompt string, timeout time.Duration, temperature float64, maxTokens int) (LLMResult, error) {
	key := os.Getenv(s.APIKeyEnv)
	if key == "" {
		return LLMResult{}, fmt.Errorf("env %s not set", s.APIKeyEnv)
	}
	if maxTokens < 1 {
		maxTokens = 4096
	}
	reqBody, err := json.Marshal(chatRequest{
		Model:       s.Model,
		Messages:    []chatMessage{{Role: "user", Content: prompt}},
		Temperature: temperature,
		Usage:       map[string]bool{"include": true},
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return LLMResult{}, err
	}
	url := strings.TrimRight(s.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return LLMResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+key)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return LLMResult{}, fmt.Errorf("%s: %w", s.Name, err)
	}
	defer resp.Body.Close()

	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return LLMResult{}, fmt.Errorf("%s: decode response (http %d): %w", s.Name, resp.StatusCode, err)
	}
	if cr.Error != nil {
		return LLMResult{}, fmt.Errorf("%s: API error: %s", s.Name, cr.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return LLMResult{}, fmt.Errorf("%s: http %d", s.Name, resp.StatusCode)
	}
	if len(cr.Choices) == 0 {
		return LLMResult{}, fmt.Errorf("%s: empty choices", s.Name)
	}
	var apiUsage struct {
		PromptTokens     int      `json:"prompt_tokens"`
		CompletionTokens int      `json:"completion_tokens"`
		TotalTokens      int      `json:"total_tokens"`
		Cost             *float64 `json:"cost"`
	}
	if len(cr.Usage) > 0 {
		_ = json.Unmarshal(cr.Usage, &apiUsage)
	}
	result := LLMResult{
		Text: strings.TrimSpace(cr.Choices[0].Message.Content),
		Usage: LLMUsage{
			PromptTokens:     apiUsage.PromptTokens,
			CompletionTokens: apiUsage.CompletionTokens,
			TotalTokens:      apiUsage.TotalTokens,
			CostUSD:          apiUsage.Cost,
		},
	}
	if cr.Choices[0].FinishReason == "length" {
		result.Text = ""
		return result, fmt.Errorf("%s: output reached --max-tokens %d; response is incomplete", s.Name, maxTokens)
	}
	if result.Text == "" {
		return result, fmt.Errorf("%s: empty response content", s.Name)
	}
	return result, nil
}
