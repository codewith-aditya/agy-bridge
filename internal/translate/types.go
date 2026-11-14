// Package translate converts between the Gemini generateContent schema
// (spoken by Google Antigravity) and the OpenAI chat/completions schema
// (spoken by OpenAI, OpenRouter, vLLM, Ollama and friends).
package translate

import "encoding/json"

// Part is a single piece of a Gemini content block.
type Part struct {
	Text             string        `json:"text,omitempty"`
	Thought          bool          `json:"thought,omitempty"`
	ThoughtSignature string        `json:"thoughtSignature,omitempty"`
	FunctionCall     *FunctionCall `json:"functionCall,omitempty"`
	FunctionResp     *FunctionResp `json:"functionResponse,omitempty"`
	InlineData       *Blob         `json:"inlineData,omitempty"`
}

// FunctionCall is a tool invocation emitted by the model.
type FunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

// FunctionResp is a tool result fed back by the client.
type FunctionResp struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response,omitempty"`
}

// Blob is inline binary data, usually an image.
type Blob struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// Content is one turn.
type Content struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

// Tool is a Gemini tool block.
type Tool struct {
	FunctionDeclarations []FunctionDecl `json:"functionDeclarations"`
}

// FunctionDecl declares one callable function.
type FunctionDecl struct {
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	Parameters           json.RawMessage `json:"parameters,omitempty"`
	ParametersJSONSchema json.RawMessage `json:"parametersJsonSchema,omitempty"`
}

// ThinkingConfig mirrors Gemini's thinking knobs.
type ThinkingConfig struct {
	IncludeThoughts bool `json:"includeThoughts,omitempty"`
	ThinkingBudget  int  `json:"thinkingBudget,omitempty"`
}

// GenerationConfig mirrors the fields the bridge cares about.
type GenerationConfig struct {
	MaxOutputTokens  int             `json:"maxOutputTokens,omitempty"`
	Temperature      *float64        `json:"temperature,omitempty"`
	TopP             *float64        `json:"topP,omitempty"`
	TopK             *int            `json:"topK,omitempty"`
	StopSequences    []string        `json:"stopSequences,omitempty"`
	ResponseMimeType string          `json:"responseMimeType,omitempty"`
	ThinkingConfig   *ThinkingConfig `json:"thinkingConfig,omitempty"`
}

// Request is a Gemini generateContent request body.
type Request struct {
	// Effort is derived from the model ID suffix (-low/-medium/-high/-max) and
	// forwarded as reasoning_effort for providers that understand it.
	Effort            string            `json:"-"`
	Contents          []Content         `json:"contents"`
	SystemInstruction *Content          `json:"systemInstruction,omitempty"`
	Tools             []Tool            `json:"tools,omitempty"`
	GenerationConfig  *GenerationConfig `json:"generationConfig,omitempty"`
}

// Usage is Gemini's usageMetadata.
type Usage struct {
	PromptTokenCount        int `json:"promptTokenCount,omitempty"`
	CandidatesTokenCount    int `json:"candidatesTokenCount,omitempty"`
	TotalTokenCount         int `json:"totalTokenCount,omitempty"`
	CachedContentTokenCount int `json:"cachedContentTokenCount,omitempty"`
}

// Candidate is one generation result.
type Candidate struct {
	Content      Content `json:"content"`
	FinishReason string  `json:"finishReason,omitempty"`
	Index        int     `json:"index"`
}

// Response is a Gemini generateContent response body.
type Response struct {
	Candidates    []Candidate `json:"candidates"`
	UsageMetadata *Usage      `json:"usageMetadata,omitempty"`
	ModelVersion  string      `json:"modelVersion,omitempty"`
}

// CloudEnvelope is the Cloud Code (v1internal) request wrapper used by the
// Antigravity IDE and its hub.
type CloudEnvelope struct {
	Model     string          `json:"model"`
	Project   string          `json:"project,omitempty"`
	RequestID string          `json:"requestId,omitempty"`
	Request   json.RawMessage `json:"request,omitempty"`
}

// CloudEnvelopeResponse is the Cloud Code (v1internal) response wrapper.
type CloudEnvelopeResponse struct {
	Response any    `json:"response"`
	TraceID  string `json:"traceId,omitempty"`
}
