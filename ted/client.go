package ted

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

type Client struct {
	apiKey string
}

func NewClient() *Client {
	return &Client{
		apiKey: os.Getenv("TED_APIKEY"),
	}
}

type Talk struct {
	ID          string
	Title       string
	Description string
	Duration    int
}

type SearchResponse struct {
	Talks []struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Duration    int    `json:"duration"`
	} `json:"results"`
}

func (c *Client) SearchTalks(ctx context.Context, topic string) ([]Talk, error) {
	if c.apiKey == "" {
		return nil, errors.New("TED_APIKEY environment variable is required")
	}

	baseURL := "https://api.ted.com/v1/talks.json"
	params := url.Values{}
	params.Add("q", topic)
	params.Add("api-key", c.apiKey)
	params.Add("fields", "title,description,duration")
	params.Add("limit", "20")

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var talks []Talk
	for _, t := range response.Talks {
		talks = append(talks, Talk{
			ID:          t.ID,
			Title:       t.Title,
			Description: t.Description,
			Duration:    t.Duration,
		})
	}

	return talks, nil
}

func (c *Client) FilterTalks(talks []Talk, minDuration, maxDuration int) []Talk {
	var filtered []Talk
	for _, talk := range talks {
		if talk.Duration >= minDuration && talk.Duration <= maxDuration {
			filtered = append(filtered, talk)
		}
	}
	return filtered
}
