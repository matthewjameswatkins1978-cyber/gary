// Package gemini implements a client for the Gemini Interactions API.
package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const baseURL = "https://generativelanguage.googleapis.com/v1beta"

// Client interacts with the Gemini Interactions API.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Gemini API client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

// InteractionRequest is the request body for the Interactions API.
type InteractionRequest struct {
	Agent       string       `json:"agent,omitempty"`
	Model       string       `json:"model,omitempty"`
	Input       string       `json:"input"`
	Environment *Environment `json:"environment,omitempty"`
	Tools       []Tool       `json:"tools,omitempty"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
}

// Environment configures the execution environment.
type Environment struct {
	Type string `json:"type,omitempty"`
}

// Tool configures available tools for the agent.
type Tool struct {
	Type string `json:"type"`
}

// InteractionResponse is the response from the Interactions API.
type InteractionResponse struct {
	ID            string    `json:"id"`
	Status        string    `json:"status"`
	Usage         *Usage    `json:"usage,omitempty"`
	EnvironmentID string    `json:"environment_id,omitempty"`
	ServiceTier   string    `json:"service_tier,omitempty"`
	Steps         []Step    `json:"steps,omitempty"`
	Created       string    `json:"created,omitempty"`
	Updated       string    `json:"updated,omitempty"`
	Model         string    `json:"model,omitempty"`
	Agent         string    `json:"agent,omitempty"`
	Error         *APIError `json:"error,omitempty"`
}

// Usage reports token usage for the interaction.
type Usage struct {
	TotalTokens        int `json:"total_tokens"`
	TotalInputTokens   int `json:"total_input_tokens"`
	TotalOutputTokens  int `json:"total_output_tokens"`
	TotalThoughtTokens int `json:"total_thought_tokens"`
	TotalCachedTokens  int `json:"total_cached_tokens"`
	TotalToolUseTokens int `json:"total_tool_use_tokens"`
}

// Step is a single step in the interaction.
type Step struct {
	ID           string                 `json:"id,omitempty"`
	Type         string                 `json:"type"`
	Content      []ContentPart          `json:"content,omitempty"`
	Arguments    map[string]interface{} `json:"arguments,omitempty"`
	Result       json.RawMessage        `json:"result,omitempty"`
	ResultString string                 `json:"-"`
	IsError      bool                   `json:"is_error,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Summary      []ContentPart          `json:"summary,omitempty"`
}

// UnmarshalJSON implements custom unmarshaling for Step.
func (s *Step) UnmarshalJSON(data []byte) error {
	type Alias Step
	aux := &struct {
		Result json.RawMessage `json:"result,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(s),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	s.Result = aux.Result
	// Try to extract string from result
	if len(s.Result) > 0 {
		var str string
		if err := json.Unmarshal(s.Result, &str); err == nil {
			s.ResultString = str
		}
	}
	return nil
}

// ContentPart is a part of a step's content.
type ContentPart struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

// APIError represents an error from the API.
type APIError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

// RunInteraction sends an interaction request and returns the response.
func (c *Client) RunInteraction(req InteractionRequest) (*InteractionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/interactions?key=%s", baseURL, c.apiKey)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result InteractionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("API error: %s", result.Error.Message)
	}

	return &result, nil
}

// EnvironmentInfo holds metadata about a remote environment.
type EnvironmentInfo struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	FileCount    string `json:"file_count"`
	SizeBytes    string `json:"size_bytes"`
	Created      string `json:"created,omitempty"`
	LastAccessed string `json:"last_accessed,omitempty"`
}

// GetEnvironment fetches environment metadata.
func (c *Client) GetEnvironment(envID string) (*EnvironmentInfo, error) {
	url := fmt.Sprintf("%s/environments/%s?key=%s", baseURL, envID, c.apiKey)
	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("environment query failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var info EnvironmentInfo
	if err := json.Unmarshal(respBody, &info); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &info, nil
}

// DownloadEnvironment streams the environment snapshot TAR archive.
// The caller must close the returned ReadCloser when done.
func (c *Client) DownloadEnvironment(envID string) (io.ReadCloser, int64, error) {
	url := fmt.Sprintf("%s/files/environment-%s:download?alt=media&key=%s", baseURL, envID, c.apiKey)
	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, 0, fmt.Errorf("download failed (%d): %s", resp.StatusCode, string(respBody))
	}

	return resp.Body, resp.ContentLength, nil
}
