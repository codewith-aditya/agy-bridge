// Package config loads agy-bridge settings from the environment.
package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds every tunable the bridge understands.
type Config struct {
	// Listen is the address the bridge binds to.
	Listen string
	// UpstreamBase is the OpenAI-compatible base URL, without a trailing slash.
	UpstreamBase string
	// UpstreamKey is used for the public Gemini route when set. When empty the
	// caller's own key (GEMINI_API_KEY / x-goog-api-key) is used instead.
	UpstreamKey string
	// CloudServerKey is used for the Cloud Code route, whose caller presents a
	// Google OAuth bearer that is not a usable upstream credential.
	CloudServerKey string
	// DefaultModel is the upstream model every unrecognised Gemini model maps to.
	DefaultModel string
	// MaxOutputTokens clamps maxOutputTokens. Antigravity asks for 65535, which
	// some upstreams reject. 0 disables the clamp.
	MaxOutputTokens int
	// ModelMap maps Gemini model IDs to upstream model IDs.
	ModelMap map[string]string
	// Debug dumps the last request bodies to /tmp.
	Debug bool
	// PassthroughBase is the Google Cloud Code host used for non-generation RPCs.
	PassthroughBase string
	// CaptureCatalog dumps the raw fetchAvailableModels response, so the model
	// catalog schema can be extended safely instead of guessed.
	CaptureCatalog bool
	// CatalogExtra lists model IDs to force into the picker even when the
	// account tier does not return them, as "id|Display Name,id2|Name2".
	CatalogExtra string
	// CatalogLabelTemplate renders the picker label. "{name}" is the original
	// display name, "{model}" the upstream model this deployment resolves to.
	// Empty disables relabelling.
	CatalogLabelTemplate string
	// CatalogLabels rewrites model display names in the IDE catalog to show the
	// upstream model that actually serves them.
	CatalogLabels bool
	// DropThoughts stops forwarding reasoning deltas. Reasoning models often
	// narrate raw JSON tool syntax, which renders as noise in the client's
	// thinking panel.
	DropThoughts bool
}

// Load reads the environment into a Config.
func Load() Config {
	return Config{
		Listen:          envStr("LISTEN", "127.0.0.1:8198"),
		UpstreamBase:    strings.TrimRight(envStr("UPSTREAM_BASE", "https://modelrouter.in/v1"), "/"),
		UpstreamKey:     envStr("UPSTREAM_KEY", ""),
		CloudServerKey:  envStr("CLOUD_SERVER_KEY", ""),
		DefaultModel:    envStr("DEFAULT_MODEL", ""),
		MaxOutputTokens: envInt("MAX_OUTPUT_TOKENS", 32000),
		ModelMap:        ParseModelMap(envStr("MODEL_MAP", "")),
		Debug:           envStr("DEBUG", "") != "",
		PassthroughBase: strings.TrimRight(envStr("CLOUD_CODE_PASSTHROUGH_BASE", "https://cloudcode-pa.googleapis.com"), "/"),
		CaptureCatalog:  envStr("CAPTURE_CATALOG", "") != "",
		CatalogLabels:   envStr("CATALOG_LABELS", "") != "",
		CatalogExtra:    envStr("CATALOG_EXTRA", ""),
		CatalogLabelTemplate: func() string {
			if envStr("CATALOG_LABELS", "") == "" {
				return ""
			}
			return envStr("CATALOG_LABEL_TEMPLATE", "{name} · {model}")
		}(),
		DropThoughts: envStr("DROP_THOUGHTS", "") != "",
	}
}

// ResolveModel maps a Gemini model ID to its configured upstream model.
func (c Config) ResolveModel(geminiModel string) string {
	geminiModel = strings.TrimPrefix(geminiModel, "models/")

	if v, ok := c.ModelMap[geminiModel]; ok {
		return v
	}

	// Antigravity appends effort and route suffixes that change between
	// releases: gemini-3.8-flash-high, gemini-3.5-flash-lite, gemini-3.1-pro-preview.
	// Strip them and retry so a family mapping keeps working.
	for _, suffix := range []string{"-high", "-medium", "-low", "-lite", "-preview", "-thinking", "-1p", "-thoughts"} {
		if trimmed := strings.TrimSuffix(geminiModel, suffix); trimmed != geminiModel {
			if v, ok := c.ModelMap[trimmed]; ok {
				return v
			}
			geminiModel = trimmed
		}
	}

	// Longest configured key that prefixes the requested ID wins, so
	// "gemini-3.8-flash" still covers "gemini-3.8-flash-experimental".
	bestKey, bestVal := "", ""
	for k, v := range c.ModelMap {
		if strings.HasPrefix(geminiModel, k) && len(k) > len(bestKey) {
			bestKey, bestVal = k, v
		}
	}
	if bestKey != "" {
		return bestVal
	}

	if c.DefaultModel != "" {
		return c.DefaultModel
	}
	return geminiModel
}

// ParseModelMap parses "a=x,b=y" into a map.
func ParseModelMap(s string) map[string]string {
	m := map[string]string{}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		if k, v, ok := strings.Cut(pair, "="); ok {
			m[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return m
}

func envStr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
