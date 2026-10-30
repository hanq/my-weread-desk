// Package ai talks to an OpenAI-compatible chat completion endpoint and turns
// retrieved passages into a grounded answer.
//
// Only the OpenAI protocol is implemented on purpose: one base URL and one model
// name cover OpenAI, DeepSeek, Moonshot, SiliconFlow, Ollama and LM Studio.
// Writing a per-vendor adapter set would add configuration surface without
// adding capability.
package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Message is one chat turn.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request is a single completion request.
type Request struct {
	Model       string
	Messages    []Message
	Temperature float64
	MaxTokens   int
}

// Provider is the whole surface the assistant needs.
type Provider interface {
	Chat(ctx context.Context, req Request, onDelta func(string)) (string, error)
}

// Config is the persisted connection settings.
type Config struct {
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float64
	MaxTokens   int
	HTTP        *http.Client
}

// OpenAI implements Provider against an OpenAI-compatible endpoint.
type OpenAI struct {
	cfg Config
}

// NewOpenAI creates a provider. MaxTokens has a floor so a bad settings value
// cannot produce empty answers.
func NewOpenAI(cfg Config) *OpenAI {
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: 180 * time.Second}
	}
	if cfg.MaxTokens < 128 {
		cfg.MaxTokens = 1200
	}
	return &OpenAI{cfg: cfg}
}

// Configured reports whether the provider can be used at all.
func (p *OpenAI) Configured() bool {
	return p != nil && strings.TrimSpace(p.cfg.BaseURL) != "" && strings.TrimSpace(p.cfg.Model) != ""
}

// Chat performs a streaming completion and returns the assembled answer. onDelta
// is called for every text fragment so the UI can render as it arrives; it may
// be nil.
func (p *OpenAI) Chat(ctx context.Context, req Request, onDelta func(string)) (string, error) {
	if !p.Configured() {
		return "", errors.New("未配置 AI 服务地址或模型")
	}
	if req.Model == "" {
		req.Model = p.cfg.Model
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = p.cfg.MaxTokens
	}

	payload := map[string]any{
		"model":       req.Model,
		"messages":    req.Messages,
		"stream":      true,
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint(p.cfg.BaseURL), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}

	resp, err := p.cfg.HTTP.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("连接 AI 服务失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("AI 服务返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	// A non-streaming reply is legal even when stream was requested. Detect it
	// by content type so a proxy that strips SSE still produces an answer.
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "event-stream") {
		return parseBuffered(resp.Body, onDelta)
	}
	return parseStream(resp.Body, onDelta)
}

func parseStream(r io.Reader, onDelta func(string)) (string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var out strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// Keep-alive comments and partial frames are not fatal.
			continue
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return out.String(), errors.New(chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		out.WriteString(delta)
		if onDelta != nil {
			onDelta(delta)
		}
	}
	if err := scanner.Err(); err != nil {
		return out.String(), err
	}
	if out.Len() == 0 {
		return "", errors.New("AI 服务没有返回内容")
	}
	return out.String(), nil
}

func parseBuffered(r io.Reader, onDelta func(string)) (string, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return "", err
	}
	if resp.Error != nil && resp.Error.Message != "" {
		return "", errors.New(resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("AI 服务返回为空")
	}
	text := resp.Choices[0].Message.Content
	if strings.TrimSpace(text) == "" {
		return "", errors.New("AI 服务返回为空")
	}
	if onDelta != nil {
		onDelta(text)
	}
	return text, nil
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// endpoint accepts either a host, a /v1 root, or a full completion URL. Ollama
// and LM Studio users habitually paste the full URL, while cloud vendors
// document only the host.
func endpoint(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/chat/completions"
	}
	return base + "/v1/chat/completions"
}
