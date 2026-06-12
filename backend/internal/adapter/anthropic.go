package adapter

import (
	"encoding/json"
	"fmt"
	"strings"
)

type anthropicReq struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	Messages    []anthropicMsg  `json:"messages"`
	System      string          `json:"system,omitempty"`
	Stream      bool            `json:"stream"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Stop        []string        `json:"stop_sequences,omitempty"`
	Tools       []anthropicTool `json:"tools,omitempty"`
	ToolChoice  interface{}     `json:"tool_choice,omitempty"`
}

type anthropicMsg struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type anthropicContent struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
	Content   interface{}            `json:"content,omitempty"`
}

type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type anthropicResp struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"`
	Role       string           `json:"role"`
	Content    []anthropicBlock `json:"content"`
	StopReason string           `json:"stop_reason"`
	Usage      *anthropicUsage  `json:"usage"`
}

type anthropicBlock struct {
	Type  string                 `json:"type"`
	Text  string                 `json:"text"`
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicStreamEvent struct {
	Type         string          `json:"type"`
	Index        int             `json:"index,omitempty"`
	Delta        *anthropicDelta `json:"delta,omitempty"`
	ContentBlock *anthropicBlock `json:"content_block,omitempty"`
	Message      *anthropicResp  `json:"message,omitempty"`
	Usage        *anthropicUsage `json:"usage,omitempty"`
}

type anthropicDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	PartialJSON string `json:"partial_json,omitempty"`
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
			system = m.ContentString()
			continue
		}
		if m.Role == "tool" {
			messages = append(messages, anthropicMsg{
				Role: "user",
				Content: []anthropicContent{{
					Type:      "tool_result",
					ToolUseID: m.ToolCallID,
					Content:   m.ContentString(),
				}},
			})
			continue
		}
		if m.Role == "assistant" && m.ToolCalls != nil {
			blocks := []anthropicContent{}
			if text := m.ContentString(); text != "" {
				blocks = append(blocks, anthropicContent{Type: "text", Text: text})
			}
			tcArr, _ := m.ToolCalls.([]interface{})
			for _, tc := range tcArr {
				tcMap, _ := tc.(map[string]interface{})
				if tcMap == nil {
					continue
				}
				id, _ := tcMap["id"].(string)
				fn, _ := tcMap["function"].(map[string]interface{})
				if fn == nil {
					continue
				}
				name, _ := fn["name"].(string)
				argsStr, _ := fn["arguments"].(string)
				var input map[string]interface{}
				if argsStr != "" {
					_ = json.Unmarshal([]byte(argsStr), &input)
				}
				if input == nil {
					input = map[string]interface{}{}
				}
				blocks = append(blocks, anthropicContent{
					Type: "tool_use", ID: id, Name: name, Input: input,
				})
			}
			messages = append(messages, anthropicMsg{Role: "assistant", Content: blocks})
			continue
		}
		messages = append(messages, anthropicMsg{Role: m.Role, Content: m.ContentString()})
	}

	var tools []anthropicTool
	if req.Tools != nil {
		rawTools, _ := req.Tools.([]interface{})
		for _, t := range rawTools {
			tMap, _ := t.(map[string]interface{})
			if tMap == nil {
				continue
			}
			fn, _ := tMap["function"].(map[string]interface{})
			if fn == nil {
				continue
			}
			name, _ := fn["name"].(string)
			desc, _ := fn["description"].(string)
			params, _ := fn["parameters"].(map[string]interface{})
			if params == nil {
				params = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
			}
			tools = append(tools, anthropicTool{Name: name, Description: desc, InputSchema: params})
		}
	}

	var toolChoice interface{}
	if req.ToolChoice != nil {
		switch v := req.ToolChoice.(type) {
		case string:
			switch v {
			case "auto":
				toolChoice = map[string]interface{}{"type": "auto"}
			case "none":
				tools = nil
			case "required":
				toolChoice = map[string]interface{}{"type": "any"}
			}
		case map[string]interface{}:
			if v["type"] == "function" {
				if fn, ok := v["function"].(map[string]interface{}); ok {
					if name, ok := fn["name"].(string); ok {
						toolChoice = map[string]interface{}{"type": "tool", "name": name}
					}
				}
			}
		}
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
		Tools:       tools,
		ToolChoice:  toolChoice,
	}

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
	var toolCalls []map[string]interface{}
	for _, b := range ar.Content {
		if b.Type == "text" {
			content += b.Text
		}
		if b.Type == "tool_use" {
			argsJSON, _ := json.Marshal(b.Input)
			toolCalls = append(toolCalls, map[string]interface{}{
				"id":   b.ID,
				"type": "function",
				"function": map[string]interface{}{
					"name":      b.Name,
					"arguments": string(argsJSON),
				},
			})
		}
	}

	promptTokens := 0
	completionTokens := 0
	if ar.Usage != nil {
		promptTokens = ar.Usage.InputTokens
		completionTokens = ar.Usage.OutputTokens
	}

	finishReason := mapStopReason(ar.StopReason)
	if ar.StopReason == "tool_use" {
		finishReason = "tool_calls"
	}

	msg := &Message{Role: "assistant", Content: content}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}

	return &OpenAIResponse{
		ID:      ar.ID,
		Object:  "chat.completion",
		Created: zeroTime.Unix(),
		Model:   model,
		Choices: []Choice{{
			Index:        0,
			Message:      msg,
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
			promptTokens := 0
			if evt.Message.Usage != nil {
				promptTokens = evt.Message.Usage.InputTokens
			}
			chunks = append(chunks, OpenAIStreamChunk{
				ID: evt.Message.ID, Object: "chat.completion.chunk", Created: zeroTime.Unix(), Model: model,
				Choices: []StreamChoice{{Index: 0, Delta: Delta{Role: "assistant"}, FinishReason: ""}},
			})
			if promptTokens > 0 {
				chunks = append(chunks, OpenAIStreamChunk{
					ID: evt.Message.ID, Object: "chat.completion.chunk", Created: zeroTime.Unix(), Model: model,
					Choices: []StreamChoice{{Index: 0, Delta: Delta{Content: fmt.Sprintf("\n%%USAGE%%:{\"prompt_tokens\":%d}", promptTokens)}}},
				})
			}
		}

	case "content_block_start":
		if evt.ContentBlock != nil && evt.ContentBlock.Type == "tool_use" {
			toolCalls := []map[string]interface{}{{
				"index": evt.Index,
				"id":    evt.ContentBlock.ID,
				"type":  "function",
				"function": map[string]interface{}{
					"name":      evt.ContentBlock.Name,
					"arguments": "",
				},
			}}
			chunks = append(chunks, OpenAIStreamChunk{
				Object: "chat.completion.chunk", Created: zeroTime.Unix(), Model: model,
				Choices: []StreamChoice{{Index: 0, Delta: Delta{ToolCalls: toolCalls}}},
			})
		}

	case "content_block_delta":
		if evt.Delta != nil && evt.Delta.Type == "text_delta" {
			chunks = append(chunks, OpenAIStreamChunk{
				Object: "chat.completion.chunk", Created: zeroTime.Unix(), Model: model,
				Choices: []StreamChoice{{Index: evt.Index, Delta: Delta{Content: evt.Delta.Text}}},
			})
		}
		if evt.Delta != nil && evt.Delta.Type == "input_json_delta" {
			toolCalls := []map[string]interface{}{{
				"index": evt.Index,
				"function": map[string]interface{}{
					"arguments": evt.Delta.PartialJSON,
				},
			}}
			chunks = append(chunks, OpenAIStreamChunk{
				Object: "chat.completion.chunk", Created: zeroTime.Unix(), Model: model,
				Choices: []StreamChoice{{Index: 0, Delta: Delta{ToolCalls: toolCalls}}},
			})
		}

	case "message_delta":
		finishReason := ""
		if evt.Delta != nil {
			finishReason = mapStopReason(evt.Delta.StopReason)
			if evt.Delta.StopReason == "tool_use" {
				finishReason = "tool_calls"
			}
		}
		chunks = append(chunks, OpenAIStreamChunk{
			Object: "chat.completion.chunk", Created: zeroTime.Unix(), Model: model,
			Choices: []StreamChoice{{Index: 0, Delta: Delta{}, FinishReason: finishReason}},
		})

	case "message_stop":
		return nil, true, nil
	}

	return chunks, false, nil
}

func mapModel(openAIModel string) string {
	switch openAIModel {
	case "claude-3-opus", "claude-3-sonnet", "claude-3-haiku",
		"claude-3-5-sonnet", "claude-3-5-haiku", "claude-opus-4", "claude-sonnet-4":
		return openAIModel
	}
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

