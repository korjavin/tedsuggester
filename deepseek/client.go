package deepseek

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type Client struct {
	apiKey string
}

func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey}
}

type DescriptionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type QuestionsResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *Client) GenerateDescription(ctx context.Context, title string) (string, error) {
	prompt := fmt.Sprintf("Create a short 2-3 sentence description for a TED talk titled: %s", title)
	return c.makeRequest(ctx, prompt)
}

func (c *Client) GenerateDiscussionQuestions(ctx context.Context, title string, description string) ([]string, error) {
	prompt := fmt.Sprintf(`Generate 3-4 discussion questions for a TED talk with the following details:
Title: %s
Description: %s

The questions should be thought-provoking and encourage meaningful discussion.`, title, description)

	response, err := c.makeRequest(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Split the response into individual questions
	questions := strings.Split(response, "\n")

	// Clean up each question
	var cleanedQuestions []string
	for _, q := range questions {
		q = strings.TrimSpace(q)
		if q != "" {
			cleanedQuestions = append(cleanedQuestions, q)
		}
	}

	return cleanedQuestions, nil
}

func (c *Client) makeRequest(ctx context.Context, prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.7,
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.deepseek.com/v1/chat/completions", strings.NewReader(string(reqBytes)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response DescriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", errors.New("no choices in response")
	}

	return response.Choices[0].Message.Content, nil
}
