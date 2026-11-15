package extract

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"iter"
	"os"
	"rag-chat/internal/appctx"
	"strings"
)

type TXTSource struct {
	path  string
	title string
}

func NewTXTSource(path string, title string) *TXTSource {
	return &TXTSource{
		path:  path,
		title: title,
	}
}

func (s *TXTSource) Title() string {
	return s.title
}

func (s *TXTSource) Iter(ctx context.Context) iter.Seq2[string, error] {
	logger := appctx.Logger(ctx)

	return func(yield func(string, error) bool) {
		file, err := os.Open(s.path)
		if err != nil {
			yield("", fmt.Errorf("open file: %w", err))
			return
		}

		defer func() {
			if err := file.Close(); err != nil {
				logger.Error("failed to close file", "error", err)
			}
		}()

		reader := bufio.NewReader(file)
		for {
			text, err := reader.ReadString('\f')
			if err != nil && err != io.EOF {
				yield("", fmt.Errorf("read file: %w", err))
				return
			}

			text = strings.TrimSpace(text)
			if text != "" {
				if !yield(text, nil) {
					return
				}
			}

			if err == io.EOF {
				return
			}
		}
	}
}
