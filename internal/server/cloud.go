package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"agy-bridge/internal/translate"
)

// handleCloud serves the Cloud Code ("v1internal:*") surface used by the
// Antigravity IDE and its hub.
//
// Generation is translated to the OpenAI upstream. Everything else (auth
// onboarding, model catalog, quota) is forwarded verbatim to Google, because the
// IDE keeps its own sign-in and native model picker.
func (s *Server) handleCloud(w http.ResponseWriter, r *http.Request, method string, raw []byte) {
	switch method {
	case "generateContent", "streamGenerateContent":
		s.handleCloudGenerate(w, r, method, raw)
	default:
		s.forwardToGoogle(w, r, raw)
	}
}

func (s *Server) handleCloudGenerate(w http.ResponseWriter, r *http.Request, method string, raw []byte) {
	var ce translate.CloudEnvelope
	if err := json.Unmarshal(raw, &ce); err != nil || len(ce.Request) == 0 {
		msg := "bad cloud envelope"
		if err != nil {
			msg += ": " + err.Error()
		} else {
			msg += ": missing request"
		}
		writeError(w, http.StatusBadRequest, msg, "INVALID_ARGUMENT")
		return
	}

	var g translate.Request
	if err := json.Unmarshal(ce.Request, &g); err != nil {
		writeError(w, http.StatusBadRequest, "bad request body: "+err.Error(), "INVALID_ARGUMENT")
		return
	}

	model := s.Cfg.ResolveModel(ce.Model)
	s.seen.record(ce.Model, model, "v1internal")
	stream := method == "streamGenerateContent"

	// The IDE sends a Google OAuth bearer, which is not a usable upstream key,
	// so this route falls back to a server-side key.
	key := s.Cfg.CloudServerKey
	if key == "" {
		key = s.Cfg.UpstreamKey
	}
	if key == "" {
		key = callerKey(r)
	}
	if key == "" {
		writeError(w, http.StatusUnauthorized, "no upstream API key", "UNAUTHENTICATED")
		return
	}

	payload, err := translate.BuildRequest(g, model, stream, s.Cfg.MaxOutputTokens)
	if err != nil {
		writeError(w, http.StatusBadRequest, "translate: "+err.Error(), "INVALID_ARGUMENT")
		return
	}
	if s.Cfg.Debug {
		dumpCloud(ce, raw, payload)
	}

	logf("[cloud] %s model=%s -> upstream (stream=%v)", ce.Model, model, stream)

	start := time.Now()
	resp, err := s.post(key, payload, stream)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream: "+err.Error(), "UNAVAILABLE")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		logf("[cloud] upstream %d: %s", resp.StatusCode, truncate(string(b), 400))
		writeError(w, resp.StatusCode, string(b), "INVALID_ARGUMENT")
		return
	}

	wrap := func(gr translate.Response) any {
		return translate.CloudEnvelopeResponse{Response: gr, TraceID: ce.RequestID}
	}

	if !stream {
		gr, err := translate.DecodeNonStream(resp.Body, ce.Model)
		if err != nil {
			writeError(w, http.StatusBadGateway, "decode upstream: "+err.Error(), "UNAVAILABLE")
			return
		}
		logf("[cloud] %s model=%s nonstream %dms", ce.Model, model, time.Since(start).Milliseconds())
		writeJSON(w, wrap(gr))
		return
	}

	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	acc := translate.NewAccum()
	emit := func(parts []translate.Part, finish string, usage *translate.Usage) {
		frame, ok := sseFrame(parts, finish, usage, ce.Model, wrap)
		if !ok {
			return
		}
		w.Write(frame)
		if flusher != nil {
			flusher.Flush()
		}
	}
	if err := translate.PumpStream(resp.Body, acc, emit); err != nil {
		logf("[cloud] stream error: %v", err)
	}

	if toolParts := acc.ToolParts(); len(toolParts) > 0 {
		if frame, ok := sseFrame(toolParts, "STOP", nil, ce.Model, wrap); ok {
			w.Write(frame)
			if flusher != nil {
				flusher.Flush()
			}
		}
	} else if frame, ok := sseFrame(nil, translate.MapFinish(acc.Finish), acc.Usage, ce.Model, wrap); ok {
		w.Write(frame)
		if flusher != nil {
			flusher.Flush()
		}
	}

	logf("[cloud] %s model=%s stream %dms text=%d tools=%d",
		ce.Model, model, time.Since(start).Milliseconds(), acc.Text.Len(), len(acc.ToolOrder))
}

// forwardToGoogle proxies non-generation Cloud Code RPCs to Google with the
// caller's own bearer token.
func (s *Server) forwardToGoogle(w http.ResponseWriter, r *http.Request, raw []byte) {
	target := s.Cfg.PassthroughBase + r.URL.Path
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}

	req, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(raw))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "INTERNAL")
		return
	}
	for _, h := range []string{"Authorization", "Content-Type", "X-Goog-Api-Client", "User-Agent", "X-Goog-User-Project"} {
		if v := r.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "passthrough: "+err.Error(), "UNAVAILABLE")
		return
	}
	defer resp.Body.Close()

	for k, vs := range resp.Header {
		if equalFold(k, "Content-Length") || equalFold(k, "Transfer-Encoding") {
			continue
		}
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	body, _ := io.ReadAll(resp.Body)
	if s.Cfg.CaptureCatalog && strings.Contains(r.URL.Path, "fetchAvailableModels") {
		os.WriteFile("/tmp/agy-bridge-catalog.json", body, 0o644)
		logf("[catalog] captured %d bytes -> /tmp/agy-bridge-catalog.json", len(body))
	}
	logf("[passthrough] %s -> %d (%d bytes)", r.URL.Path, resp.StatusCode, len(body))
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}
