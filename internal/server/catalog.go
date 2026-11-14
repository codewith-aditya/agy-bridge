package server

import (
	"encoding/json"
	"log"
	"strings"
)

// relabelCatalog rewrites the display names of the IDE's model catalog so each
// entry shows which upstream model actually serves it, e.g.
//
//	"Gemini 3.8 Flash (High)" -> "Gemini 3.8 Flash (High) · DeepSeek-V4.1-Flash-1017"
//
// Only displayName is touched: IDs, enum placeholders and capabilities stay
// exactly as Google sent them, so the picker keeps working. The original body
// is returned unchanged if anything about the payload looks unfamiliar.
func relabelCatalog(body []byte, resolve func(string) string, template string) []byte {
	if template == "" {
		return body
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		return body
	}
	rawModels, ok := doc["models"]
	if !ok {
		return body
	}
	var models map[string]map[string]any
	if err := json.Unmarshal(rawModels, &models); err != nil {
		return body
	}

	changed := 0
	for id, m := range models {
		upstream := resolve(id)
		if upstream == "" || upstream == id {
			continue
		}
		name, _ := m["displayName"].(string)
		if name == "" {
			name = id
		}
		next := strings.NewReplacer("{name}", name, "{model}", upstream).Replace(template)
		next = strings.TrimSpace(next)
		if next == "" || next == name || len(next) > 160 {
			continue
		}
		if m["displayName"] == next {
			continue
		}
		m["displayName"] = next
		changed++
	}
	if changed == 0 {
		return body
	}

	encodedModels, err := json.Marshal(models)
	if err != nil {
		return body
	}
	doc["models"] = encodedModels
	out, err := json.Marshal(doc)
	if err != nil {
		return body
	}
	log.Printf("[catalog] relabelled %d models", changed)
	return out
}
