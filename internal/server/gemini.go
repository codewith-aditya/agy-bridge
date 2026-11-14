package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"agy-bridge/internal/translate"
)

// handleGemini serves the public Gemini API shape:
//
//	POST /v1beta/models/{model}:generateContent
//	POST /v1beta/models/{model}:streamGenerateContent?alt=sse
//	POST /v1beta/models/{model}:countTokens
func (s *Server) handleGemini(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	i := strings.Index(path, "/models/")
	if i < 0 {
		writeError(w, http.StatusNotFound, "unknown path "+path, "NOT_FOUND")
		return
	}
	rest := path[i+len("/models/"):]
	modelPart, action, _ := strings.Cut(rest, ":")
	model := s.Cfg.ResolveModel(modelPart)
	s.seen.record(modelPart, model, "v1beta")

	switch action {
	case "countTokens":
		raw, _ := readAll(r)
		writeJSON(w, map[string]any{"totalTokens": len(raw) / 4})
		return
	case "generateContent", "streamGenerateContent":
	default:
		writeError(w, http.StatusNotFound, "unsupported action "+action, "NOT_FOUND")
		return
	}

	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required", "INVALID_ARGUMENT")
		return
	}

	raw, err := readAll(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body: "+err.Error(), "INVALID_ARGUMENT")
		return
	}
	var g translate.Request
	if err := json.Unmarshal(raw, &g); err != nil {
		writeError(w, http.StatusBadRequest, "parse body: "+err.Error(), "INVALID_ARGUMENT")
		return
	}

	stream := action == "streamGenerateContent"

	key := s.Cfg.UpstreamKey
	if key == "" {
		key = callerKey(r)
	}
	if key == "" {
		writeError(w, http.StatusUnauthorized, "no upstream API key (set UPSTREAM_KEY or pass GEMINI_API_KEY)", "UNAUTHENTICATED")
		return
	}

	payload, err := translate.BuildRequest(g, model, stream, s.Cfg.MaxOutputTokens)
	if err != nil {
		writeError(w, http.StatusBadRequest, "translate: "+err.Error(), "INVALID_ARGUMENT")
		return
	}
	if s.Cfg.Debug {
		dumpRequest(raw, payload)
	}

	start := time.Now()
	resp, err := s.post(key, payload, stream)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream: "+err.Error(), "UNAVAILABLE")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		logf("upstream %d: %s", resp.StatusCode, truncate(string(b), 500))
		status := "INVALID_ARGUMENT"
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			status = "PERMISSION_DENIED"
		}
		writeError(w, resp.StatusCode, string(b), status)
		return
	}

	if !stream {
		gr, err := translate.DecodeNonStream(resp.Body, modelPart)
		if err != nil {
			writeError(w, http.StatusBadGateway, "decode upstream: "+err.Error(), "UNAVAILABLE")
			return
		}
		logf("%s model=%s nonstream %dms", modelPart, model, time.Since(start).Milliseconds())
		writeJSON(w, gr)
		return
	}

	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	acc := translate.NewAccum()
	emit := func(parts []translate.Part, finish string, usage *translate.Usage) {
		frame, ok := sseFrame(parts, finish, usage, modelPart, nil)
		if !ok {
			return
		}
		w.Write(frame)
		if flusher != nil {
			flusher.Flush()
		}
	}
	if err := translate.PumpStream(resp.Body, acc, emit); err != nil {
		logf("stream error: %v", err)
	}

	if toolParts := acc.ToolParts(); len(toolParts) > 0 {
		if frame, ok := sseFrame(toolParts, "STOP", nil, modelPart, nil); ok {
			w.Write(frame)
			if flusher != nil {
				flusher.Flush()
			}
		}
	} else if frame, ok := sseFrame(nil, translate.MapFinish(acc.Finish), acc.Usage, modelPart, nil); ok {
		w.Write(frame)
		if flusher != nil {
			flusher.Flush()
		}
	}

	logf("%s model=%s stream %dms text=%d thoughts=%d tools=%d",
		modelPart, model, time.Since(start).Milliseconds(),
		acc.Text.Len(), acc.Thought.Len(), len(acc.ToolOrder))
}

// sseFrame renders one Gemini SSE frame. wrap, when non-nil, envelopes the
// response for the Cloud Code transport.
func sseFrame(parts []translate.Part, finish string, usage *translate.Usage, modelVersion string, wrap func(translate.Response) any) ([]byte, bool) {
	if len(parts) == 0 && finish == "" && usage == nil {
		return nil, false
	}
	gr := translate.Response{ModelVersion: modelVersion}
	c := translate.Candidate{Index: 0, FinishReason: finish}
	if len(parts) > 0 {
		c.Content = translate.Content{Role: "model", Parts: parts}
	}
	gr.Candidates = []translate.Candidate{c}
	gr.UsageMetadata = usage

	var body any = gr
	if wrap != nil {
		body = wrap(gr)
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, false
	}
	var buf bytes.Buffer
	buf.WriteString("data: ")
	buf.Write(b)
	buf.WriteString("\r\n\r\n")
	return buf.Bytes(), true
}

// post forwards a chat/completions request to the upstream.
func (s *Server) post(key string, payload []byte, stream bool) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, s.Cfg.UpstreamBase+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	return s.Client.Do(req)
}
