package adapter

// OpenAI-compatible request/response types shared across adapters.

type OpenAIRequest struct {
	Model       string       `json:"model"`
	Messages    []Message    `json:"messages"`
	Stream      bool         `json:"stream"`
	Temperature *float64     `json:"temperature,omitempty"`
	MaxTokens   *int         `json:"max_tokens,omitempty"`
	TopP        *float64     `json:"top_p,omitempty"`
	Stop        []string     `json:"stop,omitempty"`
	User        string       `json:"user,omitempty"`
}

type Message struct {
	Role       string        `json:"role"`
	Content    interface{}   `json:"content"`
	Name       string        `json:"name,omitempty"`
}

type OpenAIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

type Choice struct {
	Index        int      `json:"index"`
	Message      *Message `json:"message,omitempty"`
	Delta        *Delta   `json:"delta,omitempty"`
	FinishReason string   `json:"finish_reason,omitempty"`
}

type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIStreamChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []StreamChoice `json:"choices"`
	Usage   *Usage         `json:"usage,omitempty"`
}

type StreamChoice struct {
	Index        int      `json:"index"`
	Delta        Delta    `json:"delta"`
	FinishReason string   `json:"finish_reason,omitempty"`
}

type ModelInfo struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	Created     int64  `json:"created"`
	OwnedBy     string `json:"owned_by"`
}

type ModelsListResponse struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}


// ContentString extracts text from Content (handles string or OpenAI multimodal array).
func (m Message) ContentString() string {
	if s, ok := m.Content.(string); ok {
		return s
	}
	if arr, ok := m.Content.([]interface{}); ok {
		result := ""
		for _, p := range arr {
			if pm, ok := p.(map[string]interface{}); ok {
				if pm["type"] == "text" {
					if t, ok := pm["text"].(string); ok {
						if result != "" {
							result += " "
						}
						result += t
					}
				}
			}
		}
		return result
	}
	return ""
}

// CountImages counts image_url parts in Content array (OpenAI multimodal format).
func (m Message) CountImages() int {
	arr, ok := m.Content.([]interface{})
	if !ok {
		return 0
	}
	n := 0
	for _, p := range arr {
		if pm, ok := p.(map[string]interface{}); ok {
			if pm["type"] == "image_url" {
				n++
			}
		}
	}
	return n
}
