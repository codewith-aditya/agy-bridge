// Command agy-bridge lets Google Antigravity run on any third-party LLM.
//
// The Antigravity CLI speaks the Gemini generateContent API; the IDE and its hub
// speak the Cloud Code v1internal API. Both are translated here into a single
// OpenAI-compatible /chat/completions upstream.
//
//	GOOGLE_GEMINI_BASE_URL=http://127.0.0.1:8198 GEMINI_API_KEY=<key> agy
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"agy-bridge/internal/config"
	"agy-bridge/internal/server"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	cfg := config.Load()
	srv := server.New(cfg)

	log.Printf("agy-bridge %s listening on %s -> %s (default model %q)",
		version, cfg.Listen, cfg.UpstreamBase, cfg.DefaultModel)

	httpSrv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 30 * time.Second,
		// Long turns stream for minutes, so the read/write timeouts stay off;
		// an idle keep-alive is the one wait worth bounding.
		IdleTimeout: 120 * time.Second,
	}
	if err := httpSrv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
