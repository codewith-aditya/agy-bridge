package translate

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OpenAIMessage is one message in an OpenAI chat/completions request.
type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content,omitempty"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

// OpenAIToolCall is a tool invocation in the OpenAI schema.
type OpenAIToolCall struct {
	Index    int     `json:"index,omitempty"`
	ID       string  `json:"id,omitempty"`
	Type     string  `json:"type,omitempty"`
	Function OAIFunc `json:"function"`
}

// OAIFunc is the name/arguments pair of a tool call.
type OAIFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// OAITool is a tool declaration in the OpenAI schema.
type OAITool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description,omitempty"`
		Parameters  json.RawMessage `json:"parameters,omitempty"`
	} `json:"function"`
}

// BuildRequest translates a Gemini request into an OpenAI chat/completions body.
// maxOutput clamps generationConfig.maxOutputTokens when non-zero.
func BuildRequest(g Request, model string, stream bool, maxOutput int) ([]byte, error) {
	var msgs []OpenAIMessage

	if g.SystemInstruction != nil {
		var sb strings.Builder
		for _, p := range g.SystemInstruction.Parts {
			sb.WriteString(p.Text)
		}
		if sb.Len() > 0 {
			msgs = append(msgs, OpenAIMessage{Role: "system", Content: sb.String()})
		}
	}

	// Gemini functionResponse carries only a name; OpenAI wants a tool_call_id.
	// We synthesise ids per model turn and hand them back FIFO per function name.
	pending := map[string][]string{}
	seq := 0

	for _, c := range g.Contents {
		isModel := c.Role == "model" || c.Role == "assistant"

		var calls []OpenAIToolCall
		var toolMsgs []OpenAIMessage
		var text strings.Builder
		var images []map[string]any

		for _, p := range c.Parts {
			switch {
			case p.FunctionCall != nil:
				seq++
				id := fmt.Sprintf("call_%d_%s", seq, p.FunctionCall.Name)
				args := "{}"
				if len(p.FunctionCall.Args) > 0 {
					args = string(p.FunctionCall.Args)
				}
				calls = append(calls, OpenAIToolCall{
					ID: id, Type: "function",
					Function: OAIFunc{Name: p.FunctionCall.Name, Arguments: args},
				})
				pending[p.FunctionCall.Name] = append(pending[p.FunctionCall.Name], id)
			case p.FunctionResp != nil:
				// Antigravity sends functionResponse under role "model", not "user".
				id := "call_" + p.FunctionResp.Name
				if q := pending[p.FunctionResp.Name]; len(q) > 0 {
					id = q[0]
					pending[p.FunctionResp.Name] = q[1:]
				}
				resp := "{}"
				if len(p.FunctionResp.Response) > 0 {
					resp = string(p.FunctionResp.Response)
				}
				toolMsgs = append(toolMsgs, OpenAIMessage{Role: "tool", ToolCallID: id, Content: resp})
			case p.InlineData != nil:
				url := "data:" + p.InlineData.MimeType + ";base64," + p.InlineData.Data
				images = append(images, map[string]any{
					"type":      "image_url",
					"image_url": map[string]any{"url": url},
				})
			case p.Text != "" && !p.Thought:
				text.WriteString(p.Text)
			}
		}

		if len(toolMsgs) > 0 {
			msgs = append(msgs, toolMsgs...)
		}

		if isModel {
			if len(calls) > 0 || text.Len() > 0 {
				m := OpenAIMessage{Role: "assistant", ToolCalls: calls}
				if text.Len() > 0 || len(calls) == 0 {
					m.Content = text.String()
				}
				msgs = append(msgs, m)
			}
		} else if text.Len() > 0 || len(images) > 0 {
			if len(images) > 0 {
				var parts []map[string]any
				if text.Len() > 0 {
					parts = append(parts, map[string]any{"type": "text", "text": text.String()})
				}
				parts = append(parts, images...)
				msgs = append(msgs, OpenAIMessage{Role: "user", Content: parts})
			} else {
				msgs = append(msgs, OpenAIMessage{Role: "user", Content: text.String()})
			}
		}
	}

	body := map[string]any{
		"model":    model,
		"messages": msgs,
		"stream":   stream,
	}
	if stream {
		body["stream_options"] = map[string]any{"include_usage": true}
	}

	var tools []OAITool
	for _, t := range g.Tools {
		for _, fd := range t.FunctionDeclarations {
			var tl OAITool
			tl.Type = "function"
			tl.Function.Name = fd.Name
			tl.Function.Description = fd.Description
			switch {
			case len(fd.ParametersJSONSchema) > 0:
				tl.Function.Parameters = fd.ParametersJSONSchema
			case len(fd.Parameters) > 0:
				tl.Function.Parameters = fd.Parameters
			default:
				tl.Function.Parameters = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			tools = append(tools, tl)
		}
	}
	if len(tools) > 0 {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}

	if gc := g.GenerationConfig; gc != nil {
		n := gc.MaxOutputTokens
		if maxOutput > 0 && (n == 0 || n > maxOutput) {
			n = maxOutput
		}
		body["max_tokens"] = n
		if gc.Temperature != nil {
			body["temperature"] = *gc.Temperature
		}
		if gc.TopP != nil {
			body["top_p"] = *gc.TopP
		}
		if len(gc.StopSequences) > 0 {
			body["stop"] = gc.StopSequences
		}
		if gc.ResponseMimeType == "application/json" {
			body["response_format"] = map[string]any{"type": "json_object"}
		}
	}

	return json.Marshal(body)
}
