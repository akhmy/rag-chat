package client

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"rag-chat/internal/appcontext"
)

type Client struct {
	client  *http.Client
	baseURL *url.URL
}

func New(client *http.Client, baseURL *url.URL) *Client {
	return &Client{
		client:  client,
		baseURL: baseURL,
	}
}

type AskRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type AskResponse struct {
	Reply string `json:"reply"`
}

func (cli *Client) Ask(ctx context.Context, query string, limit int) (string, error) {
	logger := appcontext.Logger(ctx)

	url := cli.baseURL.JoinPath("ask")

	reqBody, err := json.Marshal(AskRequest{
		Query: query,
		Limit: limit,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url.String(), bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := cli.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if err != nil {
			return "", fmt.Errorf("unexpected status code %d, read body: %w", resp.StatusCode, err)
		}
		return "", fmt.Errorf("unexpected status code %d (%s): %s", resp.StatusCode, url.String(), body)
	}

	var askResp AskResponse
	if err := json.UnmarshalRead(resp.Body, &askResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	return askResp.Reply, nil
}
