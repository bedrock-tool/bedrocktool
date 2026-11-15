package worldstate

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// formImageMaxSize is the largest form image downloaded.
	formImageMaxSize = 4 << 20
	// formImageWorkers is how many form images download at once.
	formImageWorkers = 8
)

var (
	formImageClient = &http.Client{Timeout: 15 * time.Second}

	// formImageCache keeps downloaded form images (empty when the download
	// failed) for the whole run: every world of a session shows the same
	// menus.
	formImageCache   = map[string]formImage{}
	formImageCacheMu sync.Mutex
)

// fetchFormImages downloads the images form buttons show from URLs (url ->
// texture path, see uirecord.ImagePath) and returns them as resource pack
// files (path.png or path.jpg), the same images the game downloads to show
// the forms. An image that can't be downloaded is left out: the script
// still asks for its path, which another pack (such as Hive Recode) may
// have.
func fetchFormImages(ctx context.Context, images map[string]string, log *logrus.Entry) map[string][]byte {
	urls := make([]string, 0, len(images))
	for u := range images {
		urls = append(urls, u)
	}
	sort.Strings(urls)

	jobs := make(chan string)
	var wg sync.WaitGroup
	for range min(formImageWorkers, len(urls)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range jobs {
				formImageCacheMu.Lock()
				_, done := formImageCache[u]
				formImageCacheMu.Unlock()
				if done {
					continue
				}
				img, err := fetchFormImage(ctx, u)
				if err != nil {
					log.Debugf("form image %s: %s", u, err)
				}
				formImageCacheMu.Lock()
				formImageCache[u] = img
				formImageCacheMu.Unlock()
			}
		}()
	}
	for _, u := range urls {
		jobs <- u
	}
	close(jobs)
	wg.Wait()

	files := make(map[string][]byte)
	formImageCacheMu.Lock()
	defer formImageCacheMu.Unlock()
	for u, p := range images {
		if img := formImageCache[u]; img.data != nil {
			files[p+img.ext] = img.data
		}
	}
	return files
}

// formImage is a downloaded image as the server's CDN gave it, and the
// texture file extension for its format.
type formImage struct {
	data []byte
	ext  string
}

// fetchFormImage downloads one image. Its bytes are kept as they are (a
// photo re-encoded as PNG grows many times over); it only has to decode as
// a PNG or JPEG, which the game reads as textures.
func fetchFormImage(ctx context.Context, u string) (formImage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return formImage{}, err
	}
	req.Header.Set("User-Agent", "bedrocktool")
	res, err := formImageClient.Do(req)
	if err != nil {
		return formImage{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return formImage{}, fmt.Errorf("status %s", res.Status)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, formImageMaxSize+1))
	if err != nil {
		return formImage{}, err
	}
	if len(data) > formImageMaxSize {
		return formImage{}, fmt.Errorf("larger than %d bytes", formImageMaxSize)
	}
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return formImage{}, err
	}
	switch format {
	case "png":
		return formImage{data: data, ext: ".png"}, nil
	case "jpeg":
		return formImage{data: data, ext: ".jpg"}, nil
	}
	return formImage{}, fmt.Errorf("unsupported image format %s", format)
}
