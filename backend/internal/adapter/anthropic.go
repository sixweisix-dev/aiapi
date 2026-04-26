package adapter

import (
	"encoding/json"
	"fmt"
	"strings"
)

type anthropicReq struct {
	Model       string            `json:"model"`
	MaxTokens   int               `json:"max_tokens"`
	Messages    []anthropicMsg    `json:"messages"`
	System      string            `json:"system,omitempty"`
	Stream      bool              `json:"stream"`
	Temperature *float64          `json:"temperature,omitempty"`
	TopP        *float64          `json:"top_p,omitempty"`
	Stop        []string          `json:"stop_sequences,omitempty"`
}

type anthropicMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResp struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Role       string            `json:"role"`
	Content    []anthropicBlock  `json:"content"`
	StopReason string            `json:"stop_reason"`
	Usage      *anthropicUsage   `json:"usage"`
}

type anthropicBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicStreamEvent struct {
	Type         string           `json:"type"`
	Index        int              `json:"index,omitempty"`
	Delta        *anthropicDelta  `json:"delta,omitempty"`
	ContentBlock *anthropicBlock  `json:"content_block,omitempty"`
	Message      *anthropicResp   `json:"message,omitempty"`
	Usage        *anthropicUsage  `json:"usage,omitempty"`
}

type anthropicDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	StopReason  string `json:"stop_reason,omitempty"`
	StopSeq     string `json:"stop_sequence,omitempty"`
}

type AnthropicAdapter struct{}

func (a *AnthropicAdapter) ConvertReq(req *OpenAIRequest) ([]byte, error) {
	maxTokens := 1024
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	var system string
	messages := make([]anthropicMsg, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "assistant"
		}
		messages = append(messages, anthropicMsg{Role: role, Content: m.Content})
	}

	ar := anthropicReq{
		Model:       mapModel(req.Model),
		MaxTokens:   maxTokens,
		Messages:    messages,
		System:      system,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        req.Stop,
	}

	// Anthropic requires at least one message
	if len(ar.Messages) == 0 {
		ar.Messages = []anthropicMsg{{Role: "user", Content: "."}}
	}

	return json.Marshal(ar)
}

func (a *AnthropicAdapter) ConvertResp(body []byte, model string) (*OpenAIResponse, error) {
	var ar anthropicResp
	if err := json.Unmarshal(body, &ar); err != nil {
		return errResp(body, model), nil
	}

	content := ""
	for _, b := range ar.Content {
		if b.Type == "text" {
			content += b.Text
		}
	}

	promptTokens := 0
	completionTokens := 0
	if ar.Usage != nil {
		promptTokens = ar.Usage.InputTokens
		completionTokens = ar.Usage.OutputTokens
	}

	finishReason := mapStopReason(ar.StopReason)

	return &OpenAIResponse{
		ID:      ar.ID,
		Object:  "chat.completion",
		Created: zeroTime.Unix(),
		Model:   model,
		Choices: []Choice{{
			Index: 0,
			Message: &Message{Role: "assistant", Content: content},
			FinishReason: finishReason,
		}},
		Usage: &Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}, nil
}

func (a *AnthropicAdapter) ConvertStream(data []byte, model string) ([]OpenAIStreamChunk, bool, error) {
	body, done := parseSSELine(data)
	if body == nil {
		return nil, done, nil
	}

	var evt anthropicStreamEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return nil, false, nil
	}

	var chunks []OpenAIStreamChunk

	switch evt.Type {
	case "message_start":
		if evt.Message != nil {
			role := "assistant"
			promptTokens := 0
			if evt.Message.Usage != nil {
				promptTokens = evt.Message.Usage.InputTokens
			}
			chunks = append(chunks, OpenAIStreamChunk{
				ID: evt.Message.ID, Object: "chat.completion.chunk", Created: zeroTime.Unix(), Model: model,
				Choices: []StreamChoice{{Index: 0, Delta: Delta{Role: role}, FinishReason: ""}},
			})
			// Emit usage metadata in a separate chunk
			if promptTokens > 0 {
				chunks = append(chunks, OpenAIStreamChunk{
					ID: evt.Message.ID, Object: "chat.completion.chunk", Created: zeroTime.Unix(), Model: model,
					Choices: []StreamChoice{{Index: 0, Delta: Delta{Content: fmt.Sprintf("\n%%USAGE%%:{\"prompt_tokens\":%d}", promptTokens)}}},
				})
			}
		}

	case "content_block_delta":
		if evt.Delta != nil && evt.Delta.Type == "text_delta" {
			chunks = append(chunks, OpenAIStreamChunk{
				ID: "", Object: "chat.completion.chunk", Created: zeroTime.Unix(), Model: model,
				Choices: []StreamChoice{{Index: evt.Index, Delta: Delta{Content: evt.Delta.Text}}},
			})
		}

	case "message_delta":
		finishReason := ""
		if evt.Delta != nil {
			finishReason = mapStopReason(evt.Delta.StopReason)
		}
		chunks = append(chunks, OpenAIStreamChunk{
			ID: "", Object: "chat.completion.chunk", Created: zeroTime.Unix(), Model: model,
			Choices: []StreamChoice{{Index: 0, Delta: Delta{}, FinishReason: finishReason}},
		})

	case "message_stop":
		return nil, true, nil
	}

	return chunks, false, nil
}

func mapModel(openAIModel string) string {
	// OpenAI model name → Anthropic model name
	switch openAIModel {
	case "claude-3-opus", "claude-3-sonnet", "claude-3-haiku",
		"claude-3-5-sonnet", "claude-3-5-haiku", "claude-opus-4", "claude-sonnet-4":
		return openAIModel
	}
	// Passthrough — assume the client used the right name
	return openAIModel
}

func mapStopReason(anthropic string) string {
	switch anthropic {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	default:
		return strings.ToLower(anthropic)
	}
}
