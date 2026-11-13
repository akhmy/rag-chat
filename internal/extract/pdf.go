package extract

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"iter"
	"os/exec"
	"rag-service/internal/appcontext"
	"strings"
)

type PDFSource struct {
	path  string
	title string
}

func NewPDFSource(path string, title string) *PDFSource {
	return &PDFSource{path: path, title: title}
}

func (s *PDFSource) Title() string {
	return s.title
}

func (s *PDFSource) Iter(ctx context.Context) iter.Seq2[string, error] {
	logger := appcontext.Logger(ctx)

	return func(yield func(string, error) bool) {
		cmdCtx, cancel := context.WithCancel(ctx)
		cmd := exec.CommandContext(cmdCtx, "pdftotext", s.path, "-")
		defer cancel()

		out, err := cmd.StdoutPipe()
		if err != nil {
			yield("", fmt.Errorf("stdout pipe: %w", err))
			return
		}

		if err := cmd.Start(); err != nil {
			yield("", fmt.Errorf("start pdftotext: %w", err))
			return
		}

		reader := bufio.NewReader(out)
		for page := 1; ; page++ {
			text, err := reader.ReadString('\f')
			if err != nil && err != io.EOF {
				cancel()

				if err := cmd.Wait(); err != nil {
					logger.Warn("failed to wait for pdftotext: %w", "error", err)
				}

				yield("", fmt.Errorf("read page %d: %w", page, err))

				return
			}

			text = strings.TrimSpace(text)
			if text != "" {
				if !yield(text, nil) {
					cancel()

					if err := cmd.Wait(); err != nil {
						logger.Warn("failed to wait for pdftotext: %w", "error", err)
					}

					return
				}
			}

			if err == io.EOF {
				break
			}
		}

		if err := cmd.Wait(); err != nil {
			yield("", fmt.Errorf("wait pdftotext: %w", err))
		}
	}
}
