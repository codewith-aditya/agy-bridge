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

	out := rewriteCatalog([]byte(src), resolve, "{name} · {model}", nil)

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
	out := rewriteCatalog(src, func(string) string { return "x" }, "{name} · {model}", nil)
	if string(out) != string(src) {
		t.Fatalf("payload should be untouched, got %s", out)
	}
}

func TestRelabelCatalogInvalidJSON(t *testing.T) {
	src := []byte(`not json at all`)
	out := rewriteCatalog(src, func(string) string { return "x" }, "{name} · {model}", nil)
	if !strings.Contains(string(out), "not json") {
		t.Fatalf("invalid body must pass through, got %s", out)
	}
}

func TestRewriteCatalogAddsExtras(t *testing.T) {
	src := `{"models":{
	  "gemini-3.8-flash-high":{"displayName":"Gemini 3.8 Flash (High)","supportsThinking":true,"model":"MODEL_PLACEHOLDER_M1"},
	  "gemini-2.5-pro":{"displayName":"Gemini 2.5 Pro","supportsThinking":true,"model":"MODEL_PLACEHOLDER_M2"}
	},"agentModelSorts":[{"displayName":"Recommended","groups":[{"modelIds":["gemini-3.8-flash-high"]}]}]}`

	extras := ParseExtras("gemini-3.1-pro-preview|Gemini 3.1 Pro, gemini-3.1-pro-preview|Duplicate")
	if len(extras) != 2 {
		t.Fatalf("ParseExtras returned %d entries", len(extras))
	}

	out := rewriteCatalog([]byte(src), func(id string) string { return "deepseek-v4-flash" }, "", extras)

	var doc struct {
		Models map[string]map[string]any `json:"models"`
		Sorts  []struct {
			Groups []struct {
				ModelIDs []string `json:"modelIds"`
			} `json:"groups"`
		} `json:"agentModelSorts"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	added, ok := doc.Models["gemini-3.1-pro-preview"]
	if !ok {
		t.Fatal("extra model missing from catalog")
	}
	if added["displayName"] != "Gemini 3.1 Pro" {
		t.Fatalf("extra displayName = %v", added["displayName"])
	}
	if added["model"] == nil {
		t.Fatal("clone lost its enum placeholder")
	}
	ids := doc.Sorts[0].Groups[0].ModelIDs
	found := false
	for _, id := range ids {
		if id == "gemini-3.1-pro-preview" {
			found = true
		}
	}
	if !found {
		t.Fatalf("extra model not added to picker group: %v", ids)
	}
	if len(ids) != len(unique(ids)) {
		t.Fatalf("duplicate model IDs in picker group: %v", ids)
	}
}

func unique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
