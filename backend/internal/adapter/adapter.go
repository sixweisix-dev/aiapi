package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ChatAdapter converts between OpenAI format and provider-native formats.
type ChatAdapter interface {
	// ConvertReq transforms an OpenAIRequest into the provider's request body.
	ConvertReq(req *OpenAIRequest) ([]byte, error)
	// ConvertResp transforms the provider's response body into OpenAIResponse.
	ConvertResp(body []byte, model string) (*OpenAIResponse, error)
	// ConvertStream transforms one SSE data line from the provider into
	// zero or more OpenAI-format SSE data lines. Returns nil when the stream ends.
	ConvertStream(data []byte, model string) ([]OpenAIStreamChunk, bool, error)
}

var registry = map[string]ChatAdapter{
	"openai":    &OpenAIAdapter{},
	"anthropic": &AnthropicAdapter{},
	"google":    &GeminiAdapter{},
}

func GetAdapter(provider string) (ChatAdapter, bool) {
	a, ok := registry[strings.ToLower(provider)]
	return a, ok
}

func RegisterAdapter(provider string, a ChatAdapter) {
	registry[strings.ToLower(provider)] = a
}

// --- helpers ---

func parseSSELine(data []byte) ([]byte, bool) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, false
	}
	if bytes.HasPrefix(trimmed, []byte("data: ")) {
		body := bytes.TrimPrefix(trimmed, []byte("data: "))
		if string(body) == "[DONE]" {
			return nil, true
		}
		return body, false
	}
	return nil, false
}

func WriteSSEChunk(w io.Writer, data []byte) error {
	_, err := fmt.Fprintf(w, "data: %s\n\n", data)
	return err
}

func WriteSSEDone(w io.Writer) error {
	_, err := fmt.Fprintf(w, "data: [DONE]\n\n")
	return err
}

// errResp creates a minimal OpenAI error response from upstream error body.
func errResp(body []byte, model string) *OpenAIResponse {
	msg := string(body)
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return &OpenAIResponse{
		ID:     "error",
		Object: "chat.completion",
		Model:  model,
		Choices: []Choice{{
			Index: 0,
			Message: &Message{
				Role:    "assistant",
				Content: fmt.Sprintf("upstream error: %s", msg),
			},
			FinishReason: "error",
		}},
	}
}

// errChunk creates a minimal error SSE chunk.
func errChunk(model, msg string) OpenAIStreamChunk {
	return OpenAIStreamChunk{
		ID:     "error", Object: "chat.completion.chunk", Model: model,
		Choices: []StreamChoice{{Index: 0, Delta: Delta{Content: fmt.Sprintf("upstream error: %s", msg)}, FinishReason: "error"}},
	}
}

// --- JSON helpers ---

func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
