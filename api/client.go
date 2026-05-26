package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const endpoint = "https://api.linear.app/graphql"

var client = &http.Client{Timeout: 15 * time.Second}

// authHeader returns (headerValue, isOAuth, error).
// Precedence: LINEAR_API_KEY env > stored OAuth token > error.
func authHeader() (string, bool, error) {
	if key := os.Getenv("LINEAR_API_KEY"); key != "" {
		return key, false, nil
	}
	if tok, err := readStoredOAuthToken(); err == nil && tok != "" {
		return "Bearer " + tok, true, nil
	}
	return "", false, fmt.Errorf("no auth: set LINEAR_API_KEY or run 'linear oauth login'")
}

func readStoredOAuthToken() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "linear-cli", "auth.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var c struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return "", err
	}
	return c.AccessToken, nil
}

type graphResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphError    `json:"errors"`
}

type graphError struct {
	Message string `json:"message"`
	Path    []any  `json:"path"`
}

func Query(query string, result any) error {
	auth, _, err := authHeader()
	if err != nil {
		return err
	}

	body := map[string]string{"query": query}
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var gr graphResponse
	if err := json.Unmarshal(data, &gr); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	if len(gr.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", gr.Errors[0].Message)
	}

	if gr.Data == nil {
		return fmt.Errorf("no data in response")
	}

	return json.Unmarshal(gr.Data, result)
}
