package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client calls the Sparky REST API directly.
// Base URL should be https://<host>/api
type Client struct {
	BaseURL string
	APIKey  string
	http    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) do(method, path string, query map[string]string, body any) (json.RawMessage, error) {
	u := c.BaseURL + path
	if len(query) > 0 {
		q := url.Values{}
		for k, v := range query {
			q.Set(k, v)
		}
		u += "?" + q.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, u, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, raw)
	}

	if len(raw) == 0 {
		return json.RawMessage("null"), nil
	}
	return raw, nil
}

func (c *Client) Get(path string, query map[string]string) (json.RawMessage, error) {
	return c.do("GET", path, query, nil)
}

func (c *Client) Post(path string, body any) (json.RawMessage, error) {
	return c.do("POST", path, nil, body)
}

func (c *Client) Put(path string, body any) (json.RawMessage, error) {
	return c.do("PUT", path, nil, body)
}

func (c *Client) Delete(path string) (json.RawMessage, error) {
	return c.do("DELETE", path, nil, nil)
}
