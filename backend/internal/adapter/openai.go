package adapter

import (
	"encoding/json"
	"time"
)

// OpenAIAdapter is a passthrough — request & response already in OpenAI format.
type OpenAIAdapter struct{}

func (a *OpenAIAdapter) ConvertReq(req *OpenAIRequest) ([]byte, error) {
	return json.Marshal(req)
}

func (a *OpenAIAdapter) ConvertResp(body []byte, model string) (*OpenAIResponse, error) {
	var resp OpenAIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return errResp(body, model), nil
	}
	resp.Model = model
	return &resp, nil
}

func (a *OpenAIAdapter) ConvertStream(data []byte, model string) ([]OpenAIStreamChunk, bool, error) {
	body, done := parseSSELine(data)
	if body == nil {
		return nil, done, nil
	}
	var chunk OpenAIStreamChunk
	if err := json.Unmarshal(body, &chunk); err != nil {
		return nil, false, nil
	}
	chunk.Model = model
	return []OpenAIStreamChunk{chunk}, false, nil
}

// --- shared ---

var zeroTime = time.Now()
