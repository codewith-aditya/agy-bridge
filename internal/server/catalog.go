package server

import (
	"encoding/json"
	"log"
	"strings"
)

// ExtraModel is one entry the operator wants visible in the picker even though
// the account tier does not return it.
type ExtraModel struct {
	ID          string
	DisplayName string
}

// ParseExtras reads "id|Display Name,id2|Other Name" into entries.
func ParseExtras(s string) []ExtraModel {
	var out []ExtraModel
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, name, ok := strings.Cut(part, "|")
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if !ok || strings.TrimSpace(name) == "" {
			name = id
		}
		out = append(out, ExtraModel{ID: id, DisplayName: strings.TrimSpace(name)})
	}
	return out
}

// rewriteCatalog optionally relabels every model with the upstream that serves
// it and adds extra entries the account tier does not expose. Only display
// names and the extra clones are touched; IDs, enum placeholders and capability
// flags are preserved. Anything unfamiliar is passed through untouched.
func rewriteCatalog(body []byte, resolve func(string) string, template string, extras []ExtraModel) []byte {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		return body
	}
	rawModels, ok := doc["models"]
	if !ok {
		return body
	}
	var models map[string]map[string]any
	if err := json.Unmarshal(rawModels, &models); err != nil || len(models) == 0 {
		return body
	}

	relabelled := 0
	if template != "" {
		for id, m := range models {
			upstream := resolve(id)
			if upstream == "" || upstream == id {
				continue
			}
			name, _ := m["displayName"].(string)
			if name == "" {
				name = id
			}
			next := strings.TrimSpace(strings.NewReplacer("{name}", name, "{model}", upstream).Replace(template))
			if next == "" || next == name || len(next) > 160 {
				continue
			}
			m["displayName"] = next
			relabelled++
		}
	}

	added := appendExtras(models, extras)
	if relabelled == 0 && len(added) == 0 {
		return body
	}

	encodedModels, err := json.Marshal(models)
	if err != nil {
		return body
	}
	doc["models"] = encodedModels

	if len(added) > 0 {
		doc["agentModelSorts"] = addToPickerGroups(doc["agentModelSorts"], added)
	}

	out, err := json.Marshal(doc)
	if err != nil {
		return body
	}
	log.Printf("[catalog] relabelled %d, added %d", relabelled, len(added))
	return out
}

// appendExtras clones a template entry for every requested model that is
// missing. All catalog entries share one shape, so a clone keeps enum
// placeholders and capability flags consistent.
func appendExtras(models map[string]map[string]any, extras []ExtraModel) []string {
	var template map[string]any
	for _, m := range models {
		if m["supportsThinking"] == true {
			template = m
			break
		}
	}
	if template == nil {
		for _, m := range models {
			template = m
			break
		}
	}
	if template == nil {
		return nil
	}

	var added []string
	for _, e := range extras {
		if _, exists := models[e.ID]; exists {
			continue
		}
		clone := make(map[string]any, len(template))
		for k, v := range template {
			clone[k] = v
		}
		clone["displayName"] = e.DisplayName
		delete(clone, "disabled")
		models[e.ID] = clone
		added = append(added, e.ID)
	}
	return added
}

// addToPickerGroups appends the new IDs to the first picker group so they show
// up in the model selector rather than existing only in the catalog map.
func addToPickerGroups(raw json.RawMessage, ids []string) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var sorts []struct {
		DisplayName string            `json:"displayName"`
		Groups      []json.RawMessage `json:"groups"`
	}
	if err := json.Unmarshal(raw, &sorts); err != nil || len(sorts) == 0 || len(sorts[0].Groups) == 0 {
		return raw
	}
	var group map[string]json.RawMessage
	if err := json.Unmarshal(sorts[0].Groups[0], &group); err != nil {
		return raw
	}
	var modelIDs []string
	if existing, ok := group["modelIds"]; ok {
		_ = json.Unmarshal(existing, &modelIDs)
	}
	seen := map[string]bool{}
	for _, id := range modelIDs {
		seen[id] = true
	}
	for _, id := range ids {
		if !seen[id] {
			modelIDs = append(modelIDs, id)
		}
	}
	encoded, err := json.Marshal(modelIDs)
	if err != nil {
		return raw
	}
	group["modelIds"] = encoded
	encodedGroup, err := json.Marshal(group)
	if err != nil {
		return raw
	}
	sorts[0].Groups[0] = encodedGroup
	out, err := json.Marshal(sorts)
	if err != nil {
		return raw
	}
	return out
}
