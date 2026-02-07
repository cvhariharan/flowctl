package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

// Option represents a single option item from the remote API
type Option struct {
	Name     string `json:"name"`
	Selected bool   `json:"selected,omitempty"`
}

// InterpolateVariables replaces variable placeholders in a URL with their values
// Variables are expected in the format {{variable_name}} or ${{variable_name}}
func InterpolateVariables(url string, variables map[string]interface{}) (string, error) {
	result := url

	// Replace {{variable}} format
	pattern := regexp.MustCompile(`\{\{(\w+)\}\}`)
	matches := pattern.FindAllStringSubmatch(url, -1)

	for _, match := range matches {
		if len(match) == 2 {
			varName := match[1]
			value, exists := variables[varName]
			if !exists {
				return "", fmt.Errorf("variable '%s' not found in context", varName)
			}

			// Convert value to string
			strValue := fmt.Sprintf("%v", value)
			result = bytes.ReplaceAll(
				[]byte(result),
				[]byte(match[0]),
				[]byte(strValue),
				1,
			)
		}
	}

	return string(result), nil
}

// FetchRemoteOptions fetches options from a remote HTTP endpoint
// The endpoint should return a JSON array of Option objects
func FetchRemoteOptions(url string) ([]string, error) {
	if url == "" {
		return []string{}, nil
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make the request
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote options from %s: %w", url, err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("remote options endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse JSON response
	var options []Option
	if err := json.Unmarshal(body, &options); err != nil {
		return nil, fmt.Errorf("failed to parse options response as JSON: %w", err)
	}

	// Extract option names
	optionNames := make([]string, 0, len(options))
	for _, opt := range options {
		if opt.Name != "" {
			optionNames = append(optionNames, opt.Name)
		}
	}

	return optionNames, nil
}

// FetchRemoteOptionsWithVariables fetches options from a remote URL with variable interpolation
func FetchRemoteOptionsWithVariables(url string, variables map[string]interface{}) ([]string, error) {
	// Interpolate variables in the URL
	interpolatedURL, err := InterpolateVariables(url, variables)
	if err != nil {
		return nil, fmt.Errorf("variable interpolation failed: %w", err)
	}

	// Fetch options from the interpolated URL
	return FetchRemoteOptions(interpolatedURL)
}
