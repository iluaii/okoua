// SPDX-License-Identifier: AGPL-3.0-or-later

// Package ai answers fixed questions about a news post with an LLM, grounded only in the
// post's text. There is no free-form chat: the question is either "summary" or one of the
// follow-up questions the model suggested for that post.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const openRouterURL = "https://openrouter.ai/api/v1/chat/completions"

// OpenRouter calls models through openrouter.ai, trying them in order.
type OpenRouter struct {
	Key    string
	Models []string
	URL    string
	Client *http.Client
}

func NewOpenRouter(key string, models []string) *OpenRouter {
	return &OpenRouter{Key: key, Models: models, URL: openRouterURL, Client: &http.Client{Timeout: 60 * time.Second}}
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
	Reasoning   reasoning `json:"reasoning"`
}

// reasoning asks reasoning models to think little and not return their thoughts:
// reasoning tokens count against max_tokens and could leave no room for the answer.
type reasoning struct {
	Effort  string `json:"effort"`
	Exclude bool   `json:"exclude"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message      message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
	} `json:"error"`
}

// complete sends one prompt to one model and returns the text of the reply.
func (o *OpenRouter) complete(ctx context.Context, model, prompt string) (string, error) {
	body, _ := json.Marshal(chatRequest{
		Model: model,
		// Everything goes in one user message: some models (Gemma) don't accept a system role.
		Messages:    []message{{Role: "user", Content: prompt}},
		Temperature: 0.2,
		MaxTokens:   1500,
		Reasoning:   reasoning{Effort: "low", Exclude: true},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.URL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+o.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Title", "alert-concept")
	resp, err := o.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", fmt.Errorf("%s: %s: bad response", model, resp.Status)
	}
	if cr.Error != nil {
		return "", fmt.Errorf("%s: %s: %s", model, resp.Status, cr.Error.Message)
	}
	if resp.StatusCode != http.StatusOK || len(cr.Choices) == 0 {
		return "", fmt.Errorf("%s: %s: no answer", model, resp.Status)
	}
	if c := cr.Choices[0]; c.Message.Content == "" {
		return "", fmt.Errorf("%s (via %s): empty answer, finish_reason=%s", model, cr.Model, c.FinishReason)
	}
	return cr.Choices[0].Message.Content, nil
}

// ErrNoModel means every model failed or gave an unusable answer.
var ErrNoModel = errors.New("ai: no model produced a valid answer")
