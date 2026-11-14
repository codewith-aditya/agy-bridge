package translate

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
)

// dummySig is a stable placeholder thoughtSignature. Gemini 3 era clients
// round-trip this field on the next turn and expect it to be present.
var dummySig = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x5a}, 64))

// FuncCallPart builds a functionCall part with a thought signature attached.
func FuncCallPart(name, args string) Part {
	return Part{
		FunctionCall:     &FunctionCall{Name: name, Args: json.RawMessage(OrEmptyJSON(args))},
		ThoughtSignature: dummySig,
	}
}

// OrEmptyJSON returns valid JSON arguments, falling back to {} or a wrapper.
func OrEmptyJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "{}"
	}
	if !json.Valid([]byte(s)) {
		b, _ := json.Marshal(map[string]any{"raw": s})
		return string(b)
	}
	return s
}

// EffortFromModel extracts the reasoning effort Antigravity encodes in the
// model ID, e.g. gemini-3.8-flash-high -> "high", gemini-3.1-pro-low -> "low".
func EffortFromModel(modelID string) string {
	switch {
	case strings.HasSuffix(modelID, "-max"):
		return "max"
	case strings.HasSuffix(modelID, "-xhigh"):
		return "xhigh"
	case strings.HasSuffix(modelID, "-high"):
		return "high"
	case strings.HasSuffix(modelID, "-medium"):
		// Providers reject "medium"; the nearest accepted value is high.
		return "high"
	case strings.HasSuffix(modelID, "-low"):
		return "low"
	}
	return ""
}

// MapFinish maps an OpenAI finish_reason to a Gemini finishReason.
func MapFinish(r string) string {
	switch r {
	case "length":
		return "MAX_TOKENS"
	case "content_filter":
		return "SAFETY"
	default:
		return "STOP"
	}
}

// DecodeNonStream converts a non-streaming OpenAI response into a Gemini one.
func DecodeNonStream(body io.Reader, modelVersion string, dropThoughts bool) (Response, error) {
	var out struct {
		Choices []struct {
			Message struct {
				Content          string           `json:"content"`
				ReasoningContent string           `json:"reasoning_content"`
				Reasoning        string           `json:"reasoning"`
				ToolCalls        []OpenAIToolCall `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens        int `json:"prompt_tokens"`
			CompletionTokens    int `json:"completion_tokens"`
			TotalTokens         int `json:"total_tokens"`
			PromptTokensDetails struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		return Response{}, err
	}

	gr := Response{ModelVersion: modelVersion, Candidates: []Candidate{{Index: 0}}}
	if len(out.Choices) > 0 {
		ch := out.Choices[0]
		var parts []Part
		reasoning := ch.Message.ReasoningContent
		if reasoning == "" {
			reasoning = ch.Message.Reasoning
		}
		if reasoning != "" && !dropThoughts {
			parts = append(parts, Part{Text: reasoning, Thought: true})
		}
		if visible := StripThink(ch.Message.Content); visible != "" {
			parts = append(parts, Part{Text: visible})
		}
		for _, tc := range ch.Message.ToolCalls {
			parts = append(parts, FuncCallPart(tc.Function.Name, tc.Function.Arguments))
		}
		if len(parts) == 0 {
			parts = append(parts, Part{Text: ""})
		}
		gr.Candidates[0].Content = Content{Role: "model", Parts: parts}
		gr.Candidates[0].FinishReason = MapFinish(ch.FinishReason)
	}
	if out.Usage.PromptTokens > 0 {
		gr.UsageMetadata = &Usage{
			PromptTokenCount:        out.Usage.PromptTokens,
			CandidatesTokenCount:    out.Usage.CompletionTokens,
			TotalTokenCount:         out.Usage.TotalTokens,
			CachedContentTokenCount: out.Usage.PromptTokensDetails.CachedTokens,
		}
	}
	return gr, nil
}

// Accum collects a streamed OpenAI response and emits Gemini chunks.
type Accum struct {
	// DropThoughts suppresses reasoning deltas.
	DropThoughts bool
	Text         strings.Builder
	Thought      strings.Builder
	ToolOrder    []int
	Tools        map[int]*OpenAIToolCall
	Usage        *Usage
	Finish       string
}

// NewAccum returns an empty accumulator.
func NewAccum() *Accum {
	return &Accum{Tools: map[int]*OpenAIToolCall{}}
}

// EmitFunc receives converted parts, a finish reason and usage.
type EmitFunc func(parts []Part, finish string, usage *Usage)

// PumpStream reads an OpenAI SSE stream and calls emit for every delta.
func PumpStream(body io.Reader, acc *Accum, emit EmitFunc) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string           `json:"content"`
					ReasoningContent string           `json:"reasoning_content"`
					Reasoning        string           `json:"reasoning"`
					ToolCalls        []OpenAIToolCall `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens        int `json:"prompt_tokens"`
				CompletionTokens    int `json:"completion_tokens"`
				TotalTokens         int `json:"total_tokens"`
				PromptTokensDetails struct {
					CachedTokens int `json:"cached_tokens"`
				} `json:"prompt_tokens_details"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil && chunk.Usage.TotalTokens > 0 {
			acc.Usage = &Usage{
				PromptTokenCount:        chunk.Usage.PromptTokens,
				CandidatesTokenCount:    chunk.Usage.CompletionTokens,
				TotalTokenCount:         chunk.Usage.TotalTokens,
				CachedContentTokenCount: chunk.Usage.PromptTokensDetails.CachedTokens,
			}
		}
		for _, ch := range chunk.Choices {
			if ch.Delta.ReasoningContent != "" || ch.Delta.Reasoning != "" {
				t := ch.Delta.ReasoningContent
				if t == "" {
					t = ch.Delta.Reasoning
				}
				acc.Thought.WriteString(t)
				if !acc.DropThoughts {
					emit([]Part{{Text: t, Thought: true}}, "", nil)
				}
			}
			if ch.Delta.Content != "" {
				acc.Text.WriteString(ch.Delta.Content)
				emit([]Part{{Text: ch.Delta.Content}}, "", nil)
			}
			for _, tc := range ch.Delta.ToolCalls {
				idx := tc.Index
				cur, ok := acc.Tools[idx]
				if !ok {
					cur = &OpenAIToolCall{Index: idx, Type: "function"}
					acc.Tools[idx] = cur
					acc.ToolOrder = append(acc.ToolOrder, idx)
				}
				if tc.ID != "" {
					cur.ID = tc.ID
				}
				if tc.Function.Name != "" {
					cur.Function.Name += tc.Function.Name
				}
				cur.Function.Arguments += tc.Function.Arguments
			}
			if ch.FinishReason != "" {
				acc.Finish = ch.FinishReason
			}
		}
	}
	return scanner.Err()
}

// ToolParts renders the accumulated tool calls as Gemini parts.
func (a *Accum) ToolParts() []Part {
	var parts []Part
	for _, idx := range a.ToolOrder {
		tc := a.Tools[idx]
		parts = append(parts, FuncCallPart(tc.Function.Name, tc.Function.Arguments))
	}
	return parts
}
