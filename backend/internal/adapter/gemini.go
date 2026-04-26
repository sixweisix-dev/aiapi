package adapter

import (
	"encoding/json"
	"fmt"
	"strings"
)

type geminiReq struct {
	Contents        []geminiContent      `json:"contents"`
	GenerationConfig *geminiGenConfig    `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"topP,omitempty"`
	MaxOutputTokens  *int     `json:"maxOutputTokens,omitempty"`
	StopSequences    []string `json:"stopSequences,omitempty"`
}

type geminiResp struct {
	Candidates []geminiCandidate `json:"candidates"`
	UsageMeta  *geminiUsage      `json:"usageMetadata"`
}

type geminiCandidate struct {
	Content      geminiContent  `json:"content"`
	FinishReason string         `json:"finishReason"`
}

type geminiUsage struct {
	PromptTokens     int `json:"promptTokenCount"`
	CandidateTokens  int `json:"candidatesTokenCount"`
	TotalTokens      int `json:"totalTokenCount"`
}

// Gemini streaming event types
type geminiStreamChunk struct {
	Candidates []geminiCandidate `json:"candidates"`
	UsageMeta  *geminiUsage      `json:"usageMetadata"`
}

type GeminiAdapter struct{}

func (a *GeminiAdapter) ConvertReq(req *OpenAIRequest) ([]byte, error) {
	contents := convertMessages(req.Messages)

	genCfg := &geminiGenConfig{
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxOutputTokens: req.MaxTokens,
		StopSequences:   req.Stop,
	}

	if genCfg.Temperature == nil && genCfg.TopP == nil && genCfg.MaxOutputTokens == nil && len(genCfg.StopSequences) == 0 {
		genCfg = nil
	}

	gr := geminiReq{
		Contents:        contents,
		GenerationConfig: genCfg,
	}

	return json.Marshal(gr)
}

func (a *GeminiAdapter) ConvertResp(body []byte, model string) (*OpenAIResponse, error) {
	var gr geminiResp
	if err := json.Unmarshal(body, &gr); err != nil {
		return errResp(body, model), nil
	}

	content := ""
	for _, c := range gr.Candidates {
		for _, p := range c.Content.Parts {
			content += p.Text
		}
	}

	promptTokens := 0
	completionTokens := 0
	if gr.UsageMeta != nil {
		promptTokens = gr.UsageMeta.PromptTokens
		completionTokens = gr.UsageMeta.CandidateTokens
	}

	finishReason := mapGeminiFinishReason(gr.Candidates)

	return &OpenAIResponse{
		ID:      fmt.Sprintf("gemini-%d", zeroTime.Unix()),
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

func (a *GeminiAdapter) ConvertStream(data []byte, model string) ([]OpenAIStreamChunk, bool, error) {
	body, done := parseSSELine(data)
	if body == nil {
		return nil, done, nil
	}

	var chunk geminiStreamChunk
	if err := json.Unmarshal(body, &chunk); err != nil {
		return nil, false, nil
	}

	var content string
	finishReason := ""
	for _, c := range chunk.Candidates {
		for _, p := range c.Content.Parts {
			content += p.Text
		}
		if c.FinishReason != "" {
			finishReason = mapGeminiFinish(c.FinishReason)
		}
	}

	chunks := []OpenAIStreamChunk{{
		ID: "", Object: "chat.completion.chunk", Created: zeroTime.Unix(), Model: model,
		Choices: []StreamChoice{{
			Index: 0,
			Delta: Delta{Content: content},
			FinishReason: finishReason,
		}},
	}}

	if finishReason != "" {
		return chunks, true, nil
	}
	return chunks, false, nil
}

func convertMessages(msgs []Message) []geminiContent {
	var contents []geminiContent

	// Gemini uses "user" / "model" roles
	for _, m := range msgs {
		if m.Role == "system" {
			contents = append(contents, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: fmt.Sprintf("[System instruction] %s", m.Content)}},
			})
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	if len(contents) == 0 {
		contents = []geminiContent{{Parts: []geminiPart{{Text: "."}}}}
	}

	return contents
}

func mapGeminiFinishReason(candidates []geminiCandidate) string {
	if len(candidates) == 0 {
		return "stop"
	}
	switch candidates[0].FinishReason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION", "OTHER":
		return "content_filter"
	default:
		return strings.ToLower(candidates[0].FinishReason)
	}
}

func mapGeminiFinish(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION", "OTHER":
		return "content_filter"
	default:
		return strings.ToLower(reason)
	}
}
