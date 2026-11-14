package server

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRelabelCatalog(t *testing.T) {
	src := `{"models":{
	  "gemini-3.8-flash-high":{"displayName":"Gemini 3.8 Flash (High)","supportsImages":true,"model":"MODEL_PLACEHOLDER_M1"},
	  "chat_20706":{"maxTokens":16384}
	},"defaultAgentModelId":"gemini-3.8-flash-high"}`

	resolve := func(id string) string {
		if id == "gemini-3.8-flash-high" {
			return "deepseek-v4-flash"
		}
		return id
	}

	out := relabelCatalog([]byte(src), resolve, "{name} · {model}")

	var doc struct {
		Models map[string]struct {
			DisplayName  string `json:"displayName"`
			SupportsImgs bool   `json:"supportsImages"`
			Model        string `json:"model"`
		} `json:"models"`
		Default string `json:"defaultAgentModelId"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got := doc.Models["gemini-3.8-flash-high"].DisplayName; got != "Gemini 3.8 Flash (High) · deepseek-v4-flash" {
		t.Fatalf("displayName = %q", got)
	}
	if !doc.Models["gemini-3.8-flash-high"].SupportsImgs {
		t.Fatal("capabilities were dropped")
	}
	if doc.Models["gemini-3.8-flash-high"].Model != "MODEL_PLACEHOLDER_M1" {
		t.Fatal("enum placeholder was changed")
	}
	if doc.Default != "gemini-3.8-flash-high" {
		t.Fatal("unsrelated fields were altered")
	}
}

func TestRelabelCatalogLeavesUnknownPayloadsAlone(t *testing.T) {
	src := []byte(`{"something":"else"}`)
	out := relabelCatalog(src, func(string) string { return "x" }, "{name} · {model}")
	if string(out) != string(src) {
		t.Fatalf("payload should be untouched, got %s", out)
	}
}

func TestRelabelCatalogInvalidJSON(t *testing.T) {
	src := []byte(`not json at all`)
	out := relabelCatalog(src, func(string) string { return "x" }, "{name} · {model}")
	if !strings.Contains(string(out), "not json") {
		t.Fatalf("invalid body must pass through, got %s", out)
	}
}
