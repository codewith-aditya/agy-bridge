package server

import (
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
