package mobile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// RequestPacks contacts a remote packs API which runs `bedrocktool packs <server>`
// and returns a downloadable URL (string) on success. The remote API base URL is
// read from the environment variable PACKS_API_URL (e.g. "https://packs.example.com").
// If PACKS_API_URL is empty, RequestPacks returns an error.
//
// This function is intended to be bound by gomobile. It returns (string, error)
// which maps naturally to Swift/ObjC.
func RequestPacks(server string) (string, error) {
	base := os.Getenv("PACKS_API_URL")
	if base == "" {
		return "", errors.New("PACKS_API_URL not set")
	}
	// build request
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	b, _ := json.Marshal(map[string]string{"server": server})
	req, err := http.NewRequestWithContext(ctx, "POST", base+"/packs", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	// optional: pass an API token from environment
	if tok := os.Getenv("PACKS_API_TOKEN"); tok != "" {
		req.Header.Set("X-API-Token", tok)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var er struct{ Error string }
		_ = json.NewDecoder(resp.Body).Decode(&er)
		if er.Error != "" {
			return "", fmt.Errorf("server error: %s", er.Error)
		}
		return "", fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		var out struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return "", err
		}
		if out.URL == "" {
			return "", errors.New("empty url returned")
		}
		return out.URL, nil
	}

	// If the server returned a zip (or other binary), save to a temp file and return the path
	if strings.HasPrefix(ct, "application/zip") || strings.HasPrefix(ct, "application/octet-stream") || strings.HasPrefix(ct, "binary/") {
		f, err := os.CreateTemp("", "packs-*.zip")
		if err != nil {
			return "", err
		}
		defer f.Close()
		if _, err := io.Copy(f, resp.Body); err != nil {
			return "", err
		}
		return f.Name(), nil
	}

	// Unknown content type: attempt to save as file as a fallback
	f, err := os.CreateTemp("", "packs-*")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", err
	}
	return f.Name(), nil
}
