package worldstate

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestFetchFormImages(t *testing.T) {
	var pngData bytes.Buffer
	png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/icons/a.png":
			w.Write(pngData.Bytes())
		case "/icons/text.png":
			w.Write([]byte("<html>not an image</html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	files := fetchFormImages(context.Background(), map[string]string{
		srv.URL + "/icons/a.png":       "icons/a",
		srv.URL + "/icons/text.png":    "icons/text",
		srv.URL + "/icons/missing.png": "icons/missing",
	}, logrus.NewEntry(logrus.StandardLogger()))

	if len(files) != 1 {
		t.Fatalf("files = %v, want only icons/a.png", keys(files))
	}
	if _, err := png.Decode(bytes.NewReader(files["icons/a.png"])); err != nil {
		t.Errorf("icons/a.png isn't a PNG: %s", err)
	}
}

func keys(m map[string][]byte) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
