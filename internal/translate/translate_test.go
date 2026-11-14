package translate

import (
	"encoding/json"
	"strings"
	"testing"
)

// Antigravity sends tool results under role "model" instead of "user".
// Getting this wrong silently turns every tool result into an empty assistant
// message and the agent loop dies, so it is the single most important test here.
func TestBuildRequestToolResultUnderModelRole(t *testing.T) {
	req := Request{
		Contents: []Content{
			{Role: "user", Parts: []Part{{Text: "list files"}}},
			{Role: "model", Parts: []Part{{FunctionCall: &FunctionCall{Name: "run_command", Args: json.RawMessage(`{"CommandLine":"ls"}`)}}}},
			{Role: "model", Parts: []Part{{FunctionResp: &FunctionResp{Name: "run_command", Response: json.RawMessage(`{"output":"a.txt"}`)}}}},
		},
	}

	out, err := BuildRequest(req, "upstream-model", true, 1024)
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	var body struct {
		Model    string          `json:"model"`
		Messages []OpenAIMessage `json:"messages"`
		Stream   bool            `json:"stream"`
	}
	if err := json.Unmarshal(out, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if body.Model != "upstream-model" || !body.Stream {
		t.Fatalf("unexpected body header: %+v", body)
	}

	var assistantIdx, toolIdx = -1, -1
	for i, m := range body.Messages {
		switch m.Role {
		case "assistant":
			if len(m.ToolCalls) > 0 {
				assistantIdx = i
			}
		case "tool":
			toolIdx = i
		}
	}
	if assistantIdx < 0 {
		t.Fatal("no assistant message carrying tool_calls")
	}
	if toolIdx < 0 {
		t.Fatalf("tool result was not mapped to a role=tool message: %+v", body.Messages)
	}
	if body.Messages[toolIdx].ToolCallID != body.Messages[assistantIdx].ToolCalls[0].ID {
		t.Fatalf("tool_call_id %q does not match emitted id %q",
			body.Messages[toolIdx].ToolCallID, body.Messages[assistantIdx].ToolCalls[0].ID)
	}
}

func TestBuildRequestMapsToolsAndClampsTokens(t *testing.T) {
	req := Request{
		Contents: []Content{{Role: "user", Parts: []Part{{Text: "hi"}}}},
		Tools: []Tool{{FunctionDeclarations: []FunctionDecl{{
			Name:                 "write_file",
			Description:          "Write a file",
			ParametersJSONSchema: json.RawMessage(`{"type":"object","properties":{"Path":{"type":"string"}},"required":["Path"]}`),
		}}}},
		GenerationConfig: &GenerationConfig{MaxOutputTokens: 65535},
	}

	out, _ := BuildRequest(req, "m", false, 32000)
	var body struct {
		MaxTokens int       `json:"max_tokens"`
		Tools     []OAITool `json:"tools"`
	}
	if err := json.Unmarshal(out, &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.MaxTokens != 32000 {
		t.Fatalf("max_tokens = %d, want 32000", body.MaxTokens)
	}
	if len(body.Tools) != 1 || body.Tools[0].Function.Name != "write_file" {
		t.Fatalf("tool declaration lost: %+v", body.Tools)
	}
	if !strings.Contains(string(body.Tools[0].Function.Parameters), `"Path"`) {
		t.Fatalf("parametersJsonSchema was not forwarded: %s", body.Tools[0].Function.Parameters)
	}
}

func TestDecodeNonStream(t *testing.T) {
	src := `{"choices":[{"message":{"role":"assistant","content":"done","reasoning_content":"thinking",
	         "tool_calls":[{"id":"c1","type":"function","function":{"name":"read_file","arguments":"{\"Path\":\"a.txt\"}"}}]},
	         "finish_reason":"tool_calls"}],
	         "usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`

	gr, err := DecodeNonStream(strings.NewReader(src), "gemini-3-pro-preview")
	if err != nil {
		t.Fatalf("DecodeNonStream: %v", err)
	}
	if gr.ModelVersion != "gemini-3-pro-preview" {
		t.Fatalf("modelVersion = %q", gr.ModelVersion)
	}
	c := gr.Candidates[0]
	if c.FinishReason != "STOP" {
		t.Fatalf("finishReason = %q, want STOP", c.FinishReason)
	}
	if len(c.Content.Parts) != 3 {
		t.Fatalf("want thought+text+functionCall, got %+v", c.Content.Parts)
	}
	if !c.Content.Parts[0].Thought {
		t.Fatal("reasoning_content should surface as a thought part")
	}
	if c.Content.Parts[2].FunctionCall == nil || c.Content.Parts[2].FunctionCall.Name != "read_file" {
		t.Fatalf("functionCall missing: %+v", c.Content.Parts[2])
	}
	if c.Content.Parts[2].ThoughtSignature == "" {
		t.Fatal("functionCall parts need a thoughtSignature")
	}
	if gr.UsageMetadata == nil || gr.UsageMetadata.TotalTokenCount != 15 {
		t.Fatalf("usage lost: %+v", gr.UsageMetadata)
	}
}

func TestPumpStreamAccumulatesToolCalls(t *testing.T) {
	src := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"Let me check. "}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"run_command","arguments":"{\"Comm"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"andLine\":\"ls\"}"}}]}}]}`,
		`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
		`data: [DONE]`,
	}, "\n")

	acc := NewAccum()
	var emitted strings.Builder
	emit := func(parts []Part, finish string, usage *Usage) {
		for _, p := range parts {
			emitted.WriteString(p.Text)
		}
	}
	if err := PumpStream(strings.NewReader(src), acc, emit); err != nil {
		t.Fatalf("PumpStream: %v", err)
	}
	if acc.Text.String() != "Let me check. " {
		t.Fatalf("text = %q", acc.Text.String())
	}
	parts := acc.ToolParts()
	if len(parts) != 1 || parts[0].FunctionCall == nil {
		t.Fatalf("tool parts = %+v", parts)
	}
	if got := string(parts[0].FunctionCall.Args); got != `{"CommandLine":"ls"}` {
		t.Fatalf("reassembled args = %s", got)
	}
	if acc.Finish != "tool_calls" {
		t.Fatalf("finish = %q", acc.Finish)
	}
}

func TestMapFinish(t *testing.T) {
	cases := map[string]string{"length": "MAX_TOKENS", "content_filter": "SAFETY", "stop": "STOP", "": "STOP"}
	for in, want := range cases {
		if got := MapFinish(in); got != want {
			t.Errorf("MapFinish(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOrEmptyJSON(t *testing.T) {
	cases := map[string]string{"": "{}", "   ": "{}", `{"a":1}`: `{"a":1}`}
	for in, want := range cases {
		if got := OrEmptyJSON(in); got != want {
			t.Errorf("OrEmptyJSON(%q) = %q, want %q", in, got, want)
		}
	}
	if got := OrEmptyJSON("not json"); !strings.Contains(got, "raw") {
		t.Errorf("invalid JSON should be wrapped, got %q", got)
	}
}
