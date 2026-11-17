package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"rag-chat/internal/appctx"
)

type DocumentDownloader struct {
	client   *http.Client
	maxBytes int64
}

func NewDocumentDownloader(client *http.Client, maxBytes int64) *DocumentDownloader {
	return &DocumentDownloader{
		client:   client,
		maxBytes: maxBytes,
	}
}

var (
	ErrFileTooLarge = errors.New("file too large")
)

func (d *DocumentDownloader) DownloadTemp(ctx context.Context, downloadURL string) (string, error) {
	logger := appctx.Logger(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		if urlErr, ok := errors.AsType[*url.Error](err); ok {
			err = urlErr.Err
		}
		return "", fmt.Errorf("do request: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if err != nil {
			return "", fmt.Errorf("unexpected status code: %d, read body: %w", resp.StatusCode, err)
		}

		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, body)
	}

	if resp.ContentLength > d.maxBytes {
		return "", ErrFileTooLarge
	}

	file, err := os.CreateTemp("", "rag-upload-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	tempPath := file.Name()
	var isDownloaded bool
	defer func() {
		if !isDownloaded {
			if err := os.Remove(tempPath); err != nil {
				logger.Warn("failed to remove temp file", "error", err)
			}
		}
	}()

	n, err := io.Copy(file, io.LimitReader(resp.Body, d.maxBytes))
	if err != nil {
		return "", fmt.Errorf("write temp file: %w", err)
	}

	if n == d.maxBytes {
		var extra [1]byte
		count, err := file.Read(extra[:])

		if count > 0 {
			return "", ErrFileTooLarge
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("finish download: %w", err)
		}
	}

	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}

	isDownloaded = true

	return tempPath, nil
}
