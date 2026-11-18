package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"agy-bridge/internal/translate"
)

func readAll(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}

func equalFold(a, b string) bool { return strings.EqualFold(a, b) }

// splitInlineThink separates <think> blocks that some chat templates inline in
// the content stream, so they render as the client's thinking panel instead of
// as answer text.
func splitInlineThink(tf *translate.ThinkFilter, parts []translate.Part) []translate.Part {
	out := make([]translate.Part, 0, len(parts))
	for _, p := range parts {
		if p.Text != "" && !p.Thought {
			visible, thought := tf.Write(p.Text)
			if thought != "" {
				out = append(out, translate.Part{Text: thought, Thought: true})
			}
			if visible != "" {
				p.Text = visible
				out = append(out, p)
			}
			continue
		}
		out = append(out, p)
	}
	return out
}

// retryWithoutEffort re-posts the same request with reasoning_effort removed.
// Some providers reject effort values they do not recognise with a hard 400, and
// losing the whole turn over a hint is worse than ignoring the hint.
func (s *Server) retryWithoutEffort(ctx context.Context, key string, payload []byte, stream bool) (*http.Response, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, err
	}
	delete(body, "reasoning_effort")
	stripped, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return s.post(ctx, key, stripped, stream)
}

// cachedCount reports provider-side prompt cache hits, if the upstream sent any.
func cachedCount(acc *translate.Accum) int {
	if acc.Usage == nil {
		return 0
	}
	return acc.Usage.CachedContentTokenCount
}

// dumpRequest writes the last Gemini request pair to /tmp for debugging.
func dumpRequest(raw, payload []byte) {
	os.WriteFile("/tmp/agy-bridge-last-gemini.json", raw, 0o644)
	os.WriteFile("/tmp/agy-bridge-last-req.json", payload, 0o644)
}

// dumpCloud writes the last Cloud Code request pair to /tmp for debugging.
func dumpCloud(ce translate.CloudEnvelope, raw, payload []byte) {
	os.WriteFile("/tmp/agy-bridge-last-cloud.json", raw, 0o644)
	os.WriteFile("/tmp/agy-bridge-last-req.json", payload, 0o644)
	if len(ce.Request) > 0 {
		os.WriteFile("/tmp/agy-bridge-last-cloud-inner.json", ce.Request, 0o644)
	}
}
