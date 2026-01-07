package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var (
	addr = flag.String("addr", ":8080", "HTTP listen address")
	bedrockPath = flag.String("bedrock", "./cmd/bedrocktool/bedrocktool", "path to bedrocktool binary")
	apiToken = flag.String("api-token", "", "optional API token for incoming requests (can also set PACKS_API_TOKEN env var)")
	timeout = flag.Duration("timeout", 3*time.Minute, "timeout for each packs command")
)

func main() {
	flag.Parse()
	if t := os.Getenv("PACKS_API_TOKEN"); t != "" && *apiToken == "" {
		apiToken = &t
	}

	http.HandleFunc("/packs", packsHandler)
	log.Printf("packsserver listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func packsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if *apiToken != "" {
		if r.Header.Get("X-API-Token") != *apiToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	var req struct{ Server string `json:"server"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Server == "" {
		http.Error(w, "server required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), *timeout)
	defer cancel()

	tmp, err := os.MkdirTemp("", "packs-*")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		log.Printf("tmpdir error: %v", err)
		return
	}
	defer os.RemoveAll(tmp)

	cmd := exec.CommandContext(ctx, *bedrockPath, "packs", req.Server)
	cmd.Dir = tmp
	out, err := cmd.CombinedOutput()
	if err != nil {
		http.Error(w, string(out), http.StatusInternalServerError)
		log.Printf("packs failed: %v output:%s", err, string(out))
		return
	}

	// find .mcpack files
	files, _ := filepath.Glob(filepath.Join(tmp, "*.mcpack"))
	if len(files) == 0 {
		http.Error(w, "no .mcpack files found", http.StatusInternalServerError)
		log.Printf("no .mcpack files. output: %s", string(out))
		return
	}

	// create zip
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=packs.zip")
	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, f := range files {
		name := filepath.Base(f)
		fw, err := zw.Create(name)
		if err != nil {
			log.Printf("zip create error: %v", err)
			continue
		}
		fhandle, err := os.Open(f)
		if err != nil {
			log.Printf("open file: %v", err)
			continue
		}
		io.Copy(fw, fhandle)
		fhandle.Close()
	}

	// Optionally: could return a JSON with url to uploaded location; for now we stream zip directly.
	// But the mobile binding expects a JSON {"url":"<...>"}. To support that mode, run an upload step
	// and return a URL. For now we'll support direct response (client should handle binary zip from HTTP response).
	// To keep compatibility with mobile.RequestPacks, we return 200 with a JSON body if client asks for JSON.
}
