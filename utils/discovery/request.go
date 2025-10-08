package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

func doRequest[T any](ctx context.Context, client *http.Client, method, url string, payload any, extraHeaders func(*http.Request)) (*T, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", minecraftUserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Cache-Control", "no-cache")
	if extraHeaders != nil {
		extraHeaders(req)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		bodyResp, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		var resp map[string]any
		err = json.Unmarshal(bodyResp, &resp)
		if err != nil {
			return nil, err
		}
		return nil, &JsonResponseError{
			Status: res.Status,
			Data:   resp,
		}
	}

	var resp T
	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
